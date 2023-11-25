package peers

import (
	"fmt"
	"os"
)

type Peer struct {
	PeerId	string
	IP 		string
	Port	int64
}

// Parse Tracker response and extract peers information
func ParsePeersFromTracker(trackerData map[string]interface{}) ([]Peer) {
	peers := []Peer{}
	peerInterfaces, ok := trackerData["peers"].([]interface{})

	if ok != true {
		fmt.Println("Could not parse peers")
		os.Exit(1)
	}

	for _, peerInterface := range peerInterfaces {
		peerMap, ok := peerInterface.(map[string]interface{})
		if ok != true {
			fmt.Println("Could not parse peer")
			os.Exit(1)
		}

		peerId, ok := peerMap["peer id"].(string)
		if ok != true {
			peerId = ""
		}

		// Instantiate Peer instance
		peer := Peer{
			PeerId: peerId,
			IP: peerMap["ip"].(string),
			Port: peerMap["port"].(int64),
		}
		peers = append(peers, peer)
	}
	return peers
}
