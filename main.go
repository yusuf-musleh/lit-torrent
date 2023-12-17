package main

import (
	T "github.com/yusuf-musleh/lit-torrent/torrent"
	P "github.com/yusuf-musleh/lit-torrent/peers"

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

		// Build queue for pieces that need to be downloaded
		filePieces := torrent.GetFilePieces()
		filePiecesQueue := T.FilePiecesQueue{
			FilePieces: filePieces,
		}

		fmt.Println("filePiecesQueue", filePiecesQueue)

		// Announce to Tracker to get available peers
		interval, data := torrent.AnnounceToTracker()
		fmt.Println("\ninterval", interval)
		peers := P.ParsePeersFromTracker(data)
		fmt.Println("peers", peers)

		// TODO: Pass in a reference to the jobs queue to the peers.Connect method so they can
		// interact with it

		// Establish connections to all available Peers in parallel
		var wg sync.WaitGroup
		for i := range peers {
			wg.Add(1)
			go peers[i].Connect(torrent.InfoHash, torrent.PeerId, &wg, &filePiecesQueue)
		}
		wg.Wait()

		// TODO: Instead of waiting for the wg to complete, we should wait
		// for all the pieces to be downloaded.

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
