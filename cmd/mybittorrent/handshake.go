package main

import (
	"bytes"
	"net"
)

func connect(addr string) net.Conn {
	conn, err := net.Dial("tcp", addr)
	check(err)
	return conn
}

type HandshakeResult struct {
	PeerID []byte
}

func handshake(conn net.Conn, infoHash []byte, peerID []byte) HandshakeResult {
	var b bytes.Buffer
	b.Write([]byte{19}) // Length of "BitTorrent protocol" string
	b.Write([]byte("BitTorrent protocol"))
	b.Write(make([]byte, 8)) // 8 reserved bytes
	b.Write(infoHash)        // 20 byte info hash
	b.Write(peerID)          // 20 byte peer ID

	_, err := conn.Write(b.Bytes())
	check(err)
	resp := make([]byte, 68)
	_, err = conn.Read(resp)
	check(err)

	// Read the response to get the peer ID
	return HandshakeResult{
		PeerID: resp[48:],
	}
}
