package jsonrpc2

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// TODO: Handle batch?

type pendingMsg struct {
	msgChan   chan Message
	timestamp time.Time
}

type Service interface {
	Call(result interface{}, method string, params ...interface{}) error
}

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
	resp := r.Server.Handle(msg)
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
		if msg.Response != nil {
			r.getPendingChan(string(msg.ID)) <- msg
		} else {
			// FIXME: Anything we can do with error handling here?
			go r.handleRequest(&msg)
		}
	}
}

func (r *Remote) Send(req *Message) error {
	return json.NewEncoder(r.Conn).Encode(req)
}

func (r *Remote) Receive(ID json.RawMessage) *Message {
	key := string(ID)
	msg := <-r.getPendingChan(key)
	r.mu.Lock()
	delete(r.pending, key)
	r.mu.Unlock()
	return &msg
}

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
