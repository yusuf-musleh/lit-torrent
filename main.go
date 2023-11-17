package main

import (
	"encoding/json"
	"fmt"
	bencode "github.com/jackpal/bencode-go"
	"os"
)

type file struct {
	Length int
	Path   []string
}

type infoDict struct {
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
	Length      int    `bencode:"length"`
	Files       []file `bencode:"files"`
}

type torrentData struct {
	Announce string   `bencode:"announce"`
	Info     infoDict `bencode:"info"`
}

// Decode .torrent file and populate its values in torrentData struct
func parseTorrentFile(filePath string) (torrentData, error) {
	torrentFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer torrentFile.Close()

	decodedData := torrentData{}

	err = bencode.Unmarshal(torrentFile, &decodedData)

	return decodedData, err
}

func main() {
	command := os.Args[1]

	if command == "download" {
		decoded, err := parseTorrentFile(os.Args[2])
		if err != nil {
			fmt.Println("Invalid .torrent file:", err)
			os.Exit(1)
		}

		jsondecoded, _ := json.MarshalIndent(decoded, "", "  ")
		fmt.Println(string(jsondecoded))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
