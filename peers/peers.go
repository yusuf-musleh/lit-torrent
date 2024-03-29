package peers

import (
	T "github.com/yusuf-musleh/lit-torrent/torrent"
	"github.com/yusuf-musleh/lit-torrent/utils"

	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

const BITTORRENT_PROTOCOL = "BitTorrent protocol"
const READ_BUFFER_SIZE = 1024

const (
	HANDSHAKING = iota + 1
	CONNECTED
	INTERESTED
	CHOKED
	UNCHOKED
	DISCONNECTED
)

type Message struct {
	PrefixLength	int
	MessageId		int
	Payload			[]int
}

// Format message to adhere to BitTorrent protocol
func (m *Message) SerializeMsg() []byte {
	serialized := []byte{}

	// Convert prefix length to 4 byte big-endian format
	serialized = append(serialized, make([]byte, 4)...)
	binary.BigEndian.PutUint32(serialized, uint32(m.PrefixLength))

	// Include the message ID
	serialized = append(serialized, byte(m.MessageId))

	// Convert the payload data to 4 byte big-endian format
	payload := make([]byte, 0, 4*len(m.Payload))
	for _, intData := range m.Payload {
	    intBuffer := make([]byte, 4)
	    binary.BigEndian.PutUint32(intBuffer, uint32(intData))
	    payload = append(payload, intBuffer...)
	}
	// Include the converted payload data
	serialized = append(serialized, payload...)

	return serialized
}

// Convert BitTorrent message to Message struct, excluding a
// parsed payload
func ParseMsg(peerMsg []byte) Message {
	prefixLength := int(binary.BigEndian.Uint32(peerMsg[:4]))
	messageId := -1 // keep-alive message

	// Check if it's not an empty or keep-alive message
	if len(peerMsg) > 4 {
		messageId = int(peerMsg[4])
	}

	return Message{
		PrefixLength: prefixLength,
		MessageId: messageId,
	}
}

type PeerConnectionState int
type PeerConnection struct {
	Conn 	net.Conn
	State	PeerConnectionState
}

type Peer struct {
	PeerId		string
	IP 			string
	Port 		int64
	Connection	PeerConnection
}

// Get the IP/Port pair to connect to Peer with
func (p *Peer) GetConnectAddr() string {
	// Check if IPv6
	if strings.Contains(p.IP, ":") {
		return fmt.Sprintf("[%s]:%d", p.IP, p.Port)
	}

	// Otherwise assume it's an IPv4
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

// Get the Connection object for the peer
func (p *Peer) GetConnection() net.Conn {
	return p.Connection.Conn
}

// Send the actual message bytes to Peer through the TCP connection
func (p *Peer) SendMessageBytes(msgBytes []byte) error {
	_, err := p.GetConnection().Write(msgBytes)
	if err != nil {
		p.Disconnect()
		return err
	}

	return nil
}

// Serialize and send message to Peer
func (p *Peer) SendMessage(message Message) error {
	serializedMsg := message.SerializeMsg()
	return p.SendMessageBytes(serializedMsg)
}

// Generate, perform and validate handshake with Peer
func (p *Peer) PerformHandshake(infoHash [20]byte, peerId string) error {
	// Generate Handshake Message
	handshakeData := []byte{}
	handshakeData = append(handshakeData, byte(19))
	handshakeData = append(handshakeData, []byte("BitTorrent protocol")...)
	handshakeData = append(handshakeData, make([]byte, 8)...)
	handshakeData = append(handshakeData, infoHash[:]...)
	handshakeData = append(handshakeData, []byte(peerId)...)

	sendErr := p.SendMessageBytes(handshakeData)
	if sendErr != nil {
		return sendErr
	}

	// Wait and Read response from the server
	response := make([]byte, READ_BUFFER_SIZE)
	numBytesRecieved, readErr := p.GetConnection().Read(response)
	if readErr != nil {
		p.Disconnect()
		return readErr
	}

	if numBytesRecieved < 68 {
		return errors.New("Invalid Handshake length")
	} else if response[0] != 19 {
		return errors.New("Missing '19' at beginning of Handshake")
	} else if string(response[1:20]) != BITTORRENT_PROTOCOL {
		return errors.New("Missing protocol name in Handshake")
	} else if bytes.Equal(response[28:49], infoHash[:]) {
		return errors.New("Invalid InfoHash in Handshake")
	}

	// Convert peerIds to bytes to handle different encodings
	peerIdRecv := []byte(string(response[48:69]))
	peerIdSent := []byte(p.PeerId)

	if bytes.Equal(peerIdRecv, peerIdSent) {
		return errors.New("Invalid PeerId in Handshake")
	}

	// Populate PeerId data if it was not available before
	if p.PeerId == "" {
		p.PeerId = string(response[48:69])
	}

	return nil
}

// Send a Interested message to the Peer
func (p *Peer) Interested() {
	interested := Message{
		PrefixLength: 1,
		MessageId: 2,
		Payload: []int{},
	}

	err := p.SendMessage(interested)
	if err != nil {
		p.Disconnect()
	} else {
		p.Connection.State = INTERESTED
	}
}

// Send a Request message to the Peer
func (p *Peer) Request(index int, begin int, blockSize int) error {
	request := Message{
		PrefixLength: 13,
		MessageId: 6,
		Payload: []int{index, begin, blockSize},
	}

	err := p.SendMessage(request)
	if err != nil {
		p.Disconnect()
	}
	return err
}

// Reads all the bytes associated with the Block (i.e. downloading it)
func (p *Peer) DownloadBlock(recvMessage Message, response []byte) ([]byte, error) {
	/*
	 * -- Example Piece message --
	 *
	 * Received:               [0 0 64 9 7 0 0 0 0 0 0 0 0 35 35 32 87 ...
	 * prefixed length            |------|
	 * message id                        ||
     * index                               |-----|
     * begin                                       |-----|
     * block                                               |---------- ...
    */
	blockLength := recvMessage.PrefixLength - 9
	BLOCK_STARTING_INDEX := 13
	block := response[BLOCK_STARTING_INDEX:]
	blockBytes := len(block)
	for blockBytes < blockLength {
		response := make([]byte, READ_BUFFER_SIZE)
		n, readErr := p.GetConnection().Read(response)
		if readErr != nil {
			p.Disconnect()
			return nil, readErr
		}
		block = append(block, response[:n]...)
		blockBytes += n
	}
	return block, nil
}

// Set the connection state when disconnect + perform any other
// actions needed on disconnect
func (p *Peer) Disconnect() {
	p.Connection.State = DISCONNECTED
}

// Establish TCP connection with Peer for communication
func (p *Peer) Connect(
	infoHash [20]byte,
	peerId string,
	wg *sync.WaitGroup,
	filePieceQueue *T.FilePiecesQueue,
	file *os.File,
	peerCount *utils.PeerCount,
) {

	defer wg.Done()
	connectTo := p.GetConnectAddr()
	conn, connErr := net.Dial("tcp", connectTo)
	if connErr != nil {
		return
	}
	defer conn.Close()

	// Increment the peer count and decrement when
	// when the connect function terminates
	peerCount.Increment()
	defer peerCount.Decrement()

	// Initialize Peer Connection and begin Handshaking Protocol
	p.Connection = PeerConnection{
		Conn: conn,
		State: HANDSHAKING,
	}

	// Perform Handshake with Peer
	handshakeErr := p.PerformHandshake(infoHash, peerId)
	if handshakeErr != nil {
		p.Disconnect()
	} else {
		p.Connection.State = CONNECTED
	}

	// Send Interested message to Peer
	p.Interested()

	// Initialize required variables
	var requestFilePiece T.FilePiece
	var currentBlockIndex int
	var currentBlockOffset int

	// Begin listening to messages from Peer after successful Handshake
	// and sending the Interested message
	for (p.Connection.State != DISCONNECTED) {
		// If connection with peer is UNCHOKED, pop the next available piece
		// from the queue (if not already) and begin requesting it's blocks
		if p.Connection.State == UNCHOKED && requestFilePiece.Length == 0 {
			nextFilePiece, popErr := filePieceQueue.PopPiece()
			if popErr != nil {
				p.Disconnect()
				break
			}
			requestFilePiece = nextFilePiece
			// Reset the block index/offset at the beginning of a new piece download
			currentBlockIndex = 0
			currentBlockOffset = 0

			// Send Request message to Peer
			reqErr := p.Request(requestFilePiece.Index, currentBlockIndex, T.BLOCK_SIZE)
			if reqErr != nil {
				// Reset the piece if requesting the block failed
				requestFilePiece = requestFilePiece.Reset(filePieceQueue)
				continue
			}
		}

		// Read response from the server
		response := make([]byte, READ_BUFFER_SIZE)
		n, readErr := p.GetConnection().Read(response)
		if readErr != nil {
			p.Disconnect()
			// Reset FilePiece if it fails while being processed
			if requestFilePiece.Length != 0 {
				requestFilePiece = requestFilePiece.Reset(filePieceQueue)
			}
			break
		}

		recvMessage := ParseMsg(response[:n])

		// TODO: look into changing these to cases?
		if recvMessage.MessageId == 0 {
			p.Connection.State = CHOKED
		} else if recvMessage.MessageId == 1 {
			p.Connection.State = UNCHOKED
		} else if recvMessage.MessageId == 7 {
			// Handle receiving piece from peer
			block, err := p.DownloadBlock(recvMessage, response)
			if err != nil {
				// If any block fails, assume this whole piece failed
				// to keep it simple.
				requestFilePiece = requestFilePiece.Reset(filePieceQueue)
				continue
			}

			// Add block data to the PieceContent of the FilePiece
			requestFilePiece.PieceContent = append(requestFilePiece.PieceContent, block...)

			// Increment block offset and index
			currentBlockOffset += requestFilePiece.BlockSizes[currentBlockIndex]
			currentBlockIndex += 1
			if currentBlockIndex < len(requestFilePiece.BlockSizes) {
				// Request the next block
				reqErr := p.Request(
					requestFilePiece.Index,
					currentBlockOffset,
					requestFilePiece.BlockSizes[currentBlockIndex],
				)
				if reqErr != nil {
					// Similar to the above reset the piece if requesting the next block failed
					requestFilePiece = requestFilePiece.Reset(filePieceQueue)
					continue
				}

			} else {
				// No more blocks remain for this piece
				// Verify the integrity of the file piece, discard if not valid
				if requestFilePiece.Verify() {
					// Write downloaded Piece Content to file at the correct offset
					_, writeErr := file.WriteAt(requestFilePiece.PieceContent, int64(requestFilePiece.FileOffset))
					if writeErr != nil {
						requestFilePiece = requestFilePiece.Reset(filePieceQueue)
						continue
					}
					filePieceQueue.IncrementCompleted()
					filePieceQueue.LogProgress(peerCount)
				} else {
					// Discard the file piece content, put it back in the queue
					requestFilePiece = requestFilePiece.Reset(filePieceQueue)
					continue
				}

				// Reset the piece variable to pop the next one from the queue
				requestFilePiece = T.FilePiece{}
			}
		}
	}
}

// Parse Tracker response, extract peers information, initialize peer count
func ParsePeersFromTracker(trackerData map[string]interface{}) ([]Peer, utils.PeerCount) {
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

	initialPeerCount := utils.PeerCount{
		Mu: &sync.Mutex{},
		Count: 0,
	}

	return peers, initialPeerCount
}
