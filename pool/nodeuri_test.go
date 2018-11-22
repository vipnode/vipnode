package pool

import "testing"

func TestNormalizeNodeURI(t *testing.T) {
	testcases := []struct {
		nodeURI     string
		nodeID      string
		defaultHost string
		defaultPort string
		wantID      string
		wantErr     bool
	}{
		{
			"enode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1@[::]:30303?discport=0",
			"f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1",
			"foo.com",
			"12345",
			"enode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1@foo.com:30303",
			false,
		},
		{
			"enode://aaaa@abc.com",
			"aaaa",
			"foo.com",
			"30303",
			"enode://aaaa@abc.com:30303",
			false,
		},
		{
			"enode://aaaa@abc.com",
			"bbbb",
			"foo.com",
			"30303",
			"",
			true,
		},
	}

	for i, tc := range testcases {
		got, err := normalizeNodeURI(tc.nodeURI, tc.nodeID, tc.defaultHost, tc.defaultPort)
		if err != nil && !tc.wantErr {
			t.Errorf("Case #%d: Unexpected error: %s", i, err)
			continue
		} else if err == nil && tc.wantErr {
			t.Errorf("Case %d: Missing error", i)
			continue
		} else if err != nil {
			continue
		}
		if got != tc.wantID {
			t.Errorf("Case #%d:\n got: %s\nwant: %s", i, got, tc.wantID)
		}
	}
}
