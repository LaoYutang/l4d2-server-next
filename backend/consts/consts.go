package consts

import (
	"os"
	"path/filepath"
)

var BasePath string
var MapListFilePath string

func init() {
	BasePath = os.Getenv("L4D2_ADDONS_PATH")
	if BasePath == "" {
		BasePath = "./addons/"
	}
	MapListFilePath = filepath.Join(BasePath, "maplist.txt")
}
