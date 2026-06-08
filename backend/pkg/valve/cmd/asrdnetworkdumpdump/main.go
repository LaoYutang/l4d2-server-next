package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"l4d2-manager-next/pkg/valve/bits"
	"l4d2-manager-next/pkg/valve/protocol"
)

func main() {
	flagClient := flag.Bool("client", false, "")

	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Expecting one filename.")
		os.Exit(1)
		return
	}

	b, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot read file:", err)
		os.Exit(1)
		return
	}

	r := bits.NewReader(bytes.NewReader(b))
	for {
		p, err := protocol.ReadPacket(r, *flagClient)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		spew.Dump(p)
	}
}
