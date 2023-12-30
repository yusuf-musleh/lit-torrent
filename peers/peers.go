package peers

import (
	T "github.com/yusuf-musleh/lit-torrent/torrent"

	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
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

// Convert BitTorrent message to Message struct
func ParseMsg(peerMsg []byte) Message {
	prefixLength := int(binary.BigEndian.Uint32(peerMsg[:4]))
	messageId := -1 // keep-alive message
	payload := []byte{}

	// Check if it's not an empty or keep-alive message
	if len(peerMsg) > 4 {
		messageId = int(peerMsg[4])
		payload = peerMsg[5:]
	}

	parsedPayload := []int{}
	// TODO: This doesn't support the BitField payload
	// Check if only contains 4 byte integers, otherwise don't parse it for now
	// FIX: The following implementation is incorrect
	isIntsOnlyPayload := (len(payload) % 4 == 0)
	if isIntsOnlyPayload {
		for i := 0; i < len(payload) / 4; i += 4 {
			payloadBytes := payload[i:i+4]
			parsedPayload = append(parsedPayload, int(binary.BigEndian.Uint32(payloadBytes)))
		}
	}

	return Message{
		PrefixLength: prefixLength,
		MessageId: messageId,
		Payload: parsedPayload,
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
	bytesSent, err := p.GetConnection().Write(msgBytes)
	if err != nil {
		fmt.Println("Failed to send message to peer", err)
		p.Connection.State = DISCONNECTED
		return err
	}

	fmt.Println("Bytes written:", len(msgBytes))
	fmt.Println("Bytes sent", bytesSent)
	return nil
}

// Serialize and send message to Peer
func (p *Peer) SendMessage(message Message) error {
	serializedMsg := message.SerializeMsg()
	return p.SendMessageBytes(serializedMsg)
}

// Generate, perform and validate handshake with Peer
func (p *Peer) PerformHandshake(infoHash [20]byte, peerId string) error {
	fmt.Println("Performing handshake...")
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

	fmt.Printf("\n[%s] Waiting messages from peer...\n", p.PeerId)
	// Read response from the server
	response := make([]byte, READ_BUFFER_SIZE)
	numBytesRecieved, readErr := p.GetConnection().Read(response)
	if readErr != nil {
		fmt.Println("Failed to read from the connection:", readErr)
		p.Connection.State = DISCONNECTED
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

	fmt.Println("interested", interested)
	err := p.SendMessage(interested)
	if err != nil {
		fmt.Println("send interested message failed", err)
		p.Connection.State = DISCONNECTED
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

	fmt.Println("request", request)
	err := p.SendMessage(request)
	if err != nil {
		fmt.Println("send request message failed", err)
		p.Connection.State = DISCONNECTED
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
	fmt.Println("GOT A PIECE FROM PEER", recvMessage)
	BLOCK_STARTING_INDEX := 13
	block := response[BLOCK_STARTING_INDEX:]
	blockBytes := len(block)
	for blockBytes < blockLength {
		response := make([]byte, READ_BUFFER_SIZE)
		n, readErr := p.GetConnection().Read(response)
		if readErr != nil {
			fmt.Println("Failed to download block:", readErr)
			p.Connection.State = DISCONNECTED
			return nil, readErr
		}
		block = append(block, response[:n]...)
		blockBytes += n
	}
	return block, nil
}

// Establish TCP connection with Peer for communication
func (p *Peer) Connect(
	infoHash [20]byte, peerId string, wg *sync.WaitGroup, filePieceQueue *T.FilePiecesQueue,
) {

	defer wg.Done()
	connectTo := p.GetConnectAddr()
	conn, connErr := net.Dial("tcp", connectTo)
	if connErr != nil {
		fmt.Println("Failed to connect to peer", connErr)
		return
	}

	// Disable timeout when reading from peer
	conn.SetReadDeadline(time.Time{})
	defer conn.Close()

	// Initialize Peer Connection and begin Handshaking Protocol
	p.Connection = PeerConnection{
		Conn: conn,
		State: HANDSHAKING,
	}

	// Perform Handshake with Peer
	handshakeErr := p.PerformHandshake(infoHash, peerId)
	if handshakeErr != nil {
		fmt.Println("handshake failed", handshakeErr)
		p.Connection.State = DISCONNECTED
	} else {
		fmt.Println("handshake successful")
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
		fmt.Printf("\n[%s] Waiting messages from peer...\n", p.PeerId)
		// Read response from the server
		response := make([]byte, READ_BUFFER_SIZE)
		n, readErr := p.GetConnection().Read(response)
		if readErr != nil {
			fmt.Println("Failed to read from the connection:", readErr)
			p.Connection.State = DISCONNECTED
			break
		}

		recvMessage := ParseMsg(response[:n])

		// TODO: look into changing these to cases?
		if recvMessage.MessageId == 0 {
			p.Connection.State = CHOKED
		} else if recvMessage.MessageId == 1 {
			// If connection became unchoked request the piece
			p.Connection.State = UNCHOKED
			// If the requestFilePiece is zeroed out, pop from the Queue
			// TODO: Might need a different approach for this
			if requestFilePiece.Length == 0 {
				requestFilePiece = filePieceQueue.PopPiece()
				// Reset the block index/offset at the beginning of a new piece download
				currentBlockIndex = 0
				currentBlockOffset = 0
			}

			// Send Request message to Peer
			reqErr := p.Request(requestFilePiece.Index, currentBlockIndex, T.BLOCK_SIZE)
			if reqErr != nil {
				// TODO: reset piece content and put it back in the queue
			}

		} else if recvMessage.MessageId == 7 {
			// Handle receiving piece from peer
			fmt.Println("Response prefix", response[:13])
			block, err := p.DownloadBlock(recvMessage, response)
			if err != nil {
				fmt.Println("Failed to download block", err)
				// TODO: If any block fails, assume this whole piece failed
				// to keep it simple. Clear the piece content and put it back
				// in the piece queue so it can be processed again
			}

			// Add block data to the PieceContent of the FilePiece
			requestFilePiece.PieceContent = append(requestFilePiece.PieceContent, block...)

			// Increment block offset and index
			currentBlockOffset += requestFilePiece.BlockSizes[currentBlockIndex]
			currentBlockIndex += 1
			if currentBlockIndex < len(requestFilePiece.BlockSizes) {
				// Request the next block
				fmt.Println("\n\n =====requesting next block", currentBlockOffset)
				reqErr := p.Request(
					requestFilePiece.Index,
					currentBlockOffset,
					requestFilePiece.BlockSizes[currentBlockIndex],
				)
				if reqErr != nil {
					// TODO: Similar to above, since request this block failed, we
					// need to to reset and put the piece back in the queue
				}

			} else {
				// No more blocks remain for this piece
				// Verify the integrity of the file piece, discard if not valid
				fmt.Println("\n\n ==== the full piece", string(requestFilePiece.PieceContent))
				fmt.Println("the piece is:", requestFilePiece.VerifyPiece())
				if requestFilePiece.VerifyPiece() {
					// TODO: Write it main file buffer, but ideally disk
				} else {
					// TODO: discard the file piece content, put it back in the queue
				}

				// TODO: we should reset the piece variable to pop the next one from the queue
			}
		}
	}
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
