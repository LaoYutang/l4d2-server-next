package logic

import (
	"fmt"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/vpkmission"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Campaign = vpkmission.Campaign
type Chapter = vpkmission.Chapter

type MapSummary struct {
	Title        string   `json:"title"`
	Campaigns    []string `json:"campaigns"`
	ChapterCount int      `json:"chapter_count"`
	Error        string   `json:"error"`
}

type mapSummaryCacheEntry struct {
	size    int64
	modTime int64
	summary MapSummary
}

var (
	mapSummaryCacheMu  sync.RWMutex
	mapSummaryCache    = make(map[string]mapSummaryCacheEntry)
	parseMapSummaryVPK = vpkmission.ParseVPK
)

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
	mapName, err := NormalizeMapVPKName(mapName)
	if err != nil {
		return nil, fmt.Errorf("invalid map filename")
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

func GetMapSummaries(mapNames []string) map[string]MapSummary {
	result := make(map[string]MapSummary, len(mapNames))
	if len(mapNames) == 0 {
		return result
	}

	allowedMaps, err := readAllowedMapNames()
	if err != nil {
		for _, name := range mapNames {
			if strings.TrimSpace(name) != "" {
				result[name] = MapSummary{Error: fmt.Sprintf("读取地图记录失败: %v", err)}
			}
		}
		return result
	}

	for _, requestedName := range mapNames {
		mapName := strings.TrimSpace(requestedName)
		if mapName == "" {
			continue
		}
		if _, exists := result[mapName]; exists {
			continue
		}

		normalizedName, err := NormalizeMapVPKName(mapName)
		if err != nil {
			result[mapName] = MapSummary{Error: "地图名称无效"}
			continue
		}
		if !allowedMaps[normalizedName] {
			result[mapName] = MapSummary{Error: "地图记录不存在"}
			continue
		}

		summary := getMapSummary(normalizedName)
		result[mapName] = summary
	}
	return result
}

func getMapSummary(mapName string) MapSummary {
	vpkPath := filepath.Join(consts.AddonsBasePath, mapName)
	info, err := os.Stat(vpkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return MapSummary{Error: "地图文件不存在"}
		}
		return MapSummary{Error: fmt.Sprintf("检查地图文件失败: %v", err)}
	}
	if info.IsDir() {
		return MapSummary{Error: "地图文件是目录"}
	}

	cacheEntry, ok := getCachedMapSummary(mapName, info)
	if ok {
		return cacheEntry
	}

	campaigns, err := parseMapSummaryVPK(vpkPath)
	if err != nil {
		summary := MapSummary{Error: fmt.Sprintf("解析地图名称失败: %v", err)}
		setCachedMapSummary(mapName, info, summary)
		return summary
	}

	summary := buildMapSummary(campaigns)
	setCachedMapSummary(mapName, info, summary)
	return summary
}

func getCachedMapSummary(mapName string, info os.FileInfo) (MapSummary, bool) {
	mapSummaryCacheMu.RLock()
	defer mapSummaryCacheMu.RUnlock()

	entry, ok := mapSummaryCache[mapName]
	if !ok {
		return MapSummary{}, false
	}
	if entry.size != info.Size() || entry.modTime != info.ModTime().UnixNano() {
		return MapSummary{}, false
	}
	return entry.summary, true
}

func setCachedMapSummary(mapName string, info os.FileInfo, summary MapSummary) {
	mapSummaryCacheMu.Lock()
	defer mapSummaryCacheMu.Unlock()

	mapSummaryCache[mapName] = mapSummaryCacheEntry{
		size:    info.Size(),
		modTime: info.ModTime().UnixNano(),
		summary: summary,
	}
}

func buildMapSummary(campaigns []*Campaign) MapSummary {
	titles := make([]string, 0, len(campaigns))
	seenTitles := make(map[string]bool, len(campaigns))
	chapterCount := 0

	for _, campaign := range campaigns {
		if campaign == nil {
			continue
		}
		title := strings.TrimSpace(campaign.Title)
		if title == "" {
			title = "未命名战役"
		}
		if !seenTitles[title] {
			seenTitles[title] = true
			titles = append(titles, title)
		}
		chapterCount += len(campaign.Chapters)
	}

	return MapSummary{
		Title:        strings.Join(titles, " / "),
		Campaigns:    titles,
		ChapterCount: chapterCount,
	}
}

func readAllowedMapNames() (map[string]bool, error) {
	mapListBytes, err := os.ReadFile(consts.MapListFilePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(mapListBytes), "\n")
	allowed := make(map[string]bool, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" {
			allowed[name] = true
		}
	}
	return allowed, nil
}

func NormalizeMapVPKName(mapName string) (string, error) {
	mapName = strings.TrimSpace(mapName)
	if mapName == "" ||
		strings.Contains(mapName, "\x00") ||
		strings.Contains(mapName, "/") ||
		strings.Contains(mapName, "\\") ||
		filepath.IsAbs(mapName) ||
		filepath.Base(mapName) != mapName {
		return "", fmt.Errorf("invalid map filename")
	}
	if !strings.EqualFold(filepath.Ext(mapName), ".vpk") {
		return "", fmt.Errorf("invalid map filename: only .vpk files are supported")
	}
	return mapName, nil
}

func resetMapSummaryCacheForTest() {
	mapSummaryCacheMu.Lock()
	defer mapSummaryCacheMu.Unlock()
	mapSummaryCache = make(map[string]mapSummaryCacheEntry)
}
