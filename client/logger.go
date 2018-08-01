package client

import (
	"io"
	"io/ioutil"
	"log"
)

var logger *log.Logger

func SetLogger(w io.Writer) {
	flags := log.Flags()
	prefix := "[client] "
	logger = log.New(w, prefix, flags)
}

func init() {
	SetLogger(ioutil.Discard)
}
