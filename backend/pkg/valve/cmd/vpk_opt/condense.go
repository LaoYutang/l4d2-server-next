package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/google/gapid/core/math/interval"
	"l4d2-manager-next/pkg/valve/vpk"
)

func condense() {
	var (
		flagDir     = flag.String("dir", "pak01", "")
		flagArchive = flag.Int("archive", -1, "")
		flagAll     = flag.Bool("all", false, "")
	)

	flag.Parse()

	if *flagAll && *flagArchive != -1 {
		panic("-all is incompatible with -archive")
	}

	dir := vpk.Dir(*flagDir)
	defer dir.Close()

	a, err := dir.ReadArchive()
	if err != nil {
		panic(err)
	}

	if *flagAll {
		var wg sync.WaitGroup

		seen := make(map[uint16]bool)

		for _, e := range a.Files {
			for _, c := range e.DataLocation {
				if seen[c.ArchiveIndex] {
					continue
				}

				seen[c.ArchiveIndex] = true

				wg.Add(1)

				go func(index int) {
					defer wg.Done()

					condenseArchive(dir, a, *flagDir, index)
				}(int(c.ArchiveIndex))
			}
		}

		wg.Wait()
	} else {
		if *flagArchive == -1 {
			for _, e := range a.Files {
				for _, c := range e.DataLocation {
					if *flagArchive < int(c.ArchiveIndex) {
						*flagArchive = int(c.ArchiveIndex)
					}
				}
			}
		}

		condenseArchive(dir, a, *flagDir, *flagArchive)
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

func condenseArchive(o *vpk.Opener, a *vpk.Archive, dir string, archive int) {
	src, err := o.Archive(archive)
	if err != nil {
		panic(err)
	}

	fi, err := src.Stat()
	if err != nil {
		panic(err)
	}

	b := make([]byte, fi.Size())
	_, err = src.ReadAt(b, 0)
	if err != nil {
		panic(err)
	}

	unused := &interval.U64SpanList{
		{0, uint64(len(b))},
	}

	for _, e := range a.Files {
		for _, c := range e.DataLocation {
			if int(c.ArchiveIndex) == archive {
				interval.Remove(unused, interval.U64Span{
					Start: uint64(c.EntryOffset),
					End:   uint64(c.EntryOffset) + uint64(c.EntryLength),
				})
			}
		}
	}

	for i := len(*unused) - 1; i >= 0; i-- {
		start := uint32((*unused)[i].Start)
		end := uint32((*unused)[i].End)
		count := end - start

		b = append(b[:start], b[end:]...)
		for j := range a.Files {
			e := &a.Files[j]
			for k := range e.DataLocation {
				c := &e.DataLocation[k]
				if int(c.ArchiveIndex) == archive && c.EntryOffset > start {
					c.EntryOffset -= count
				}
			}
		}
	}

	err = os.WriteFile(fmt.Sprintf("%s_%03dnew.vpk", dir, archive), b, 0644)
	if err != nil {
		panic(err)
	}
}
