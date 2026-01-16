package logic

import (
	"bufio"
	"fmt"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type AdminUser struct {
	SteamID string `json:"steamid"`
	Remark  string `json:"remark"`
}

func getAdminsFilePath() string {
	return filepath.Join(consts.GamePath, "addons", "sourcemod", "configs", "admins_simple.ini")
}

// ParseAdminsSimple 解析 admins_simple.ini 文件
func ParseAdminsSimple() ([]AdminUser, error) {
	path := getAdminsFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("SourceMod 未启用或配置文件不存在")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var admins []AdminUser
	scanner := bufio.NewScanner(file)

	// 正则匹配: "SteamID" "Flags" (可选 "Password") // 注释
	// 示例: "STEAM_1:1:123456" "99:z" // 我的管理员
	// 我们只关心 SteamID 和 注释 (Remark)。Flags 保留原样。
	// 我们假设第一个引号内的字符串是 SteamID。

	// 基础正则匹配引号内的内容
	re := regexp.MustCompile(`"([^"]+)"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// 提取注释作为备注
		remark := ""
		if idx := strings.Index(line, "//"); idx != -1 {
			remark = strings.TrimSpace(line[idx+2:])
			line = line[:idx] // 从解析部分移除注释
		}

		matches := re.FindAllStringSubmatch(line, -1)
		if len(matches) >= 2 {
			// 至少包含 SteamID 和 Flags
			steamID := matches[0][1]
			// flags := matches[1][1] // 忽略 Flags

			// 简单的 SteamID 验证
			// 原逻辑只允许 STEAM_ 或 [U: 开头，这导致 "1" 这种简写 ID 被忽略
			// 现在放宽限制：只要非空即可，SourceMod 本身支持多种格式（IP、Name、SteamID等）
			if steamID != "" {
				admins = append(admins, AdminUser{
					SteamID: steamID,
					Remark:  remark,
				})
			}
		}
	}

	return admins, scanner.Err()
}

// AddAdmin 追加新管理员到文件
func AddAdmin(steamID, remark string) error {
	path := getAdminsFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("SourceMod 未启用或配置文件不存在")
	}

	// 读取现有列表以检查重复
	admins, err := ParseAdminsSimple()
	if err != nil {
		return err
	}

	for _, admin := range admins {
		if admin.SteamID == steamID {
			return fmt.Errorf("该 SteamID 已存在")
		}
	}

	// 追加到文件
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// 格式: "STEAM_..." "99:z" // 备注
	line := fmt.Sprintf("\n\"%s\" \"99:z\" // %s", steamID, remark)
	if _, err := f.WriteString(line); err != nil {
		return err
	}

	return nil
}

// DeleteAdmin 根据 SteamID 删除管理员 (重写文件)
func DeleteAdmin(steamID string) error {
	path := getAdminsFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("SourceMod 未启用或配置文件不存在")
	}

	// 读取所有行
	input, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(input), "\n")
	var newLines []string
	deleted := false

	// 正则用于识别包含特定 SteamID 的行
	re := regexp.MustCompile(`"([^"]+)"`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检查此行是否包含我们要删除的 SteamID
		// 我们只检查有效行，不检查纯注释行
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			// 移除注释部分以便匹配
			codePart := trimmed
			if idx := strings.Index(trimmed, "//"); idx != -1 {
				codePart = trimmed[:idx]
			}

			matches := re.FindAllStringSubmatch(codePart, -1)
			if len(matches) > 0 {
				currentSteamID := matches[0][1]
				if currentSteamID == steamID {
					deleted = true
					continue // 跳过此行
				}
			}
		}
		newLines = append(newLines, line)
	}

	if !deleted {
		return fmt.Errorf("未找到该管理员")
	}

	output := strings.Join(newLines, "\n")
	return os.WriteFile(path, []byte(output), 0644)
}
