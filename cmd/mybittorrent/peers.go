package main

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	bencode "github.com/jackpal/bencode-go"
)

type PeersResult struct {
}

func peers(file *os.File, showOutput bool) PeersResult {
	infoRes := info(file, false)
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
		fmt.Println("Error: " + resp.Status)
	}

	// Read the response body
	// body, err := ioutil.ReadAll(resp.Body)
	// check(err)
	// fmt.Println(string(body))

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
	for i := 0; i < len(trackerResp.Peers); i += 6 {
		ip := trackerResp.Peers[i : i+4]
		portBytes := []byte(trackerResp.Peers[i+4 : i+6])
		port := binary.BigEndian.Uint16(portBytes)
		fmt.Printf("%d.%d.%d.%d:%d\n", ip[0], ip[1], ip[2], ip[3], port)
	}

	defer resp.Body.Close()
	return PeersResult{}
}
