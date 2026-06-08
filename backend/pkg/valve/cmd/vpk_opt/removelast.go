package main

import (
	"flag"
	"os"

	"l4d2-manager-next/pkg/valve/vpk"
)

func removelast() {
	flagDir := flag.String("dir", "pak01", "")

	flag.Parse()

	dir := vpk.Dir(*flagDir)
	defer dir.Close()

	a, err := dir.ReadArchive()
	if err != nil {
		panic(err)
	}

	var lastArchive uint16

	for _, f := range a.Files {
		for _, c := range f.DataLocation {
			if c.ArchiveIndex > lastArchive && c.ArchiveIndex != 0x7fff {
				lastArchive = c.ArchiveIndex
			}
		}
	}

	newFiles := make([]vpk.File, 0, len(a.Files))
	for _, f := range a.Files {
		ok := true
		for _, c := range f.DataLocation {
			if c.ArchiveIndex == lastArchive {
				ok = false
				break
			}
		}
		if ok {
			newFiles = append(newFiles, f)
		}
	}

	a.Files = newFiles

	if a.Hashes != nil {
		newHashes := make([]vpk.ChunkHash, 0, len(a.Hashes))
		for _, h := range a.Hashes {
			if h.ArchiveIndex != uint32(lastArchive) {
				newHashes = append(newHashes, h)
			}
		}

		a.Hashes = newHashes
	}

	f, err := os.Create(*flagDir + "_dirnew.vpk")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = vpk.WriteDirectory(f, a)
	if err != nil {
		panic(err)
	}
}
