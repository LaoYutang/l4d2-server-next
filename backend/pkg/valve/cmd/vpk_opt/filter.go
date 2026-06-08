package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"l4d2-manager-next/pkg/valve/vpk"
)

func filter() {
	flagDirName := flag.String("dirname", "pak01", "vpk directory name")

	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Println("Usage: vpk_opt filter [options] file_list.txt")
		flag.PrintDefaults()
		os.Exit(1)
	}

	namePresent, err := readNamesForFilter(flag.Arg(0))
	if err != nil {
		panic(err)
	}

	o := vpk.Dir(*flagDirName)
	a, err := o.ReadArchive()
	_ = o.Close()
	if err != nil {
		panic(err)
	}

	filtered := make([]vpk.File, 0, len(a.Files))
	anySkipped := false
	for _, f := range a.Files {
		if name := f.Name(); namePresent[name] {
			filtered = append(filtered, f)
		} else {
			fmt.Println("Removed", name)
			anySkipped = true
		}
	}

	if anySkipped {
		a.Files = filtered
		f, err := os.Create(*flagDirName + "_dirnew.vpk")
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = vpk.WriteDirectory(f, a)
		if err != nil {
			panic(err)
		}
	}
}

func readNamesForFilter(filename string) (map[string]bool, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	namePresent := make(map[string]bool)

	s := bufio.NewScanner(f)
	for s.Scan() {
		name := s.Text()
		name = strings.ReplaceAll(name, "\\", "/")
		name = path.Clean(name)
		base := path.Base(name)
		if i, j := strings.IndexByte(base, '.'), strings.LastIndexByte(base, '.'); i != j {
			name = path.Join(path.Dir(name), base[:i]+base[j:])
		}
		name = strings.ToLower(name)
		namePresent[name] = true
	}

	return namePresent, s.Err()
}
