package jsonrpc2

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// TODO: Handle batch?

// ServePipe sets up symmetric server/clients over a net.Pipe() and starts
// both in goroutines. Useful for testing. Services still need to be registered.
func ServePipe() (*Remote, *Remote) {
	c1, c2 := net.Pipe()
	client := Remote{
		Codec:  IOCodec(c1),
		Client: &Client{},
		Server: &Server{},
	}
	server := Remote{
		Codec:  IOCodec(c2),
		Client: &Client{},
		Server: &Server{},
	}
	go server.Serve()
	go client.Serve()
	return &server, &client
}

// ErrContextMissingValue is returned when a context is missing an expected value.
type ErrContextMissingValue struct {
	Key serviceContext
}

func (err ErrContextMissingValue) Error() string {
	return fmt.Sprintf("context missing value: %s", err.Key)
}

type serviceContext string

var ctxService serviceContext = "service"

// CtxService returns a Service associated with this request from a context
// used within a call. This is useful for initiating bidirectional calls.
func CtxService(ctx context.Context) (Service, error) {
	s, ok := ctx.Value(ctxService).(Service)
	if !ok {
		return nil, ErrContextMissingValue{ctxService}
	}
	return s, nil
}

// Service represents a remote service that can be called.
type Service interface {
	Call(ctx context.Context, result interface{}, method string, params ...interface{}) error
}

var _ Service = &Remote{}

// TODO: Make Remote private with a Remote() helper?

// Remote is a wrapper around a connection that can be both a Client and a
// Server. It implements the Service interface, and manages async message
// routing.
type Remote struct {
	Codec
	Client Requester
	Server Handler

	// PendingLimit is the number of messages to hold before oldest messages get discarded.
	PendingLimit int
	// PendingDiscard is the number of oldest messages that get discarded when PendingLimit is reached.
	PendingDiscard int

	mu      sync.Mutex
	pending map[string]pendingMsg
}

// clearPending removes num oldest entries, must hold the r.mu lock.
func (r *Remote) cleanPending(num int) {
	// Clear oldest entries
	for _, item := range pendingOldest(r.pending, num) {
		delete(r.pending, item.key)
	}
}

func (r *Remote) getPendingChan(key string) chan Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending == nil {
		r.pending = map[string]pendingMsg{}
	}
	if r.PendingLimit > 0 && len(r.pending) >= r.PendingLimit && r.PendingDiscard > 0 {
		r.cleanPending(r.PendingDiscard)
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
	ctx := context.WithValue(context.Background(), ctxService, r)
	resp := r.Server.Handle(ctx, msg)
	return r.Codec.WriteMessage(resp)
}

func (r *Remote) Serve() error {
	for {
		msg, err := r.Codec.ReadMessage()
		if err != nil {
			return err
		}
		if msg.Request != nil {
			// FIXME: Anything we can do with error handling here?
			go r.handleRequest(msg)
		} else if len(msg.ID) > 0 {
			r.getPendingChan(string(msg.ID)) <- *msg
		} else {
			logger.Printf("Remote.Serve(): Dropping invalid message: %s", msg)
		}
	}
}

// receive blocks until the given message ID is received. Use Call for an
// end-to-end solution.
func (r *Remote) receive(ctx context.Context, ID json.RawMessage) (*Message, error) {
	key := string(ID)
	select {
	case msg := <-r.getPendingChan(key):
		r.mu.Lock()
		delete(r.pending, key)
		r.mu.Unlock()
		return &msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Call handles sending an RPC and receiving the corresponding response synchronously.
func (r *Remote) Call(ctx context.Context, result interface{}, method string, params ...interface{}) error {
	if r.Client == nil {
		r.Client = &Client{}
	}
	req, err := r.Client.Request(method, params...)
	if err != nil {
		return err
	}
	if err = r.Codec.WriteMessage(req); err != nil {
		return err
	}
	resp, err := r.receive(ctx, req.ID)
	if err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.Error != nil {
		return resp.Response.Error
	}
	if resp.Response == nil || len(resp.Result) == 0 || string(resp.Result) == "null" {
		// No result
		return nil
	}
	return json.Unmarshal(resp.Result, result)
}
