package host

import (
	"io"
	"io/ioutil"
	"log"
)

var logger *log.Logger

func SetLogger(w io.Writer) {
	flags := log.Flags()
	prefix := "[host] "
	logger = log.New(w, prefix, flags)
}

func init() {
	SetLogger(ioutil.Discard)
}
