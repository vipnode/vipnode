package jsonrpc2

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// ServePipe sets up symmetric server/clients over a net.Pipe() and starts
// both in goroutines. Useful for testing. Services still need to be registered.
// FIXME: This is a testing helper, ideally we want to get rid of it. It leaks
// goroutines by design.
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

// ContextMissingValueError is returned when a context is missing an expected value.
type ContextMissingValueError struct {
	Key serviceContext
}

func (err ContextMissingValueError) Error() string {
	return fmt.Sprintf("context missing value: %s", err.Key)
}

type serviceContext string

var ctxService serviceContext = "service"

// CtxService returns a Service associated with this request from a context
// used within a call. This is useful for initiating bidirectional calls.
func CtxService(ctx context.Context) (Service, error) {
	s, ok := ctx.Value(ctxService).(Service)
	if !ok {
		return nil, ContextMissingValueError{ctxService}
	}
	return s, nil
}

// Service represents a remote service that can be called.
type Service interface {
	// Call sends a request message with an auto-incrementing ID, then block
	// until the response is received and unmarshalled into result.
	Call(ctx context.Context, result interface{}, method string, params ...interface{}) error
	// Notify sends a notification message without an ID, returning as soon as
	// the send is completed. Results are ignored.
	Notify(ctx context.Context, method string, params ...interface{}) error
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
	resp := r.Server.Handle(ctx, msg.Request)
	if msg.IsNotification() {
		// No ID, must be a Notification, so response is ignored
		// https://www.jsonrpc.org/specification#notification
		return nil
	}
	return msg.Request.Reply(resp)
}

func (r *Remote) serveOne(blocking bool) error {
	msg, err := r.Codec.ReadMessage()
	if err != nil {
		return err
	}
	if msg.Request != nil {
		if blocking {
			r.handleRequest(msg)
		} else {
			go r.handleRequest(msg)
		}
	} else if !msg.IsNotification() {
		r.getPendingChan(string(msg.ID)) <- *msg
	} else {
		logger.Printf("Remote.Serve(): Dropping invalid message: %s", msg)
	}
	return nil
}

// Serve starts consuming messages from the codec until it fails.
func (r *Remote) Serve() error {
	for {
		if err := r.serveOne(false); err != nil {
			return err
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
	return resp.UnmarshalResult(result)
}

// Notify sends an RPC notification without a message ID, ignoring any results.
func (r *Remote) Notify(ctx context.Context, method string, params ...interface{}) error {
	msg, err := newNotification(method, params)
	if err != nil {
		return err
	}
	return r.Codec.WriteMessage(msg)
}
