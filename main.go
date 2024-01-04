package main

import (
	T "github.com/yusuf-musleh/lit-torrent/torrent"
	P "github.com/yusuf-musleh/lit-torrent/peers"

	"os"
	"fmt"
	"sync"
)

func main() {
	command := os.Args[1]

	if command == "download" {
		torrent, filePiecesQueue, file := T.ParseTorrentFile(os.Args[2])
		defer file.Close()

		// While there are still file pieces to process in the queue,
		// and there are no longer any active connections with peers,
		// keep fetching and connecting to peers to download them
		// TODO: We can improve this by making use of the `interval` that
		// is returned from `AnnounceToTracker`.
		for len(filePiecesQueue.FilePieces) > 0 {
			// Announce to Tracker to get available peers
			_, data := torrent.AnnounceToTracker()
			peers := P.ParsePeersFromTracker(data)

			// Establish connections to all available Peers in parallel
			var wg sync.WaitGroup
			for i := range peers {
				wg.Add(1)
				go peers[i].Connect(
					torrent.InfoHash, torrent.PeerId, &wg, filePiecesQueue, &file,
				)
			}

			// Blocks on all go routines with peer connections in this
			// batch until they terminate
			wg.Wait()
		}

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
