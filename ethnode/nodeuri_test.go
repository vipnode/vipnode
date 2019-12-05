package ethnode

import "testing"

func TestNodeURI(t *testing.T) {
	testcases := []struct {
		Input         string
		IsError       bool
		ID            string
		RemoteAddress string
		String        string
	}{
		{"enode://someid@1.2.3.4:30303", false, "someid", "1.2.3.4:30303", "enode://someid@1.2.3.4:30303"},
		{"foo://someid@1.2.3.4:30303", true, "", "", "foo://someid@1.2.3.4:30303"},
		{"someid@1.2.3.4:30303", false, "someid", "1.2.3.4:30303", "enode://someid@1.2.3.4:30303"},
		{"someid@vipnode.org:30303", false, "someid", "vipnode.org:30303", "enode://someid@vipnode.org:30303"},
		{"someid@localhost:30303", false, "someid", "", "enode://someid@localhost:30303"},
		{"someid@127.0.0.42:30303", false, "someid", "", "enode://someid@127.0.0.42:30303"},
		{"someid@vipnode.org", false, "someid", "vipnode.org", "enode://someid@vipnode.org"},
		{
			Input:         "enode://2c23f8da76a7ecb6e8b220a9165788c0229f7c05ce63102802f72770ac4240ece94596204564702eede6c0ad902b366dcbf0209b246ec64aed3cf1300d77ecf7@18.179.8.13:41100?discport=30301",
			IsError:       false,
			ID:            "2c23f8da76a7ecb6e8b220a9165788c0229f7c05ce63102802f72770ac4240ece94596204564702eede6c0ad902b366dcbf0209b246ec64aed3cf1300d77ecf7",
			RemoteAddress: "18.179.8.13:41100",
			String:        "enode://2c23f8da76a7ecb6e8b220a9165788c0229f7c05ce63102802f72770ac4240ece94596204564702eede6c0ad902b366dcbf0209b246ec64aed3cf1300d77ecf7@18.179.8.13:41100?discport=30301",
		},
		{
			Input:         "enode://someid@[::]:30303?discport=0",
			IsError:       false,
			ID:            "someid",
			RemoteAddress: "",
			String:        "enode://someid@[::]:30303?discport=0",
		},
	}

	for i, tc := range testcases {
		u, err := ParseNodeURI(tc.Input)
		if err != nil {
			if !tc.IsError {
				t.Errorf("[%d] unexpected error: %q", i, err)
			}
			continue
		} else if tc.IsError {
			t.Errorf("[%d] missing expected error", i)
			continue
		}
		if got, want := u.ID(), tc.ID; got != want {
			t.Errorf("[%d] ID - got: %q; want %q", i, got, want)
		}
		if got, want := u.RemoteAddress(), tc.RemoteAddress; got != want {
			t.Errorf("[%d] RemoteAddress - got: %q; want %q", i, got, want)
		}
		if got, want := u.String(), tc.String; got != want {
			t.Errorf("[%d] String - got: %q; want %q", i, got, want)
		}
	}
}
