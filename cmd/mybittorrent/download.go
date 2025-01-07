package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

func verifyPiece(piece []byte, pieceIndex uint32, torrent MetaInfoFile) bool {
	var expectedHash [20]byte
	copy(expectedHash[:], torrent.Info.Pieces[pieceIndex*20:(pieceIndex+1)*20])
	hash := sha1.Sum(piece)
	return hash == expectedHash
}

func DownloadPiece(torrentFile string, outputFile string, pieceIndex uint32) {
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

	p.connect()
	defer p.conn.Close()
	p.Handshake(peersRes.InfoResult)

	out, err := os.Create(outputFile)
	check(err)

	p.InitialDownloadPieceHandshake()
	piece := p.DownloadPiece(uint32(pieceIndex))

	// Verify the SHA1 hash of the piece matches the one in the torrent file
	if !verifyPiece(piece, pieceIndex, p.MetaInfoFile) {
		panic(fmt.Sprintf("Piece %d failed sha verification", pieceIndex))
	}

	n, err := out.Write(piece)
	check(err)
	fmt.Printf("Wrote %d bytes to output\n", n)
}

func DownloadFile(torrentFile string, outputFile string) {
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

	p.connect()
	defer p.conn.Close()
	p.Handshake(p.InfoResult)
	// For each piece in the torrent file, download it, write to a separate file with a suffix with the piece index
	// Combime them after verifying the hashes
	pieceCount := peersRes.InfoResult.MetaInfoFile.Info.PiecesCount()
	p.InitialDownloadPieceHandshake()
	for i := 0; i < pieceCount; i++ {
		fmt.Printf("Downloading piece %d out of %d\n", i, pieceCount)
		piece := p.DownloadPiece(uint32(i))
		pieceFile, err := os.Create(fmt.Sprintf("%s.%d", outputFile, i))
		check(err)
		n, err := pieceFile.Write(piece)
		check(err)
		fmt.Printf("Wrote %d bytes to output\n", n)
	}

	// Combine the files
	fmt.Printf("Opening %s\n", outputFile)
	combinedFile, err := os.Create(outputFile)
	check(err)
	defer combinedFile.Close()

	for i := 0; i < peersRes.InfoResult.MetaInfoFile.Info.PiecesCount(); i++ {
		pieceFile, err := os.Open(fmt.Sprintf("%s.%d", outputFile, i))
		check(err)
		_, err = io.Copy(combinedFile, pieceFile)
		check(err)
		pieceFile.Close()
	}

	fmt.Printf("Downloaded %s\n", outputFile)

}
