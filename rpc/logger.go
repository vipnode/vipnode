package rpc

import (
	"io"
	"io/ioutil"
	"log"
)

var logger *log.Logger

func SetLogger(w io.Writer) {
	flags := log.Flags()
	prefix := "[rpc] "
	logger = log.New(w, prefix, flags)
}

func init() {
	SetLogger(ioutil.Discard)
}
