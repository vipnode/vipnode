package agent

import "fmt"

// AgentPoolError is used when the agent fails during a request to a pool.
type AgentPoolError struct {
	cause error
	msg   string
}

func (err AgentPoolError) Error() string {
	return fmt.Sprintf("agent error: %s: %s", err.msg, err.cause.Error())
}

func (err AgentPoolError) Cause() error {
	return err.cause
}
