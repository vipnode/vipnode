package jsonrpc2

import (
	"encoding/json"
	"sync/atomic"
)

type Requester interface {
	// Request takes call inputs and creates a valid request Message.
	Request(method string, params ...interface{}) (*Message, error)
}

var _ Requester = &Client{}

// Client is responsible for making request messages.
type Client struct {
	id int32
}

func (c *Client) NextID() int {
	return int(atomic.AddInt32(&c.id, 1))
}

func (c *Client) Request(method string, params ...interface{}) (*Message, error) {
	return newRequest(c.NextID(), method, params...)
}

// newNotification is a helper for creating encoded Request Messages without IDs.
func newNotification(method string, params ...interface{}) (*Message, error) {
	msg := &Message{
		Version: Version,
		Request: &Request{
			Method: method,
		},
	}
	if len(params) > 0 {
		var err error
		if msg.Request.Params, err = json.Marshal(params); err != nil {
			return nil, err
		}
	}
	return msg, nil
}

// newRequest is a helper for creating encoded Request Messages with an ID.
func newRequest(id interface{}, method string, params ...interface{}) (*Message, error) {
	msg, err := newNotification(method, params...)
	if err != nil {
		return nil, err
	}
	if msg.ID, err = json.Marshal(id); err != nil {
		return nil, err
	}
	return msg, nil
}
