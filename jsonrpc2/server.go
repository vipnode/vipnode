package jsonrpc2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"unicode"
)

var ErrNoPublicMethods = errors.New("no public methods")

// Handler is a server that executes an RPC request message, returning an RPC
// response message.
type Handler interface {
	// Handle takes a request message and returns a response message.
	Handle(ctx context.Context, request *Message) (response *Message)

	// FIXME: Register* really shouldn't be part of this signature, right?
	Register(prefix string, receiver interface{}, onlyMethods ...string) error
	RegisterMethod(rpcName string, receiver interface{}, methodName string) error
}

var nullResult = json.RawMessage([]byte("null"))

var _ Handler = &Server{}

// Server contains the method registry.
type Server struct {
	mu       sync.Mutex
	registry map[string]Method
}

// Register adds valid methods from the receiver to the registry with the given
// prefix. Method names are lowercased.
func (s *Server) Register(prefix string, receiver interface{}, onlyMethods ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.registry == nil {
		s.registry = map[string]Method{}
	}

	methods, err := Methods(receiver)
	if err != nil {
		return err
	}
	if len(methods) == 0 {
		// FIXME: This happens when you pass a value receiver instead of
		// pointer. We should be smarter about this.
		return ErrNoPublicMethods
	}

	var methodWhitelist map[string]struct{}
	if len(onlyMethods) > 0 {
		methodWhitelist = map[string]struct{}{}
		for _, name := range onlyMethods {
			methodWhitelist[name] = struct{}{}
		}
	}

	var buf bytes.Buffer
	for name, m := range methods {
		buf.Reset()
		buf.WriteString(prefix)
		buf.WriteRune(unicode.ToLower(rune(name[0])))
		buf.WriteString(name[1:])

		if methodWhitelist != nil {
			// Skip methods that are not whitelisted
			methodName := buf.String()[len(prefix):]
			if _, ok := methodWhitelist[methodName]; !ok {
				continue
			}
		}

		s.registry[buf.String()] = m
	}
	return nil
}

// RegisterMethod registers a single methodName from receiver with the given rpcName.
func (s *Server) RegisterMethod(rpcName string, receiver interface{}, methodName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.registry == nil {
		s.registry = map[string]Method{}
	}

	method, err := MethodByName(receiver, methodName)
	if err != nil {
		return err
	}

	s.registry[rpcName] = method
	return nil
}

// Handle executes a request message against the server registry.
func (s *Server) Handle(ctx context.Context, req *Message) *Message {
	r := &Message{
		Response: &Response{
			Result: nullResult,
		},
		ID:      req.ID,
		Version: Version,
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
			Message: fmt.Sprintf("invalid params: %s %s", req.Method, req.Params),
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
