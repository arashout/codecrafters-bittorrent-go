package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	bencode "github.com/jackpal/bencode-go"
)

const (
	blockSize = uint32(1024 * 16) // 16 KiB
)

type PeerAddress struct {
	IP   net.IP
	Port uint16
}

func (p PeerAddress) String() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}
func (p PeersResult) Print() {
	// Print the IP address and port number in the format "IP:port"
	for _, peer := range p.Peers {
		fmt.Println(peer.String())
	}
}

type PeerMessage struct {
	Length  uint32 // 4 bytes
	ID      uint8  // 1 byte
	Payload []byte // variable length
}
type PeerMessageType = uint8

const (
	Choke         PeerMessageType = 0
	Unchoke       PeerMessageType = 1
	Interested    PeerMessageType = 2
	NotInterested PeerMessageType = 3
	Have          PeerMessageType = 4
	Bitfield      PeerMessageType = 5
	Request       PeerMessageType = 6
	Piece         PeerMessageType = 7
	Cancel        PeerMessageType = 8
)

func (p PeerMessage) Generate() []byte {
	length := 5 + len(p.Payload) // 4 bytes for length, 1 byte for ID, and variable length for payload
	buf := make([]byte, length)
	binary.BigEndian.PutUint32(buf[0:4], uint32(length))
	buf[4] = byte(p.ID)
	copy(buf[5:], p.Payload)
	return buf

}

type Peer struct {
	address PeerAddress
	ID      []byte
	conn    net.Conn
	InfoResult
}

func NewPeer(address PeerAddress, infoRes InfoResult) *Peer {
	return &Peer{
		address:    address,
		InfoResult: infoRes,
	}
}

func (p *Peer) connect() {
	if p.conn != nil {
		return
	}
	conn, err := net.Dial("tcp", p.address.String())
	check(err)
	p.conn = conn
}

func (p *Peer) Handshake(infoRes InfoResult) HandshakeResult {
	var b bytes.Buffer
	b.Write([]byte{19}) // Length of "BitTorrent protocol" string
	b.Write([]byte("BitTorrent protocol"))
	b.Write(make([]byte, 8))  // 8 reserved bytes
	b.Write(infoRes.InfoHash) // 20 byte info hash
	b.Write([]byte(myPeerID)) // 20 byte peer ID

	p.connect()
	_, err := p.conn.Write(b.Bytes())
	check(err)
	resp := make([]byte, 68)
	_, err = p.conn.Read(resp)
	check(err)

	// Read the response to get the peer ID
	return HandshakeResult{
		PeerID: resp[48:],
	}
}
func (p *Peer) ReadMessage() PeerMessage {
	// Read the reserved length prefix first (4 bytes) to determine how big our other buffer should be
	reader := bufio.NewReader(p.conn)
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(reader, lengthBuf)
	check(err)

	pm := PeerMessage{
		Length: binary.BigEndian.Uint32(lengthBuf),
	}

	buf := make([]byte, pm.Length)
	_, err = io.ReadFull(reader, buf)
	check(err)

	pm.ID = buf[0]
	pm.Payload = buf[1:]
	return pm
}

func (p *Peer) GenerateRequestBlock(index uint32, begin uint32, length uint32) PeerMessage {
	message := PeerMessage{
		ID:      Request,
		Payload: make([]byte, 12),
		Length:  12,
	}
	binary.BigEndian.PutUint32(message.Payload[0:4], index)
	binary.BigEndian.PutUint32(message.Payload[4:8], begin)
	binary.BigEndian.PutUint32(message.Payload[8:12], length)
	return message
}

func (p *Peer) fetchBlock(pieceIndex uint32, begin uint32, length uint32) []byte {
	requestMessage := p.GenerateRequestBlock(pieceIndex, begin, length)
	_, err := p.conn.Write(requestMessage.Generate())
	check(err)

	// Wait for "piece" message
	message := p.ReadMessage()
	Assert(message.ID == Piece, fmt.Sprintf("Expected Piece message, but got: %+v\n", message))
	fmt.Printf("Received piece message with length: %d\n", message.Length)

	return message.Payload
}

func (p *Peer) fetchBlocks(pieceIndex uint32, pieceLength uint32) [][]byte {
	numBlocks := (pieceLength + blockSize - 1) / blockSize
	blocks := make([][]byte, numBlocks)
	for i := uint32(0); i < numBlocks; i++ {
		length := blockSize
		if i == numBlocks-1 {
			fmt.Printf("Last block of piece %d has length: %d, calculation %d - %d*%d\n", pieceIndex, length, pieceLength, i, blockSize)
			length = pieceLength - i*blockSize
		}
		fmt.Printf("Fetching block %d of %d of length: %d\n", i+1, numBlocks, length)
		block := p.fetchBlock(pieceIndex, i*blockSize, length)
		blocks[i] = block
	}

	return blocks
}
func (p *Peer) DownloadPiece(output io.Writer, pieceIndex uint32, pieceLength uint32) {
	p.connect() // Ensure the connection is established
	p.Handshake(p.InfoResult)
	defer p.conn.Close()

	// Read initial bitfield message
	bitfieldMessage := p.ReadMessage()
	Assert(bitfieldMessage.ID == Bitfield, "Expected Bitfield message")
	fmt.Printf("Bitfield: %+v\n", bitfieldMessage)

	// Send interested message
	interestedMessage := PeerMessage{
		ID:      Interested,
		Payload: []byte{},
	}.Generate()
	p.conn.Write(interestedMessage)

	// Wait for unchoke message
	message := p.ReadMessage()
	Assert(message.ID == Unchoke, "Expected Unchoke message")

	fmt.Printf("Starting to request piece for torrent: %+v and with index: %d and piece length: %d\n", p.InfoResult, pieceIndex, pieceLength)
	blocks := p.fetchBlocks(pieceIndex, pieceLength)
	n, err := output.Write(bytes.Join(blocks, []byte{}))
	check(err)
	fmt.Printf("Wrote %d bytes to output\n", n)
}

type HandshakeResult struct {
	PeerID []byte
}

type PeersResult struct {
	Peers    []PeerAddress
	Interval int
	InfoResult
}

func peers(file *os.File) PeersResult {
	infoRes := info(file)
	params := url.Values{}
	params.Add("info_hash", string(infoRes.InfoHash))
	params.Add("peer_id", myPeerID)
	params.Add("port", "6881")
	params.Add("uploaded", "0")
	params.Add("downloaded", "0")
	params.Add("left", strconv.Itoa(infoRes.MetaInfoFile.Info.Length))
	params.Add("compact", "1")
	finalURL := fmt.Sprintf("%s?%s", infoRes.Announce, params.Encode())

	resp, err := http.Get(finalURL)
	check(err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		check(fmt.Errorf("expected status code 200, got %d", resp.StatusCode))
	}

	type TrackerResponse struct {
		Interval int    `bencode:"interval"`
		Peers    string `bencode:"peers"`
	}
	trackerResp := TrackerResponse{}
	err = bencode.Unmarshal(resp.Body, &trackerResp)
	check(err)
	// Each peer is 6 bytes:
	//  - 4 bytes for IP address
	//  - 2 bytes for port
	res := PeersResult{
		Interval:   trackerResp.Interval,
		Peers:      []PeerAddress{},
		InfoResult: infoRes,
	}
	for i := 0; i < len(trackerResp.Peers); i += 6 {
		ip := trackerResp.Peers[i : i+4]
		portBytes := []byte(trackerResp.Peers[i+4 : i+6])
		port := binary.BigEndian.Uint16(portBytes)
		res.Peers = append(res.Peers, PeerAddress{
			IP:   net.IP(ip),
			Port: port,
		})
	}

	defer resp.Body.Close()
	return res
}
