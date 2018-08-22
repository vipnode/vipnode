package jsonrpc2

import (
	"reflect"
	"testing"
	"time"
)

func TestPendingOldest(t *testing.T) {
	now := time.Now()
	pending := map[string]pendingMsg{
		"1": pendingMsg{timestamp: now.Add(time.Second * 1)},
		"2": pendingMsg{timestamp: now.Add(time.Second * 2)},
		"3": pendingMsg{timestamp: now.Add(time.Second * 3)},
		"4": pendingMsg{timestamp: now.Add(time.Second * 4)},
		"5": pendingMsg{timestamp: now.Add(time.Second * 5)},
	}

	keys := []string{}
	for _, item := range pendingOldest(pending, 3) {
		keys = append(keys, item.key)
	}

	if want, got := []string{"1", "2", "3"}, keys; !reflect.DeepEqual(got, want) {
		t.Errorf("got: %q; want: %q", got, want)
	}
}
