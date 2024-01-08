# lit-torrent

Basic BitTorrent client written in Go that's pretty lit. 🔥

### Getting Started

1. Clone this repo.
1. Build an executable: `cd ./lit-torrent && go build -o lit-torrent`
1. Run the download command with a .torrent file: `./lit-torrent download [TORRENT].torrent`
1. Enjoy watching the download progress :D

<!-- Insert screenshot here -->

### Implementation Details

This BitTorrent client implements the basics of the original version of the BitTorrent Protocol described in the following specs:

- https://www.bittorrent.org/beps/bep_0003.html
- https://wiki.theory.org/BitTorrentSpecification

To start off, I tried to keep things straightforward and simple to get something that works out sooner. So in its current state, the outline of the algorithm is as follows:

1. Validate and extract necessary information from the .torrent file
1. Break down `Pieces` to the separate pieces that would need to be downloaded based on `PieceLength`. Except the last piece as it could be less than `PieceLength`
1. For each of those pieces, they are further broken down to multiple blocks, each block of size 16384 bytes (16kiB is the recommended block size in the BitTorrent Protocol). Except the last block as it could be less than 16kiB
1. Populate a job queue that contains the file pieces that need to be downloaded, this will be shared across all Peers
1. Initialize the file that we will populate with downloaded pieces onto disk
1. Announce to the Tracker with our PeerID to get information about available peers for the file we wish to download
1. Parse the Peers and fire goroutines to attempt to connect to them in parallel, perform handshakes, and let them know we are `INTERESTED`
1. Once a Peer is ready to serve us, it sends us the `UNCHOKE` message
1. After an `UNCHOKE` message is received, pop the next file piece available from the job queue to process it
1. For each file piece, we `REQUEST` a block from Peer, download it and store in the currently being process FilePiece struct instance
1. Once all the blocks for the current file piece have been downloaded, verify the correctness of the downloaded file piece using the SHA1 Hash
    1. If there are any issues faced when downloading a piece, it is discarded and returned back to the job queue to be picked up again
1. Once verified, write it to the file on disk in the correct position offset
1. Repeat the above process until all the file pieces have been downloaded, processed and written to disk
1. If all the connections with the Peers terminate and there are still file pieces to download, it fetches new peers from the Tracker
1. All the communication with Peers mentioned above above follows the messaging format specified in the BitTorrent Protocol

#### Peer Connection Life-cycle
```
                      ┌─────────────┐          CONNECT           ┌─────────────┐
                      │             ├───────────────────────────►│             │
                      │             │                            │             │
                      │ lit-torrent │                            │   Peer 1    │
                      │   client    ├───────────────────────────►│             │
                      │             │          HANDSHAKE         │             │
                      │             │◄───────────────────────────┤             │
                      │             │                            │             │
                      │             │                            │             │
                      │             │         INTERESTED         │             │
                      │             ├───────────────────────────►│             │
                      │             │                            │             │
                      │             │                            │             │
                      │             │          UNCHOKED          │             │
                      │      ┌──────┤◄───────────────────────────┤             │
                      │      ▼      │                            │             │
                      │  Job Queue  │                            │             │
                      │  ┌───────┐  │                            │             │
                      │  │       │  │          REQUEST           │             │
                      │  │       ├──┼───────────────────────────►│             │
                      │  │  FP1  │  │                            │             │
                      │  ├───────┤  │                            │             │
                      │  │       │  │                            │             │
                      │  │       │  │                            │             │
                      │  │  FP2  │  │                            │             │
                      │  ├───────┤  │                            │             │
                      │  │       │  │                            │             │
                      │  │  FP3  │  │                            │             │
┌──────────┐          │  └───────┘  │                            │             │
│          │   WRITE  │             │           PIECE            │             │
│          │◄─────────┤             │◄───────────────────────────┤             │
│   File   │          │             │                            │             │
│          │          │             │                            │             │
│          │          │             │                            │             │
└──────────┘          └─────────────┘                            └─────────────┘
```

### Verify Correctness

To verify the correctness of this implementation, we check that the checksum of the downloaded file matches what we are expecting. For example, try downloading the [latest Debian image](https://cdimage.debian.org/debian-cd/current/amd64/bt-cd/) that is available through the BitTorrent network. After the download is complete, run the following command (replacing with actual filename) and compare it's output with the SHA256 checksum [provided by Debian](https://cdimage.debian.org/debian-cd/current/amd64/bt-cd/SHA256SUMS):

```sh
sha256sum debian-12.4.0-amd64-netinst.iso
```

### Missing Features

This client is not feature complete, there are a bunch of features missing and will be added incrementally:

- [ ] Refreshing peers based on interval provided by tracker
- [ ] Seeding (uploading) is not supported. Currently this client only supports downloading (leeching)
- [ ] Non-HTTP (eg: UDP) trackers are not supported
- [ ] Multi file downloads, i.e. `files` in .torrent, not supported
- [ ] IPv6 Peers not supported/untested
- [ ] Utilizing Bitfields not supported
- [ ] Unit tests not implemented, currently only manually tested with several .torrent files
- [ ] Optimize file piece download algorithm, improve on current basic ordered file piece algorithm
- [ ] Build out a better TUI for the whole downloading/seeding experience


### Dependencies

This project relies on the following external libraries:

- [github.com/jackpal/bencode-go](https://github.com/jackpal/bencode-go): Bencode encoding and decoding library for Go.
