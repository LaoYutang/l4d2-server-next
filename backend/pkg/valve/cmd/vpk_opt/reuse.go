package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/google/gapid/core/math/interval"
	"l4d2-manager-next/pkg/valve/vpk"
)

func reuse() {
	var (
		flagWorkers      = flag.Int("workers", runtime.GOMAXPROCS(0), "")
		flagDir          = flag.String("dir", "pak01", "")
		flagStartArchive = flag.Int("startarchive", 0, "")
	)

	flag.Parse()

	dir := vpk.Dir(*flagDir)
	defer dir.Close()

	a, err := dir.ReadArchive()
	if err != nil {
		panic(err)
	}

	var usedArchives []int

	for _, e := range a.Files {
		for _, c := range e.DataLocation {
			index := int(c.ArchiveIndex)

			i := sort.SearchInts(usedArchives, index)
			if i >= len(usedArchives) || usedArchives[i] != index {
				usedArchives = append(usedArchives[:i], append([]int{index}, usedArchives[i:]...)...)

				// pre-open the file
				_, _ = dir.Archive(index)
			}
		}
	}

	log.Println(len(usedArchives), "archives,", len(a.Files), "files to process")
	if *flagStartArchive != 0 {
		if *flagStartArchive < 0 {
			*flagStartArchive += len(usedArchives)
		}

		log.Println("skipping", *flagStartArchive, "archives")
	}

	var totalArchiveSize, totalUnused uint64

	for _, archive := range usedArchives {
		log.Println("processing archive", archive)

		b := readFullArchive(dir, archive)
		totalArchiveSize += uint64(len(b))

		var wg sync.WaitGroup
		wg.Add(*flagWorkers)

		for i := 0; i < *flagWorkers; i++ {
			start := i
			go func() {
				defer wg.Done()

				for j := start; j < len(a.Files); j += *flagWorkers {
					e := &a.Files[j]
					for k := range e.DataLocation {
						c := &e.DataLocation[k]
						if int(c.ArchiveIndex) < archive || int(c.ArchiveIndex) < *flagStartArchive {
							continue
						}

						if c.EntryLength == 0 {
							continue
						}

						limit := len(b)
						if int(c.ArchiveIndex) == archive {
							limit = int(c.EntryOffset) + int(c.EntryLength) - 1
						}

						compare := make([]byte, c.EntryLength)

						f, err := dir.Archive(int(c.ArchiveIndex))
						if err != nil {
							panic(err)
						}

						_, err = f.ReadAt(compare, int64(c.EntryOffset))
						if err != nil {
							panic(err)
						}

						off := bytes.Index(b[:limit], compare)
						if off == -1 {
							continue
						}

						log.Println("moved", c.EntryLength, "bytes from archive", c.ArchiveIndex, "offset", c.EntryOffset, "to archive", archive, "offset", off, "-", e.Name())
						c.ArchiveIndex = uint16(archive)
						c.EntryOffset = uint32(off)
					}
				}
			}()
		}

		wg.Wait()

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

		var unusedCount uint64
		for _, s := range *unused {
			unusedCount += s.End - s.Start
		}

		totalUnused += unusedCount

		log.Println("archive", archive, "is", len(b), "bytes,", unusedCount, "unused")
	}

	log.Println("total unused space", totalUnused, "/", totalArchiveSize, "bytes;", fmt.Sprintf("%.2f%%", float64(totalUnused*100)/float64(totalArchiveSize)))

	f, err := os.Create(*flagDir + "_dirnew.vpk")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	vpk.WriteDirectory(f, a)
}

func readFullArchive(o *vpk.Opener, index int) []byte {
	f, err := o.Archive(index)
	if err != nil {
		panic(err)
	}

	fi, err := f.Stat()
	if err != nil {
		panic(err)
	}

	b := make([]byte, fi.Size())
	_, err = f.ReadAt(b, 0)
	if err != nil {
		panic(err)
	}

	return b
}

func verify(o *vpk.Opener, a *vpk.Archive) {
	for _, e := range a.Files {
		r, err := e.Open(o)
		if err != nil {
			panic(err)
		}

		err = r.Close()
		if err != nil {
			panic(err)
		}
	}

	for _, h := range a.Hashes {
		f, err := o.Archive(int(h.ArchiveIndex))
		if err != nil {
			panic(err)
		}

		err = h.Verify(f)
		if err != nil {
			panic(err)
		}
	}
}
