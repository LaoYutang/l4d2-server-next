package logic

import (
	"fmt"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/vpkmission"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Campaign = vpkmission.Campaign
type Chapter = vpkmission.Chapter

// 获取章节列表
func GetChapterList() []*Campaign {
	campaigns, fileErrors, err := vpkmission.ScanDir(consts.AddonsBasePath)
	if err != nil {
		log.Printf("读取目录失败: %v", err)
		return nil
	}
	for _, fileErr := range fileErrors {
		log.Printf("解析 VPK 任务文件失败: %v", fileErr)
	}

	result := make([]*Campaign, 0, len(campaigns))
	seenTitles := make(map[string]bool)
	for _, campaign := range campaigns {
		if campaign == nil {
			continue
		}
		if seenTitles[campaign.Title] {
			continue
		}
		seenTitles[campaign.Title] = true
		result = append(result, campaign)
	}

	return result
}

func GetMapMissionDetail(mapName string) ([]*Campaign, error) {
	mapName = strings.TrimSpace(mapName)
	if mapName == "" ||
		strings.Contains(mapName, "\x00") ||
		strings.Contains(mapName, "/") ||
		strings.Contains(mapName, "\\") ||
		filepath.IsAbs(mapName) ||
		filepath.Base(mapName) != mapName {
		return nil, fmt.Errorf("invalid map filename")
	}
	if !strings.EqualFold(filepath.Ext(mapName), ".vpk") {
		return nil, fmt.Errorf("invalid map filename: only .vpk files are supported")
	}

	vpkPath := filepath.Join(consts.AddonsBasePath, mapName)
	info, err := os.Stat(vpkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("map file %s does not exist", mapName)
		}
		return nil, fmt.Errorf("stat map file %s: %w", mapName, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("map file %s is a directory", mapName)
	}

	return vpkmission.ParseVPK(vpkPath)
}
