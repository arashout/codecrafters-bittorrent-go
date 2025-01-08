package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"sync"
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
	peers := make([]*Peer, len(peersRes.Peers))

	// For each piece in the torrent file, download it, write to a separate file with a suffix with the piece index
	// Combime them after verifying the hashes
	pieceCount := peersRes.InfoResult.MetaInfoFile.Info.PiecesCount()
	pieceJobs := make(chan uint32, pieceCount)
	wg := sync.WaitGroup{}
	wg.Add(pieceCount)

	workerFunc := func(peer *Peer) {
		for {
			pieceIndex := <-pieceJobs
			piece := peer.DownloadPiece(pieceIndex)
			if !verifyPiece(piece, pieceIndex, peer.MetaInfoFile) {
				panic(fmt.Sprintf("Piece %d failed sha verification", pieceIndex))
			}
			pieceFile, err := os.Create(fmt.Sprintf("%s.%d", outputFile, pieceIndex))
			check(err)
			_, err = pieceFile.Write(piece)
			check(err)
			pieceFile.Close()
			wg.Done()
		}
	}

	for i, peer := range peersRes.Peers {
		peers[i] = NewPeer(
			PeerAddress{
				IP:   peer.IP,
				Port: uint16(peer.Port),
			}, peersRes.InfoResult,
		)
		peers[i].connect()
		peers[i].Handshake(peersRes.InfoResult)
		peers[i].InitialDownloadPieceHandshake()
	}

	for i := 0; i < len(peers); i++ {
		go workerFunc(peers[i])
	}

	for i := 0; i < pieceCount; i++ {
		pieceJobs <- uint32(i)
	}
	wg.Wait()

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
