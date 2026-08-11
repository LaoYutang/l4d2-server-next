package logic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"l4d2-manager-next/consts"
)

const (
	ServerCustomConfigMarker = "// [L4D2-MANAGER-CUSTOM]"
	GameBanConfigMarker      = "// [L4D2-MANAGER-GAME-BANS]"
)

var gameBanPersistenceMu sync.Mutex

var gameBanConfigLines = []string{
	GameBanConfigMarker,
	"exec banned_user.cfg",
	"exec banned_ip.cfg",
}

var gameBanTickConfigNames = []string{
	"server.cfg.128tick",
	"server.cfg.100tick",
	"server.cfg.60tick",
	"server.cfg.30tick",
}

// EnsureGameBanPersistenceConfig keeps the native Source ban files loaded by
// server.cfg without exposing those directives as user custom configuration.
func EnsureGameBanPersistenceConfig() error {
	gameBanPersistenceMu.Lock()
	defer gameBanPersistenceMu.Unlock()

	cfgDir := filepath.Join(consts.GamePath, "cfg")
	info, err := os.Stat(cfgDir)
	if err != nil {
		return fmt.Errorf("访问游戏配置目录失败: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("游戏配置路径不是目录: %s", cfgDir)
	}

	targets, err := gameBanPersistenceTargets(cfgDir)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if err := ensureGameBanPersistenceFile(target); err != nil {
			return err
		}
	}
	return nil
}

// IsGameBanPersistenceReady reports whether every current server config target
// already contains the canonical hidden block. It never changes files.
func IsGameBanPersistenceReady() bool {
	gameBanPersistenceMu.Lock()
	defer gameBanPersistenceMu.Unlock()

	cfgDir := filepath.Join(consts.GamePath, "cfg")
	info, err := os.Stat(cfgDir)
	if err != nil || !info.IsDir() {
		return false
	}
	targets, err := gameBanPersistenceTargets(cfgDir)
	if err != nil {
		return false
	}
	for _, target := range targets {
		content, err := os.ReadFile(target)
		if err != nil || !hasCanonicalGameBanPersistenceBlock(string(content)) {
			return false
		}
	}
	return true
}

func gameBanPersistenceTargets(cfgDir string) ([]string, error) {
	targets := []string{filepath.Join(cfgDir, "server.cfg")}
	for _, name := range gameBanTickConfigNames {
		path := filepath.Join(cfgDir, name)
		if _, err := os.Stat(path); err == nil {
			targets = append(targets, path)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("检查游戏配置 %s 失败: %w", name, err)
		}
	}
	return targets, nil
}

func ensureGameBanPersistenceFile(path string) error {
	content, err := os.ReadFile(path)
	mode := os.FileMode(0644)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("读取 %s 失败: %w", filepath.Base(path), err)
		}
	} else if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	} else {
		return fmt.Errorf("读取 %s 权限失败: %w", filepath.Base(path), statErr)
	}

	updated := buildGameBanPersistenceContent(string(content))
	if updated == string(content) {
		return nil
	}
	if err := atomicWriteFile(path, []byte(updated), mode); err != nil {
		return fmt.Errorf("更新 %s 失败: %w", filepath.Base(path), err)
	}
	return nil
}

func buildGameBanPersistenceContent(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == GameBanConfigMarker || isGameBanExecDirective(trimmed) {
			continue
		}
		filtered = append(filtered, strings.TrimSuffix(line, "\r"))
	}
	filtered = trimTrailingBlankLines(filtered)

	customIndex := len(filtered)
	for index, line := range filtered {
		if strings.TrimSpace(line) == ServerCustomConfigMarker {
			customIndex = index
			break
		}
	}

	prefix := trimTrailingBlankLines(filtered[:customIndex])
	suffix := trimLeadingBlankLines(filtered[customIndex:])
	result := make([]string, 0, len(filtered)+5)
	result = append(result, prefix...)
	if len(result) > 0 {
		result = append(result, "")
	}
	result = append(result, gameBanConfigLines...)
	if len(suffix) > 0 {
		result = append(result, "")
		result = append(result, suffix...)
	}

	return strings.Join(result, "\n") + "\n"
}

func hasCanonicalGameBanPersistenceBlock(content string) bool {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	markerIndex := -1
	customIndex := len(lines)
	for index, line := range lines {
		switch strings.TrimSpace(line) {
		case GameBanConfigMarker:
			if markerIndex != -1 {
				return false
			}
			markerIndex = index
		case ServerCustomConfigMarker:
			if customIndex == len(lines) {
				customIndex = index
			}
		}
	}
	if markerIndex < 0 || markerIndex+2 >= len(lines) || markerIndex >= customIndex {
		return false
	}
	if strings.TrimSpace(lines[markerIndex+1]) != gameBanConfigLines[1] ||
		strings.TrimSpace(lines[markerIndex+2]) != gameBanConfigLines[2] {
		return false
	}

	execCounts := map[string]int{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isGameBanExecDirective(trimmed) {
			execCounts[strings.ToLower(strings.Trim(strings.Fields(trimmed)[1], `"'`))]++
		}
	}
	return execCounts["banned_user.cfg"] == 1 && execCounts["banned_ip.cfg"] == 1
}

func isGameBanExecDirective(line string) bool {
	fields := strings.Fields(line)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "exec") {
		return false
	}
	name := strings.ToLower(strings.Trim(fields[1], `"'`))
	return name == "banned_user.cfg" || name == "banned_ip.cfg"
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func trimLeadingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return lines
}

func atomicWriteFile(path string, content []byte, mode os.FileMode) (returnErr error) {
	temp, err := os.CreateTemp(filepath.Dir(path), ".l4d2-manager-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if returnErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err := temp.Chmod(mode); err != nil {
		return err
	}
	if _, err := temp.Write(content); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
