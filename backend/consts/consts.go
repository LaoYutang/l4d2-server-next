package consts

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

var AddonsBasePath string
var GamePath string
var MapListFilePath string
var ManagerDataPath string
var PrivateKeyPath string
var MonitorDBPath string
var ManagerConfigPath string
var Version = "Dev"

func init() {
	ManagerDataPath = "./data"
	if abs, err := filepath.Abs(ManagerDataPath); err == nil {
		ManagerDataPath = abs
	}
	PrivateKeyPath = filepath.Join(ManagerDataPath, "private.key")
	MonitorDBPath = filepath.Join(ManagerDataPath, "monitor.db")
	ManagerConfigPath = filepath.Join(ManagerDataPath, "manager_config.json")

	if err := EnsureManagerDataPath(); err != nil {
		log.Printf("failed to create manager data directory %s: %v", ManagerDataPath, err)
	}
	migrateLegacyManagerData()

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

func EnsureManagerDataPath() error {
	return os.MkdirAll(ManagerDataPath, 0755)
}

func migrateLegacyManagerData() {
	legacyFiles := map[string]string{
		"private.key":         PrivateKeyPath,
		"manager_config.json": ManagerConfigPath,
		"monitor.db":          MonitorDBPath,
		"monitor.db-wal":      filepath.Join(ManagerDataPath, "monitor.db-wal"),
		"monitor.db-shm":      filepath.Join(ManagerDataPath, "monitor.db-shm"),
	}

	for legacyPath, targetPath := range legacyFiles {
		migrateLegacyFile(legacyPath, targetPath)
	}
}

func migrateLegacyFile(legacyPath, targetPath string) {
	legacyAbs, legacyErr := filepath.Abs(legacyPath)
	targetAbs, targetErr := filepath.Abs(targetPath)
	if legacyErr == nil && targetErr == nil && legacyAbs == targetAbs {
		return
	}

	info, err := os.Stat(legacyPath)
	if err != nil || info.IsDir() {
		return
	}
	if _, err := os.Stat(targetPath); err == nil {
		return
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		log.Printf("failed to create directory for %s: %v", targetPath, err)
		return
	}
	if err := os.Rename(legacyPath, targetPath); err == nil {
		return
	}
	if err := copyFile(legacyPath, targetPath, info.Mode().Perm()); err != nil {
		log.Printf("failed to migrate %s to %s: %v", legacyPath, targetPath, err)
	}
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
