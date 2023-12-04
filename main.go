package main

import (
	T "github.com/yusuf-musleh/lit-torrent/torrent"

	"os"
	"fmt"
	"sync"

	bencode "github.com/jackpal/bencode-go"
)

// Decode .torrent file and populate its values in Torrent struct
func parseTorrentFile(filePath string) (T.Torrent, error) {
	torrentFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer torrentFile.Close()

	decodedData := T.Torrent{}

	err = bencode.Unmarshal(torrentFile, &decodedData)

	return decodedData, err
}

func main() {
	command := os.Args[1]

	if command == "download" {
		torrent, err := parseTorrentFile(os.Args[2])
		if err != nil {
			fmt.Println("Invalid .torrent file:", err)
			os.Exit(1)
		}

		// Initializing torrent data
		torrent.GeneratePeerId()
		torrent.GenerateInfoHashSHA1()

		// Announce to Tracker to get available peers
		interval, peers := torrent.AnnounceToTracker()
		fmt.Println("\ninterval", interval)
		fmt.Println("peers", peers)

		// Establish connections to all available Peers in parallel
		var wg sync.WaitGroup
		for i := range peers {
			wg.Add(1)
			go peers[i].Connect(torrent.InfoHash, torrent.PeerId, &wg)
		}
		wg.Wait()

		// TODO: Implement the rest of BitTorrent's communication protocol

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
