package ice_test

import (
	"fmt"
	"os"
	"strings"

	"l4d2-manager-next/pkg/valve/ice"
)

func Example() {
	// EKV key from Alien Swarm videocfg.lib
	// https://developer.valvesoftware.com/wiki/Encrypted_Key_Values
	key := ice.Must(ice.New("sW9.JupP"))

	b, err := os.ReadFile("testdata/gpu_mem_level_0_pc.ekv")
	if err != nil {
		panic(err)
	}

	// .ekv files use ECB mode and the last block is not encrypted
	for i := 0; i+key.BlockSize() <= len(b); i += key.BlockSize() {
		key.Decrypt(b[i:], b[i:])
	}

	// these files use CRLF line endings, which Go's testing package does not like
	s := strings.ReplaceAll(string(b), "\r", "")

	fmt.Println(s)

	// Output:
	// "kv"
	// {
	// 	"r_rootlod" "0"
	// 	"mat_picmip" "1"
	// }
}
