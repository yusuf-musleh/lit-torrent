package torrent

import (
	"github.com/yusuf-musleh/lit-torrent/utils"

	P "github.com/yusuf-musleh/lit-torrent/peers"

	"fmt"
	"os"
	"bytes"
	"strconv"
	"net/http"
	"net/url"
	"crypto/sha1"

	bencode "github.com/jackpal/bencode-go"
)

type infoDict struct {
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
	Length      int    `bencode:"length"`
}

type Torrent struct {
	Announce string   `bencode:"announce"`
	Info     infoDict `bencode:"info"`
	InfoHash [20]byte
	PeerId	 string
}

// Generate random peer ID for torrent session
func (t *Torrent) GeneratePeerId() {
	// Generate random string of length 12
	randomStr, _ := utils.GenerateRandomString(12)
	t.PeerId = "-LI1000-" + randomStr
}

func (t *Torrent) GenerateInfoHashSHA1() {
	bencodedInfo := bytes.NewBuffer([]byte{})
	bencode.Marshal(bencodedInfo, t.Info)
    infoHash := sha1.Sum(bencodedInfo.Bytes())
    t.InfoHash = infoHash
}

// Build tracker request URL with required query params
func (t *Torrent) GenerateTrackerRequestURL() (string) {
    // Build request url query params
	queryParams := url.Values{}
	queryParams.Add("info_hash", string(t.InfoHash[:]))
	queryParams.Add("peer_id", t.PeerId)
	queryParams.Add("port", "6889")
	queryParams.Add("uploaded", "0")
	queryParams.Add("downloaded", "0")
	queryParams.Add("left", strconv.Itoa(t.Info.Length))
	queryParams.Add("event", "started")

	url := t.Announce + "?" + queryParams.Encode()
	return url
}

// Performs the announce request to the tracker
func (t *Torrent) AnnounceToTracker() (int64, []P.Peer) {
	url := t.GenerateTrackerRequestURL()
	fmt.Println("url", url)

	response, err := http.Get(url)

	if err != nil {
		fmt.Println("Failed to reach Tracker:", err)
		os.Exit(1)
	}

	defer response.Body.Close()

	data, parseErr := utils.ParseBencodeResponse(response.Body)

	if parseErr != nil {
		fmt.Println("Failed to parse Tracker response body: ", parseErr)
		os.Exit(1)
	}

	if failReason, announceFailed := data["failure reason"]; announceFailed {
		fmt.Println("Announce failed: ", failReason)
		os.Exit(1)
	}

	peers := P.ParsePeersFromTracker(data)

	return data["interval"].(int64), peers
}
