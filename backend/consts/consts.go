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
		// Check for local left4dead2 directory for testing
		if _, err := os.Stat("./left4dead2"); err == nil {
			if abs, err := filepath.Abs("./left4dead2"); err == nil {
				GamePath = abs
			}
		} else if _, err := os.Stat("backend/left4dead2"); err == nil {
			// Check for backend/left4dead2 (if running from project root)
			if abs, err := filepath.Abs("backend/left4dead2"); err == nil {
				GamePath = abs
			}
		} else {
			GamePath = "/left4dead2"
		}
	}
	AddonsBasePath = filepath.Join(GamePath, "addons")
	MapListFilePath = filepath.Join(AddonsBasePath, "maplist.txt")
}
