package jsonrpc2

import (
	"encoding/json"
	"errors"
	"reflect"
)

// reflectPositionalArgs takes the params of a JSONRPC message, and asserts
// each positional argument into the reflected value of its type. It only
// supports positional arguments for params, and will give an error for other
// kinds of params.
func reflectPositionalArgs(msgParams json.RawMessage, types []reflect.Type) ([]reflect.Value, error) {
	// TODO: Add error type
	if len(msgParams) == 0 {
		return nil, errors.New("no params given")
	}

	args := make([]interface{}, 0, len(types))
	if err := json.Unmarshal(msgParams, &args); err != nil {
		return nil, err
	}
	if len(args) > types {
		return nil, errors.New("too many arguments")
	}

	values := make([]reflect.Value, 0, len(types))
	for i, arg := range args {
		if arg == nil {
			return nil, errors.New("not enough arguments")
		}
		value := reflect.New(types[i])
	}

	return values, nil
}
