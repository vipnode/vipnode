package status

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/vipnode/vipnode/pool/store"
)

func TestPoolStatus(t *testing.T) {
	now := time.Now()
	s := PoolStatus{
		Store:         store.MemoryStore(),
		TimeStarted:   now,
		Version:       "foo",
		CacheDuration: time.Minute * 10,
	}

	r, err := s.Status(context.Background())
	if err != nil {
		t.Error(err)
	}

	expected := &StatusResponse{
		TimeUpdated: r.TimeUpdated,
		TimeStarted: now,
		Version:     "foo",
		ActiveHosts: []Host{},
		Error:       nil,
	}

	if !reflect.DeepEqual(r, expected) {
		t.Errorf("incorrect status response:\n got: %v;\nwant: %v", r, expected)
	}

	hostNode := store.Node{ID: "12345678901234567890", IsHost: true, Kind: "geth", LastSeen: now}
	if err := s.Store.SetNode(hostNode); err != nil {
		t.Fatal(err)
	}

	// Get cached response again
	r, err = s.Status(context.Background())
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(r, expected) {
		t.Errorf("expected cached response:\n got: %v;\nwant: %v", r, expected)
	}

	// Disable cache and try again
	s.CacheDuration = 0

	r, err = s.Status(context.Background())
	if err != nil {
		t.Error(err)
	}

	expected = &StatusResponse{
		TimeUpdated: r.TimeUpdated,
		TimeStarted: now,
		Version:     "foo",
		ActiveHosts: []Host{
			Host{
				ShortID:  "123456789012",
				LastSeen: now,
				Kind:     "geth",
			},
		},
		Error: nil,
	}

	if !reflect.DeepEqual(r, expected) {
		t.Errorf("incorrect status response:\n got: %v;\nwant: %v", r, expected)
	}
}
