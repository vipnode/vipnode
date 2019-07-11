package ethnode

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestGethParsePeerInfo(t *testing.T) {
	// Some examples of parity_netPeers output
	testcases := []struct {
		input   []byte
		want    []PeerInfo
		NodeIDs []string
	}{
		{
			input: []byte(`[{
				"caps": ["eth/62", "eth/63"],
				"enode": "enode://2c23f8da76a7ecb6e8b220a9165788c0229f7c05ce63102802f72770ac4240ece94596204564702eede6c0ad902b366dcbf0209b246ec64aed3cf1300d77ecf7@18.179.8.13:41100",
				"id": "f8ad8f79e5fcb50c98d68db5f5c1c1ff0e6867cab733e1c8890101d5306704b4",
				"name": "Geth/v1.8.23-stable-c9427004/linux-amd64/go1.10.4",
				"network": {
				  "inbound": true,
				  "localAddress": "10.0.74.242:30303",
				  "remoteAddress": "18.179.8.13:41100",
				  "static": false,
				  "trusted": false
				},
				"protocols": {
				  "eth": {}
				}
			}, {
				"id": "foo"
			}]`),
			want: []PeerInfo{
				{
					ID:    "f8ad8f79e5fcb50c98d68db5f5c1c1ff0e6867cab733e1c8890101d5306704b4",
					Enode: "enode://2c23f8da76a7ecb6e8b220a9165788c0229f7c05ce63102802f72770ac4240ece94596204564702eede6c0ad902b366dcbf0209b246ec64aed3cf1300d77ecf7@18.179.8.13:41100",
					Name:  "Geth/v1.8.23-stable-c9427004/linux-amd64/go1.10.4",
					Caps:  []string{"eth/62", "eth/63"},
					Network: struct {
						LocalAddress  string `json:"localAddress"`
						RemoteAddress string `json:"remoteAddress"`
					}{"10.0.74.242:30303", "18.179.8.13:41100"},
					Protocols: map[string]json.RawMessage{
						"eth": json.RawMessage("{}"),
					},
				},
				{
					ID: "foo",
				},
			},
			NodeIDs: []string{"2c23f8da76a7ecb6e8b220a9165788c0229f7c05ce63102802f72770ac4240ece94596204564702eede6c0ad902b366dcbf0209b246ec64aed3cf1300d77ecf7", "foo"},
		},
	}

	for i, tc := range testcases {
		var result []PeerInfo
		err := json.Unmarshal(tc.input, &result)
		if err != nil {
			t.Errorf("[case %d] unexpected error for testcase: %s", i, err)
			continue
		}
		if !reflect.DeepEqual(result, tc.want) {
			t.Errorf("[case %d] wrong agent values:\n  got: %+v;\n want: %+v", i, result, tc.want)
		}
		if got, want := Peers(result).IDs(), tc.NodeIDs; !reflect.DeepEqual(got, want) {
			t.Errorf("[case %d] wrong node IDs:\n  got: %+v;\n want: %+v", i, got, want)
		}
	}

}
