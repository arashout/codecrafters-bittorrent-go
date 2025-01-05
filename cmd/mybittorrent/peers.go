package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	bencode "github.com/jackpal/bencode-go"
)

type Peer struct {
	IP   net.IP
	Port uint16
}
type PeersResult struct {
	Peers    []Peer
	Interval int
}

func (p Peer) String() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}
func (p PeersResult) Print() {
	// Print the IP address and port number in the format "IP:port"
	for _, peer := range p.Peers {
		fmt.Println(peer.String())
	}

}

func peers(file *os.File) PeersResult {
	infoRes := info(file)
	params := url.Values{}
	params.Add("info_hash", string(infoRes.InfoHash))
	params.Add("peer_id", peerID)
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
		check(fmt.Errorf("Expected status code 200, got %d", resp.StatusCode))
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
		Interval: trackerResp.Interval,
		Peers:    []Peer{},
	}
	for i := 0; i < len(trackerResp.Peers); i += 6 {
		ip := trackerResp.Peers[i : i+4]
		portBytes := []byte(trackerResp.Peers[i+4 : i+6])
		port := binary.BigEndian.Uint16(portBytes)
		res.Peers = append(res.Peers, Peer{
			IP:   net.IP(ip),
			Port: port,
		})
	}

	defer resp.Body.Close()
	return res
}
