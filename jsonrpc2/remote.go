package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"
)

// TODO: Handle batch?

// ServePipe sets up symmetric server/clients over a net.Pipe() and starts
// both in goroutines. Useful for testing. Services still need to be registered.
func ServePipe() (*Remote, *Remote) {
	c1, c2 := net.Pipe()
	client := Remote{Codec: IOCodec(c1)}
	server := Remote{Codec: IOCodec(c2)}
	go server.Serve()
	go client.Serve()
	return &server, &client
}

var ErrContextMissingValue = errors.New("context missing value")

type serviceContext string

var ctxService serviceContext = "service"

// CtxService returns a Service associated with this request from a context
// used within a call. This is useful for initiating bidirectional calls.
func CtxService(ctx context.Context) (Service, error) {
	s, ok := ctx.Value(ctxService).(Service)
	if !ok {
		return nil, ErrContextMissingValue
	}
	return s, nil
}

type pendingMsg struct {
	msgChan   chan Message
	timestamp time.Time
}

// Service represents a remote service that can be called.
type Service interface {
	Call(ctx context.Context, result interface{}, method string, params ...interface{}) error
}

var _ Service = &Remote{}

// Remote is a wrapper around a connection that can be both a Client and a
// Server. It implements the Service interface, and manages async message
// routing.
type Remote struct {
	Codec
	Client
	Server

	mu      sync.Mutex
	pending map[string]pendingMsg
}

func (r *Remote) getPendingChan(key string) chan Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending == nil {
		r.pending = map[string]pendingMsg{}
	}
	pending, ok := r.pending[key]
	if !ok {
		pending = pendingMsg{
			msgChan:   make(chan Message, 1),
			timestamp: time.Now(),
		}
		r.pending[key] = pending
	}
	return pending.msgChan
}

func (r *Remote) handleRequest(msg *Message) error {
	ctx := context.WithValue(context.TODO(), ctxService, r)
	resp := r.Server.Handle(ctx, msg)
	return r.Codec.WriteMessage(resp)
}

func (r *Remote) Serve() error {
	// TODO: Discard old pending messages
	for {
		msg, err := r.Codec.ReadMessage()
		if err != nil {
			return err
		}
		if msg.Response != nil {
			r.getPendingChan(string(msg.ID)) <- *msg
		} else {
			// FIXME: Anything we can do with error handling here?
			go r.handleRequest(msg)
		}
	}
}

// Receive blocks until the given message ID is received. Use Call for an
// end-to-end solution.
func (r *Remote) Receive(ID json.RawMessage) *Message {
	key := string(ID)
	msg := <-r.getPendingChan(key)
	r.mu.Lock()
	delete(r.pending, key)
	r.mu.Unlock()
	return &msg
}

// Call handles sending an RPC and receiving the corresponding response synchronously.
func (r *Remote) Call(ctx context.Context, result interface{}, method string, params ...interface{}) error {
	// TODO: Plumb ctx
	req, err := r.Client.Request(method, params...)
	if err != nil {
		return err
	}
	if err = r.Codec.WriteMessage(req); err != nil {
		return err
	}
	resp := r.Receive(req.ID)
	if resp.Error != nil {
		return resp.Error
	}
	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		// No result
		return nil
	}
	return json.Unmarshal(resp.Result, result)
}
