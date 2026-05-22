package logic

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	LinkSourceWorkshop        = "workshop"
	LinkSourceQQFlashTransfer = "qq_flash_transfer"
)

type LinkParseItem struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Filename       string `json:"filename"`
	FileSize       string `json:"file_size"`
	FileURL        string `json:"file_url"`
	PreviewURL     string `json:"preview_url"`
	Referer        string `json:"referer"`
	Supported      bool   `json:"supported"`
	DisabledReason string `json:"disabled_reason"`
}

type LinkParseResult struct {
	SourceType string          `json:"source_type"`
	SourceID   string          `json:"source_id"`
	Items      []LinkParseItem `json:"items"`
}

func ParseDownloadLink(rawLink string) (LinkParseResult, error) {
	rawLink = strings.TrimSpace(rawLink)
	if rawLink == "" {
		return LinkParseResult{}, fmt.Errorf("链接不能为空")
	}

	if IsQQFlashTransferLink(rawLink) {
		return ParseQQFlashTransferLink(rawLink)
	}

	if _, err := ParseWorkshopID(rawLink); err == nil {
		return ParseWorkshopDownloadLinkAsGeneric(rawLink)
	}

	return LinkParseResult{}, fmt.Errorf("暂不支持该链接类型")
}

func ParseWorkshopDownloadLinkAsGeneric(workshopURL string) (LinkParseResult, error) {
	result, err := ParseWorkshopDownloadLink(workshopURL)
	if err != nil {
		return LinkParseResult{}, err
	}

	items := make([]LinkParseItem, 0, len(result.Items))
	for _, item := range result.Items {
		filename := cleanWorkshopDownloadFilename(item)
		items = append(items, LinkParseItem{
			ID:         item.PublishedFileID,
			Title:      item.Title,
			Filename:   filename,
			FileSize:   item.FileSize,
			FileURL:    item.FileURL,
			PreviewURL: item.PreviewURL,
			Supported:  true,
		})
	}

	return LinkParseResult{
		SourceType: LinkSourceWorkshop,
		SourceID:   result.SourceID,
		Items:      items,
	}, nil
}

func IsQQFlashTransferLink(rawLink string) bool {
	u, err := url.Parse(strings.TrimSpace(rawLink))
	if err != nil || u.Scheme == "" {
		return false
	}

	host := strings.ToLower(u.Hostname())
	if host != "qfile.qq.com" && !strings.HasSuffix(host, ".qfile.qq.com") {
		return false
	}

	return strings.HasPrefix(strings.TrimRight(u.EscapedPath(), "/"), "/q/")
}

func IsSupportedDownloadFilename(filename string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(filename))) {
	case ".vpk", ".zip", ".rar", ".7z":
		return true
	default:
		return false
	}
}

func downloadItemSupportState(filename string) (bool, string) {
	if IsSupportedDownloadFilename(filename) {
		return true, ""
	}
	return false, "仅支持 .vpk, .zip, .rar, .7z 文件"
}

func cleanWorkshopDownloadFilename(item WorkshopDownloadItem) string {
	filename := strings.TrimSpace(item.Filename)
	if filename == "" {
		filename = strings.TrimSpace(item.Title)
	}
	if filename == "" {
		filename = strings.TrimSpace(item.PublishedFileID)
	}
	if filename == "" {
		filename = "workshop"
	}

	if ext := strings.ToLower(filepath.Ext(filename)); ext == "" {
		filename += ".vpk"
	}

	return cleanLinkParseFilename(filename)
}

func cleanLinkParseFilename(filename string) string {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\x00", ""))
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = filepath.Base(filename)
	filename = strings.Trim(filename, " .")

	invalid := []string{`<`, `>`, `:`, `"`, `/`, `\`, `|`, `?`, `*`}
	for _, item := range invalid {
		filename = strings.ReplaceAll(filename, item, "_")
	}

	if filename == "" || filename == "." || filename == "/" {
		return ""
	}

	if len([]rune(filename)) > 180 {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		baseRunes := []rune(base)
		if len(baseRunes) > 160 {
			base = string(baseRunes[:160])
		}
		filename = base + ext
	}

	return filename
}
