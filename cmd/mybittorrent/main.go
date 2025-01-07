package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"unicode"

	bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

const (
	defaultPort = 6881
	myPeerID    = "ash___out_1234567890"
)

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	if unicode.IsDigit(rune(bencodedString[0])) {
		var firstColonIndex int

		for i := 0; i < len(bencodedString); i++ {
			if bencodedString[i] == ':' {
				firstColonIndex = i
				break
			}
		}

		lengthStr := bencodedString[:firstColonIndex]

		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return "", err
		}

		return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], nil
	} else {
		return "", fmt.Errorf("Only strings are supported at the moment")
	}
}

type MetaInfoFile struct {
	Announce string       `bencode:"announce"`
	Info     MetaInfoInfo `bencode:"info"`
}
type MetaInfoInfo struct {
	Length      uint32 `bencode:"length"`
	Name        string `bencode:"name"`
	PieceLength uint32 `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	command := os.Args[1]

	switch command {
	case "decode":
		bencodedValue := os.Args[2]
		data, err := bencode.Decode(bytes.NewBufferString(bencodedValue))
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(data)
		fmt.Println(string(jsonOutput))
	case "info":
		filename := os.Args[2]
		file, err := os.Open(filename)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer file.Close()

		res := info(file)
		res.Print()

	case "peers":
		filename := os.Args[2]
		file, err := os.Open(filename)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer file.Close()
		res := peers(file)
		res.Print()
	case "handshake":
		filename := os.Args[2]
		file, err := os.Open(filename)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer file.Close()

		address := os.Args[3]
		peerAddress, err := net.ResolveUDPAddr("udp", address)
		check(err)
		infoRes := info(file)
		peer := NewPeer(
			PeerAddress{
				IP:   peerAddress.IP,
				Port: uint16(peerAddress.Port),
			}, infoRes,
		)

		res := peer.Handshake(infoRes)
		fmt.Printf("Peer ID: %x\n", res.PeerID)
	case "download_piece":
		// -o <output_file> <torrent_file> <piece_index>
		fs := flag.NewFlagSet("download_piece", flag.ExitOnError)
		outputFilename := fs.String("o", "", "Output filename")

		// Parse flags starting AFTER the subcommand name
		fs.Parse(os.Args[2:])

		// Now fs.Args() holds the positional args AFTER we parse flags
		args := fs.Args()
		if len(args) < 2 {
			fmt.Println("Usage: download_piece -o <output> <torrent_file> <piece_index>")
			os.Exit(1)
		}
		torrentFile := args[0]
		pieceIndex := args[1]

		pieceIndexInt, err := strconv.ParseInt(pieceIndex, 10, 32)
		check(err)
		downloadPiece(torrentFile, *outputFilename, pieceIndexInt)

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}

}
