package peers

import (
	"fmt"
	"os"
	"net"
	"errors"
	"bytes"
	"time"
	"sync"
)

const BITTORRENT_PROTOCOL = "BitTorrent protocol"
const READ_BUFFER_SIZE = 1024

type PeerConnectionState int

const (
	HANDSHAKING = iota + 1
	CONNECTED
	DISCONNECTED
)

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
func (p *Peer) GetConnectUrl() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

// Get the Connection object for the peer
func (p *Peer) GetConnection() net.Conn {
	return p.Connection.Conn
}

// Send messages to Peer through TCP connection
func (p *Peer) SendMessage(message []byte) error {
	bytesSent, err := p.GetConnection().Write(message)
	if err != nil {
		fmt.Println("Failed to send message to peer", err)
		p.Connection.State = DISCONNECTED
		return err
	}

	fmt.Println("Bytes written:", len(message))
	fmt.Println("Bytes sent", bytesSent)
	return nil
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

	sendErr := p.SendMessage(handshakeData)
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

// Establish TCP connection with Peer for communication
func (p *Peer) Connect(infoHash [20]byte, peerId string, wg *sync.WaitGroup) {
	defer wg.Done()
	connectTo := p.GetConnectUrl()
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

	// Begin listening to messages from Peer once Handshake succeeds
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

		fmt.Println("Received:", response[:n])
		fmt.Println("Received (String)", string(response[:n]))
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
			IP: peerMap["ip"].(string),  // TODO: Convert non-IP format to IP
			Port: peerMap["port"].(int64),
		}
		peers = append(peers, peer)
	}
	return peers
}
