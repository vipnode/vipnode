package status

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/vipnode/vipnode/pool/store"
	"github.com/vipnode/vipnode/pool/store/memory"
)

func compareJSON(t *testing.T, got, want interface{}) {
	t.Helper()

	gotBytes, err := json.Marshal(got)
	if err != nil {
		t.Errorf("compareJSON: failed to marshal got value: %s", err)
		return
	}
	wantBytes, err := json.Marshal(want)
	if err != nil {
		t.Errorf("compareJSON: failed to marshal want value: %s", err)
		return
	}

	if bytes.Compare(gotBytes, wantBytes) != 0 {
		t.Errorf("compareJSON failed:\n got: %s\nwant: %s", gotBytes, wantBytes)
	}
}

func TestPoolStatus(t *testing.T) {
	now := time.Now()
	s := PoolStatus{
		Store:         memory.New(),
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
		Stats:       &store.Stats{},
		ActiveHosts: []Host{},
		Error:       nil,
	}

	compareJSON(t, r, expected)

	hostNode := store.Node{ID: "12345678901234567890", IsHost: true, Kind: "geth", LastSeen: now}
	if err := s.Store.SetNode(hostNode); err != nil {
		t.Fatal(err)
	}

	// Get cached response again
	r, err = s.Status(context.Background())
	if err != nil {
		t.Error(err)
	}

	compareJSON(t, r, expected)

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
		Stats: &store.Stats{
			NumTotalHosts:  1,
			NumActiveHosts: 1,
		},
		ActiveHosts: []Host{
			Host{
				ShortID:  "123456789012",
				LastSeen: now,
				Kind:     "geth",
			},
		},
		Error: nil,
	}

	compareJSON(t, r, expected)
}
