package main

import "os"

func downloadPiece(torrentFile string, outputFile string, pieceIndex int64) {
	file, err := os.Open(torrentFile)
	check(err)
	defer file.Close()

	peersRes := peers(file)
	// Right now we're just going to pick the first peer
	peerAddress := peersRes.Peers[0]
	p := NewPeer(
		PeerAddress{
			IP:   peerAddress.IP,
			Port: uint16(peerAddress.Port),
		}, peersRes.InfoResult,
	)

	out, err := os.Create(outputFile)
	check(err)

	p.DownloadPiece(out, uint32(pieceIndex))
}
