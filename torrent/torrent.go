package torrent

import (
	"github.com/yusuf-musleh/lit-torrent/utils"

	"bytes"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"

	bencode "github.com/jackpal/bencode-go"
)

const BLOCK_SIZE = 16384 // 16kiB

type FilePiece struct {
	Index			int
	Length 			int
	Hash 			string
	BlockSizes		[]int
	PieceContent	[]byte
}

// Returns the sizes of the blocks that need to be downloaded
// that add up to the FilePiece
func (fp *FilePiece) ComputeBlockSizes() {
	blockSizes := []int{}
	fullBlocks := fp.Length / BLOCK_SIZE
	finalBlock := fp.Length % BLOCK_SIZE

	for i := 0; i < fullBlocks; i++ {
		blockSizes = append(blockSizes, BLOCK_SIZE)
	}

	if finalBlock > 0 {
		blockSizes = append(blockSizes, finalBlock)
	}

	fp.BlockSizes = blockSizes
}

// Verify the integrity of the content of the downloaded piece
// by comparing the SHA1 hash
func (fp *FilePiece) Verify() bool {
	hashedPieceContent := sha1.Sum(fp.PieceContent)
	return fp.Hash == string(hashedPieceContent[:])
}

// Clear the piece content and put it back in the piece queue
// so it can be processed again
func (fp *FilePiece) Reset(queue *FilePiecesQueue) FilePiece {
	fp.PieceContent = []byte{}
	queue.InsertPiece(*fp)
	return FilePiece{}
}

type FilePiecesQueue struct {
	mu 			sync.Mutex
	FilePieces	[]FilePiece
}

// Safely pop next available FilePiece from File Piece Queue
func (queue *FilePiecesQueue) PopPiece() FilePiece {
	var piece FilePiece
	queue.mu.Lock()
	if len(queue.FilePieces) > 0 {
		piece, queue.FilePieces = queue.FilePieces[0], queue.FilePieces[1:]
	}
	queue.mu.Unlock()
	return piece
}

// Safely add FilePiece to File Piece Queue, this is used to retry failed
// attempts to download File Piece
func (queue *FilePiecesQueue) InsertPiece(piece FilePiece) {
	queue.mu.Lock()
	queue.FilePieces = append(queue.FilePieces, piece)
	queue.mu.Unlock()
}

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

// Generate the SHA1 Hash for the content of Info in torrent file
func (t *Torrent) GenerateInfoHashSHA1() {
	bencodedInfo := bytes.NewBuffer([]byte{})
	bencode.Marshal(bencodedInfo, t.Info)
    infoHash := sha1.Sum(bencodedInfo.Bytes())
    t.InfoHash = infoHash
}

// Returns number of pieces needed to download along with
// remaining bytes in last piece
func (t *Torrent) GetFilePiecesCount() (int, int) {
	pieceCount := t.Info.Length / t.Info.PieceLength
	finalPieceBytes := t.Info.Length % t.Info.PieceLength
	return pieceCount, finalPieceBytes
}

// Returns instances of `FilePiece` containing information about
// all the file pieces that need to be downloaded for the torrent
func (t *Torrent) GetFilePieces() ([]FilePiece) {
	filePieces := []FilePiece{}
	pieceCount, finalPieceBytes := t.GetFilePiecesCount()
	pieceCounter := 0
	index := 0
	hashIndex := 0

	// Populate file pieces
	for pieceCounter < pieceCount {
		filePiece := FilePiece{
			Index: index,
			Length: t.Info.PieceLength,
			Hash: t.Info.Pieces[hashIndex:hashIndex+20],
		}
		filePiece.ComputeBlockSizes()
		filePieces = append(filePieces, filePiece)
		pieceCounter += 1
		index += t.Info.PieceLength
		hashIndex += 20
	}

	// Populate last remaining piece
	if finalPieceBytes > 0 {
		filePiece := FilePiece{
			Index: index,
			Length: finalPieceBytes,
			Hash: t.Info.Pieces[hashIndex:],
		}
		filePiece.ComputeBlockSizes()
		filePieces = append(filePieces, filePiece)
	}

	return filePieces
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
func (t *Torrent) AnnounceToTracker() (int64, map[string]interface{}) {
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

	return data["interval"].(int64), data
}
