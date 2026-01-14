package consts

import (
	"os"
	"path/filepath"
)

var AddonsBasePath string
var GamePath string
var MapListFilePath string

func init() {
	GamePath = os.Getenv("L4D2_GAME_PATH")
	if GamePath == "" {
		GamePath = "./left4dead2"
	}
	AddonsBasePath = filepath.Join(GamePath, "addons")
	MapListFilePath = filepath.Join(AddonsBasePath, "maplist.txt")
}
