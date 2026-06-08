package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"log"
	"os"

	"l4d2-manager-next/pkg/valve/vpk"
)

func replacefile() {
	var (
		flagDir = flag.String("dir", "pak01", "")
	)

	flag.Parse()

	if flag.NArg() != 2 {
		log.Println("Usage: vpk_opt replacefile [options] path/in/directory file/with/new/contents")
		flag.PrintDefaults()
		os.Exit(1)
	}

	b, err := os.ReadFile(flag.Arg(1))
	if err != nil {
		panic(err)
	}

	dir := vpk.Dir(*flagDir)
	defer dir.Close()

	a, err := dir.ReadArchive()
	if err != nil {
		panic(err)
	}

	fileIndex := -1
	for i := range a.Files {
		if a.Files[i].Name() == flag.Arg(0) {
			fileIndex = i
			break
		}
	}

	if fileIndex == -1 {
		log.Println("Could not find file in archive:", flag.Arg(0))
		return
	}

	if a.Files[fileIndex].Size() != len(b) {
		log.Printf("Can only replace a file with one of the same size (old %d new %d)", a.Files[fileIndex].Size(), len(b))
		return
	}

	for _, l1 := range a.Files[fileIndex].DataLocation {
		for i := range a.Files {
			if i == fileIndex {
				continue
			}

			for _, l2 := range a.Files[i].DataLocation {
				if l1.ArchiveIndex == l2.ArchiveIndex && l1.EntryOffset+l1.EntryLength > l2.EntryOffset && l2.EntryOffset+l2.EntryLength > l1.EntryOffset {
					log.Print("Cannot replace file in archive; overlaps with", a.Files[i].Name())
					return
				}
			}
		}
	}

	a.Files[fileIndex].CRC = crc32.ChecksumIEEE(b)
	b = b[copy(a.Files[fileIndex].Metadata, b):]

	f, err := os.Create(*flagDir + "_dirnew.vpk")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = vpk.WriteDirectory(f, a)
	if err != nil {
		panic(err)
	}

	err = f.Close()
	if err != nil {
		panic(err)
	}

	seen := make(map[uint16]bool)

	for _, l := range a.Files[fileIndex].DataLocation {
		sourceSuffix := "new.vpk"
		if !seen[l.ArchiveIndex] {
			sourceSuffix = ".vpk"
			seen[l.ArchiveIndex] = true
		}

		b1, err := os.ReadFile(fmt.Sprintf("%s_%03d%s", *flagDir, l.ArchiveIndex, sourceSuffix))
		if err != nil {
			panic(err)
		}

		copy(b1[l.EntryOffset:], b[:l.EntryLength])
		b = b[l.EntryLength:]

		err = os.WriteFile(fmt.Sprintf("%s_%03dnew.vpk", *flagDir, l.ArchiveIndex), b1, 0644)
		if err != nil {
			panic(err)
		}
	}

	if len(b) != 0 {
		panic(fmt.Errorf("expected EOF but there are %d bytes remaining", len(b)))
	}
}
