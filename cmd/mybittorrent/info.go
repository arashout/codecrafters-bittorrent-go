package main

import (
	"crypto/sha1"
	"fmt"
	"os"

	bencode "github.com/jackpal/bencode-go"
)

type InfoResult struct {
	MetaInfoFile
	InfoHash []byte
}

func info(f *os.File, showOutput bool) InfoResult {
	metaInfoFile := MetaInfoFile{}
	err := bencode.Unmarshal(f, &metaInfoFile)
	check(err)

	// Calculate info hash
	h := sha1.New()
	err = bencode.Marshal(h, metaInfoFile.Info)
	check(err)

	res := InfoResult{
		MetaInfoFile: metaInfoFile,
		InfoHash:     h.Sum(nil),
	}

	if showOutput {
		printInfo(res)
	}
	return res
}

func printInfo(res InfoResult) {
	fmt.Printf("Tracker URL: %s\nLength: %d\nInfo Hash: %x", res.MetaInfoFile.Announce, res.MetaInfoFile.Info.Length, res.InfoHash)
	fmt.Printf("Piece Length: %d\nPiece Hashes:\n", res.MetaInfoFile.Info.PieceLength)
	for i := 0; i < len(res.MetaInfoFile.Info.Pieces); i += 20 {
		fmt.Printf("%x\n", res.MetaInfoFile.Info.Pieces[i:i+20])
	}
}
