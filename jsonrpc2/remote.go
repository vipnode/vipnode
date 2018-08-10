package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

// TODO: Handle batch?

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
	Call(result interface{}, method string, params ...interface{}) error
}

var _ Service = &Remote{}

// Remote is a wrapper around a connection that can be both a Client and a
// Server. It implements the Service interface, and manages async message
// routing.
type Remote struct {
	Conn io.ReadWriteCloser
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
	return json.NewEncoder(r.Conn).Encode(resp)
}

func (r *Remote) Serve() error {
	// TODO: Discard old pending messages
	decoder := json.NewDecoder(r.Conn)
	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			return err
		}
		// TODO: Handle err == io.EOF?
		if msg.Response != nil {
			r.getPendingChan(string(msg.ID)) <- msg
		} else {
			// FIXME: Anything we can do with error handling here?
			go r.handleRequest(&msg)
		}
	}
}

// Send encodes the Message and sends it to the Connection. Use Call for an
// end-to-end solution.
func (r *Remote) Send(req *Message) error {
	return json.NewEncoder(r.Conn).Encode(req)
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
func (r *Remote) Call(result interface{}, method string, params ...interface{}) error {
	req, err := r.Client.Request(method, params...)
	if err != nil {
		return err
	}
	if err = r.Send(req); err != nil {
		return err
	}
	resp := r.Receive(req.ID)
	if resp.Error != nil {
		return resp.Error
	}
	return json.Unmarshal(resp.Result, result)
}
