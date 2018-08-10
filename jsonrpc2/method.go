package jsonrpc2

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()
var typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()

// methodArgTypes returns the arg types and whether all the types are valid
// (exported or builtin).
func methodArgTypes(methodType reflect.Type) (argTypes []reflect.Type, hasCtx bool, ok bool) {
	argNum := methodType.NumIn()
	argTypes = make([]reflect.Type, 0, argNum-1)
	argPos := 1 // Skip receiver
	for ; argPos < argNum; argPos++ {
		argType := methodType.In(argPos)
		if !isExportedOrBuiltin(argType) {
			return nil, hasCtx, false
		}
		if argType == typeOfContext {
			hasCtx = true
			continue
		}
		argTypes = append(argTypes, argType)
	}
	return argTypes, hasCtx, true
}

// methodErrPos returns the return value index position of an error type for
// supported return layouts: (), (interface{}), (error), (interface{}, error)
func methodErrPos(methodType reflect.Type) (int, bool) {
	switch methodType.NumOut() {
	case 0:
	case 1:
		if methodType.Out(0) == typeOfError {
			// Single error return value
			return 0, true
		}
		// Single non-error return value
		return -1, true
	case 2:
		if methodType.Out(1) == typeOfError {
			// Two return values, one error type
			return 1, true
		}
		// Two return values, no error type, unsupported.
		return -1, false
	}
	return -1, false
}

// Methods returns a mapping of valid method names to Method definitions for a
// instance's receiver.
func Methods(receiver interface{}) (map[string]Method, error) {
	kind := reflect.TypeOf(receiver)
	val := reflect.ValueOf(receiver)
	if name := reflect.Indirect(val).Type().Name(); !isExported(name) {
		return nil, fmt.Errorf("receiver must be exported: %s", name)
	}

	methods := map[string]Method{}
	for i := 0; i < kind.NumMethod(); i++ {
		method := kind.Method(i)
		if method.PkgPath != "" {
			// Skip unexported methods
			continue
		}

		// Load arg types (skip first arg, the receiver)
		argTypes, hasCtx, ok := methodArgTypes(method.Type)
		if !ok {
			// Skip methods with unexported arg types
			continue
		}

		// Substitute injected types

		// Find ErrPos, if any.
		errPos, ok := methodErrPos(method.Type)
		if !ok {
			return nil, fmt.Errorf("unsupported return values in method: %s", method.Name)
		}

		methods[method.Name] = Method{
			Receiver: val,
			Method:   method,
			ArgTypes: argTypes,
			ErrPos:   errPos,
			HasCtx:   hasCtx,
		}
	}

	return methods, nil
}

// Method is the definition of a callable method.
type Method struct {
	Receiver reflect.Value
	Method   reflect.Method
	ArgTypes []reflect.Type
	ErrPos   int
	HasCtx   bool
}

// CallJSON wraps Call but supports JSON-encoded args
func (m *Method) CallJSON(ctx context.Context, rawArgs json.RawMessage) (interface{}, error) {
	args, err := parsePositionalArguments(rawArgs, m.ArgTypes)
	if err != nil {
		return nil, err
	}
	return m.Call(ctx, args)
}

// Call executes the method with the given arguments.
func (m *Method) Call(ctx context.Context, args []reflect.Value) (interface{}, error) {
	if len(args) != len(m.ArgTypes) {
		return nil, fmt.Errorf("invalid number of args: expected %d, got %d", len(m.ArgTypes), len(args))
	}

	arguments := []reflect.Value{m.Receiver}
	if m.HasCtx {
		arguments = append(arguments, reflect.ValueOf(ctx))
	}
	if len(args) > 0 {
		arguments = append(arguments, args...)
	}

	reply := m.Method.Func.Call(arguments)

	// Are there any return values?
	if len(reply) == 0 {
		return nil, nil
	}
	// Is there an error return value?
	if m.ErrPos >= 0 && !reply[m.ErrPos].IsNil() {
		return nil, reply[m.ErrPos].Interface().(error)
	}

	// All is good, assume the first result is what we want to return
	// This supports (), (err), (res, err)
	return reply[0].Interface(), nil
}
