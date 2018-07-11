package main

import (
	"io/ioutil"

	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/golog"
)

var logger *golog.Logger

// SetLogger overrides the main logger of this command.
func SetLogger(l *golog.Logger) {
	logger = l
}

func init() {
	// Set a default null logger
	SetLogger(golog.New(ioutil.Discard, log.Debug))
}
