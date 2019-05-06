package ethnode

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParityParsePeerInfo(t *testing.T) {
	// Some examples of parity_netPeers output
	testcases := []struct {
		input []byte
		want  parityPeers
	}{
		{
			input: []byte(`{
				"active":0,
				"connected":1,
				"max":25,
				"peers":[
				  {
				    "caps":[
				      "eth/62",
				      "eth/63",
				      "par/1",
				      "par/2",
				      "pip/1"
				    ],
				    "id":"a117e71696a740f2d0e5427fed5fa0ab1f343799aa873b46c361d405e9b3319689b26867e9b76921b0714ad79470d374ae4ef15f28ad4fe521de4c4f3ce702bc",
				    "name":"Parity/v1.8.0-beta-9882902-20171015/x86_64-linux-gnu/rustc1.21.0",
				    "network":{
				      "localAddress":"172.18.0.2:41980",
				      "remoteAddress":"172.18.0.3:30303"
				    },
				    "protocols":{
				      "eth":{},
				      "pip":{}
				    }
				  }
				]
			}`),
			want: parityPeers{
				Peers: []PeerInfo{
					{
						ID:   "a117e71696a740f2d0e5427fed5fa0ab1f343799aa873b46c361d405e9b3319689b26867e9b76921b0714ad79470d374ae4ef15f28ad4fe521de4c4f3ce702bc",
						Name: "Parity/v1.8.0-beta-9882902-20171015/x86_64-linux-gnu/rustc1.21.0",
						Caps: []string{"eth/62", "eth/63", "par/1", "par/2", "pip/1"},
						Network: struct {
							LocalAddress  string `json:"localAddress"`
							RemoteAddress string `json:"remoteAddress"`
						}{"172.18.0.2:41980", "172.18.0.3:30303"},
						Protocols: map[string]json.RawMessage{
							"eth": json.RawMessage("{}"),
							"pip": json.RawMessage("{}"),
						},
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		var result parityPeers
		err := json.Unmarshal(tc.input, &result)
		if err != nil {
			t.Errorf("[case %d] unexpected error for testcase: %s", i, err)
			continue
		}
		// Clear protocol values for comparison
		if !reflect.DeepEqual(result, tc.want) {
			t.Errorf("[case %d] wrong agent values:\n  got: %+v;\n want: %+v", i, result, tc.want)
		}
	}

}
