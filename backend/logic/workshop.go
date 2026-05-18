package logic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const workshopParseAPIURL = "https://l4d2-workshop-parse.laoyutang.cn"

type WorkshopChild struct {
	PublishedFileID string `json:"publishedfileid"`
}

type WorkshopDownloadItem struct {
	Result          int             `json:"result,omitempty"`
	PublishedFileID string          `json:"publishedfileid"`
	Title           string          `json:"title"`
	Filename        string          `json:"filename"`
	FileSize        string          `json:"file_size"`
	FileURL         string          `json:"file_url"`
	PreviewURL      string          `json:"preview_url"`
	Children        []WorkshopChild `json:"children,omitempty"`
}

type WorkshopParseResult struct {
	SourceID string                 `json:"source_id"`
	Items    []WorkshopDownloadItem `json:"items"`
}

func ParseWorkshopDownloadLink(workshopURL string) (WorkshopParseResult, error) {
	id, err := ParseWorkshopID(workshopURL)
	if err != nil {
		return WorkshopParseResult{}, err
	}

	details, err := fetchWorkshopDetails([]string{id})
	if err != nil {
		return WorkshopParseResult{}, err
	}
	if len(details) == 0 {
		return WorkshopParseResult{}, fmt.Errorf("未找到工坊文件")
	}

	items := details
	if len(details[0].Children) > 0 {
		childIDs := make([]string, 0, len(details[0].Children))
		for _, child := range details[0].Children {
			childID := strings.TrimSpace(child.PublishedFileID)
			if childID != "" {
				childIDs = append(childIDs, childID)
			}
		}

		childDetails := make([]WorkshopDownloadItem, 0)
		if len(childIDs) > 0 {
			childDetails, err = fetchWorkshopDetails(childIDs)
			if err != nil {
				return WorkshopParseResult{}, fmt.Errorf("解析合集/依赖详情失败: %v", err)
			}
		}

		items = childDetails
		if isValidWorkshopDownloadItem(details[0]) {
			items = append([]WorkshopDownloadItem{details[0]}, items...)
		}
	}

	validItems := normalizeWorkshopItems(items)
	if len(validItems) == 0 {
		return WorkshopParseResult{}, fmt.Errorf("未找到可下载的工坊文件")
	}

	return WorkshopParseResult{
		SourceID: id,
		Items:    validItems,
	}, nil
}

func ParseWorkshopID(workshopURL string) (string, error) {
	workshopURL = strings.TrimSpace(workshopURL)
	if isValidWorkshopID(workshopURL) {
		return workshopURL, nil
	}

	u, err := url.Parse(workshopURL)
	if err != nil {
		return "", fmt.Errorf("无效的工坊链接")
	}

	host := strings.ToLower(u.Hostname())
	isSteamHost := strings.HasSuffix(host, "steamcommunity.com") ||
		strings.HasSuffix(host, "steampowered.com") ||
		strings.HasSuffix(host, "steamworkshop.download")

	if isSteamHost {
		id := u.Query().Get("id")
		if isValidWorkshopID(id) {
			return id, nil
		}
	}

	if isSteamHost || strings.Contains(workshopURL, "steamcommunity.com/sharedfiles") ||
		strings.Contains(workshopURL, "steamcommunity.com/workshop") {
		re := regexp.MustCompile(`\d+`)
		match := re.FindString(workshopURL)
		if isValidWorkshopID(match) {
			return match, nil
		}
	}

	return "", fmt.Errorf("未找到有效的工坊 ID")
}

func fetchWorkshopDetails(ids []string) ([]WorkshopDownloadItem, error) {
	payload, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, workshopParseAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("解析服务返回 HTTP %d", resp.StatusCode)
	}

	var details []WorkshopDownloadItem
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}

	return details, nil
}

func normalizeWorkshopItems(items []WorkshopDownloadItem) []WorkshopDownloadItem {
	result := make([]WorkshopDownloadItem, 0, len(items))
	bestByID := make(map[string]WorkshopDownloadItem)
	bestScoreByID := make(map[string]int)
	order := make([]string, 0, len(items))

	for _, item := range items {
		if !isValidWorkshopDownloadItem(item) {
			continue
		}

		id := strings.TrimSpace(item.PublishedFileID)
		item.PublishedFileID = id
		item.FileURL = strings.TrimSpace(item.FileURL)
		item.Filename = cleanWorkshopFilename(item.Filename)
		// Ant Design Table treats a "children" field as tree data. The API response
		// should only contain flat downloadable rows; child IDs are resolved above.
		item.Children = nil

		score := workshopItemQualityScore(item)
		if score == 0 {
			continue
		}

		if _, exists := bestByID[id]; !exists {
			order = append(order, id)
			bestByID[id] = item
			bestScoreByID[id] = score
			continue
		}

		if score > bestScoreByID[id] {
			bestByID[id] = item
			bestScoreByID[id] = score
		}
	}

	for _, id := range order {
		result = append(result, bestByID[id])
	}

	return result
}

func isValidWorkshopDownloadItem(item WorkshopDownloadItem) bool {
	return item.Result == 1 &&
		strings.TrimSpace(item.PublishedFileID) != "" &&
		strings.TrimSpace(item.FileURL) != ""
}

func workshopItemQualityScore(item WorkshopDownloadItem) int {
	score := 0
	if strings.TrimSpace(item.Title) != "" {
		score += 4
	}
	if strings.TrimSpace(item.Filename) != "" {
		score += 3
	}
	if strings.TrimSpace(item.FileSize) != "" && strings.TrimSpace(item.FileSize) != "0" {
		score += 2
	}
	if strings.TrimSpace(item.PreviewURL) != "" {
		score++
	}
	return score
}

func cleanWorkshopFilename(filename string) string {
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" {
		return ""
	}

	lowerName := strings.ToLower(filename)
	for _, prefix := range []string{"my l4d2addons", "myl4d2addons"} {
		if strings.HasPrefix(lowerName, prefix) {
			filename = strings.TrimLeft(filename[len(prefix):], " _-")
			lowerName = strings.ToLower(filename)
		}
	}

	return filename
}

func isValidWorkshopID(id string) bool {
	if id == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^\d+$`, id)
	if !matched {
		return false
	}
	num, err := strconv.ParseInt(id, 10, 64)
	return err == nil && num >= 100000
}
