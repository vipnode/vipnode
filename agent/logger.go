package agent

import (
	"io"
	"io/ioutil"
	"log"
)

var logger *log.Logger

// SetLogger overrides the logger output for this package.
func SetLogger(w io.Writer) {
	flags := log.Flags()
	prefix := "[agent] "
	logger = log.New(w, prefix, flags)
}

func init() {
	SetLogger(ioutil.Discard)
}
