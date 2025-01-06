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
	Length  int    // 4 bytes
	ID      int    // 1 byte
	Payload []byte // variable length
}
type PeerMessageType int

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

func parsePeerMessage(data []byte) PeerMessage {
	return PeerMessage{
		Length:  int(binary.BigEndian.Uint32(data[0:4])),
		ID:      int(data[4]), // Tells you the message type
		Payload: data[5:],
	}
}

func (p PeerMessage) Generate() []byte {
	length := 1 + len(p.Payload)
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
	// Create a buffered reader
	reader := bufio.NewReader(p.conn)
	buf := make([]byte, 1024) // TODO: Is this enough?
	n, err := reader.Read(buf)
	check(err)
	return parsePeerMessage(buf[:n])
}

func (p *Peer) DownloadPiece(output io.Writer, pieceIndex int64, pieceLength int) {
	p.connect() // Ensure the connection is established
	p.Handshake(p.InfoResult)
	defer p.conn.Close()

	// Read initial bitfield message
	bitfieldMessage := p.ReadMessage()
	Assert(bitfieldMessage.ID == int(Bitfield), "Expected Bitfield message")
	fmt.Printf("Bitfield: %+v\n", bitfieldMessage)

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
