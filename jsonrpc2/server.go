package jsonrpc2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"unicode"
)

// Server contains the method registry.
type Server struct {
	mu       sync.Mutex
	registry map[string]Method
}

// Register adds valid methods from the receiver to the registry with the given
// prefix. Method names are lowercased.
func (s *Server) Register(prefix string, receiver interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.registry == nil {
		s.registry = map[string]Method{}
	}

	methods, err := Methods(receiver)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	for name, m := range methods {
		buf.WriteString(prefix)
		buf.WriteRune(unicode.ToLower(rune(name[0])))
		buf.WriteString(name[1:])
		s.registry[buf.String()] = m
		buf.Reset()
	}
	return nil
}

// Handle executes a request message against the server registry.
func (s *Server) Handle(ctx context.Context, req *Message) *Message {
	r := &Message{
		Response: &Response{},
		ID:       req.ID,
		Version:  Version,
	}
	if req.Request == nil {
		r.Error = &ErrResponse{
			Code:    ErrCodeInvalidRequest,
			Message: "server received misformed request",
		}
		return r
	}

	s.mu.Lock()
	m, ok := s.registry[req.Request.Method]
	s.mu.Unlock()

	if !ok {
		r.Response.Error = &ErrResponse{
			Code:    ErrCodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
		return r
	}
	args, err := parsePositionalArguments(req.Params, m.ArgTypes)
	if err != nil {
		r.Error = &ErrResponse{
			Code:    ErrCodeInvalidParams,
			Message: fmt.Sprintf("invalid params: %s", req.Params),
		}
		return r
	}
	res, err := m.Call(ctx, args)
	if err != nil {
		r.Error = &ErrResponse{
			Code:    ErrCodeInternal,
			Message: err.Error(),
		}
		return r
	}
	if res == nil {
		return r
	}
	if r.Result, err = json.Marshal(res); err != nil {
		r.Error = &ErrResponse{
			Code:    ErrCodeServer,
			Message: fmt.Sprintf("failed to encode response: %s", err),
		}
	}
	return r
}
