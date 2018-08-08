package jsonrpc2

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// methodArgTypes returns the arg types and whether all the types are valid
// (exported or builtin).
func methodArgTypes(methodType reflect.Type) ([]reflect.Type, bool) {
	argNum := methodType.NumIn()
	argTypes := make([]reflect.Type, 0, argNum-1)
	for j := 1; j < argNum; j++ {
		argType := methodType.In(j)
		if !isExportedOrBuiltin(argType) {
			return nil, false
		}
		argTypes = append(argTypes, argType)
	}
	return argTypes, true
}

// methodErrPos returns the return value index position of an error type for
// supported return layouts: (), (interface{}), (error), (interface{}, error)
func methodErrPos(methodType reflect.Type) (int, bool) {
	switch methodType.NumOut() {
	case 0:
	case 1:
		if methodType.Out(0) == typeOfError {
			return 0, true
		}
	case 2:
		if methodType.Out(1) == typeOfError {
			return 1, true
		}
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
		argTypes, ok := methodArgTypes(method.Type)
		if !ok {
			// Skip methods with unexported arg types
			continue
		}

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
}

// CallJSON wraps Call but supports JSON-encoded args
func (m *Method) CallJSON(rawArgs json.RawMessage) (interface{}, error) {
	args, err := parsePositionalArguments(rawArgs, m.ArgTypes)
	if err != nil {
		return nil, err
	}
	return m.Call(args)
}

// Call executes the method with the given arguments.
func (m *Method) Call(args []reflect.Value) (interface{}, error) {
	if len(args) != len(m.ArgTypes) {
		return nil, fmt.Errorf("invalid number of args: expected %d, got %d", len(m.ArgTypes), len(args))
	}

	arguments := []reflect.Value{m.Receiver}
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
