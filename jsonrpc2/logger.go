package jsonrpc2

import (
	"io"
	"io/ioutil"
	"log"
)

var logger *log.Logger

// SetLogger overrides the logger output for this package.
func SetLogger(w io.Writer) {
	flags := log.Flags()
	prefix := "[jsonrpc2] "
	logger = log.New(w, prefix, flags)
}

func init() {
	SetLogger(ioutil.Discard)
}
