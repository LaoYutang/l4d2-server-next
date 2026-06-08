package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) <= 1 || os.Args[1] == "help" {
		log.Println("available tools:")
		log.Println("  condense")
		log.Println("  filter")
		log.Println("  removelast")
		log.Println("  replacefile")
		log.Println("  reuse")

		return
	}

	tool := os.Args[1]
	os.Args = append(os.Args[:1], os.Args[2:]...)

	switch tool {
	case "condense":
		condense()
	case "filter":
		filter()
	case "removelast":
		removelast()
	case "replacefile":
		replacefile()
	case "reuse":
		reuse()
	default:
		log.Println("unknown tool; try vpk_opt help")
	}
}
