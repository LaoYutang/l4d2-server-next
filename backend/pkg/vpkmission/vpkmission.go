package vpkmission

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"l4d2-manager-next/pkg/valve/vdf"
	"l4d2-manager-next/pkg/valve/vpk"
)

type Campaign struct {
	Title    string
	Chapters []*Chapter
	VpkName  string
}

type Chapter struct {
	Code  string
	Title string
	Modes []string
}

type FileError struct {
	Path string
	Err  error
}

func (e FileError) Error() string {
	if e.Path == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

func (e FileError) Unwrap() error {
	return e.Err
}

func ParseMission(r io.Reader) (*Campaign, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parse mission vdf: read input: %w", err)
	}

	result, err := parseMissionData(data)
	if err != nil {
		return nil, fmt.Errorf("parse mission vdf: %w", err)
	}
	return result.Campaign, nil
}

func ParseVPK(path string) ([]*Campaign, error) {
	opener := vpk.Single(path)
	defer opener.Close()

	archive, err := opener.ReadArchive()
	if err != nil {
		return nil, fmt.Errorf("read vpk archive: %w", err)
	}

	foundMission := false
	campaigns := make([]*Campaign, 0, 4)
	missionErrors := make([]string, 0)
	for _, archiveFile := range archive.Files {
		name := archiveFile.Name()
		if !isMissionFile(name) {
			continue
		}
		foundMission = true

		rc, err := archiveFile.Open(opener)
		if err != nil {
			return nil, fmt.Errorf("open mission file %s: %w", name, err)
		}

		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read mission file %s: %w", name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close mission file %s: %w", name, closeErr)
		}

		result, parseErr := parseMissionData(data)
		if parseErr != nil {
			missionErrors = append(missionErrors, fmt.Sprintf("%s: %v", name, parseErr))
			log.Printf("跳过无法解析的 mission 文件 %s（VPK: %s）: %v", name, filepath.Base(path), parseErr)
			continue
		}
		if result.Recovered {
			log.Printf(
				"已宽容恢复 mission 文件 %s（VPK: %s）: %s",
				name,
				filepath.Base(path),
				formatRecoveryIssues(result.Issues),
			)
		}

		result.Campaign.VpkName = filepath.Base(path)
		campaigns = mergeCampaignList(campaigns, result.Campaign)
	}

	if !foundMission {
		return nil, fmt.Errorf("vpk %s contains no mission files", filepath.Base(path))
	}
	if len(campaigns) == 0 {
		if len(missionErrors) > 0 {
			return nil, fmt.Errorf(
				"vpk %s contains no parsed campaigns: %s",
				filepath.Base(path),
				strings.Join(missionErrors, "; "),
			)
		}
		return nil, fmt.Errorf("vpk %s contains no parsed campaigns", filepath.Base(path))
	}

	return campaigns, nil
}

func ScanDir(dir string) ([]*Campaign, []FileError, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	campaigns := make([]*Campaign, 0, len(entries))
	fileErrors := make([]FileError, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".vpk") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		parsed, err := ParseVPK(path)
		if err != nil {
			fileErrors = append(fileErrors, FileError{Path: path, Err: err})
			continue
		}
		campaigns = append(campaigns, parsed...)
	}

	return campaigns, fileErrors, nil
}

func isMissionFile(name string) bool {
	lowerName := strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	return strings.HasPrefix(lowerName, "missions/") && strings.HasSuffix(lowerName, ".txt")
}

func chaptersFromMode(modeNode *vdf.KeyValues, mode string) []*Chapter {
	chapters := make([]*Chapter, 0, 8)
	for chapterNode := modeNode.FirstTrueSubKey(); chapterNode != nil; chapterNode = chapterNode.NextTrueSubKey() {
		mapName := valueForKey(chapterNode, "map")
		if mapName == "" {
			continue
		}

		chapters = append(chapters, &Chapter{
			Code:  mapName,
			Title: valueForKey(chapterNode, "displayname"),
			Modes: []string{mode},
		})
	}
	return chapters
}

func valueForKey(node *vdf.KeyValues, key string) string {
	value := node.FindKey(key)
	if value == nil || !value.HasValue {
		return ""
	}
	return value.Value
}

func mergeCampaignList(campaigns []*Campaign, campaign *Campaign) []*Campaign {
	key := campaignIdentity(campaign)
	if key == "" {
		return append(campaigns, campaign)
	}

	for _, existing := range campaigns {
		if campaignIdentity(existing) == key {
			mergeCampaign(existing, campaign)
			return campaigns
		}
	}
	return append(campaigns, campaign)
}

func campaignIdentity(campaign *Campaign) string {
	if campaign == nil {
		return ""
	}
	if campaign.Title != "" {
		return "title:" + campaign.Title
	}
	if len(campaign.Chapters) > 0 && campaign.Chapters[0].Code != "" {
		return "chapter:" + campaign.Chapters[0].Code
	}
	return ""
}

func mergeCampaign(base, additional *Campaign) {
	if base.Title == "" && additional.Title != "" {
		base.Title = additional.Title
	}
	if base.VpkName == "" && additional.VpkName != "" {
		base.VpkName = additional.VpkName
	}
	for _, chapter := range additional.Chapters {
		mergeChapter(base, chapter)
	}
}

func mergeChapter(campaign *Campaign, chapter *Chapter) {
	if chapter == nil || chapter.Code == "" {
		return
	}

	for _, existing := range campaign.Chapters {
		if existing.Code == chapter.Code {
			if existing.Title == "" && chapter.Title != "" {
				existing.Title = chapter.Title
			}
			existing.Modes = mergeModes(existing.Modes, chapter.Modes)
			return
		}
	}

	campaign.Chapters = append(campaign.Chapters, &Chapter{
		Code:  chapter.Code,
		Title: chapter.Title,
		Modes: mergeModes(nil, chapter.Modes),
	})
}

func mergeModes(base, additional []string) []string {
	seen := make(map[string]bool, len(base)+len(additional))
	for _, mode := range base {
		if mode == "" || seen[mode] {
			continue
		}
		seen[mode] = true
	}

	result := make([]string, 0, len(base)+len(additional))
	for _, mode := range base {
		if mode == "" {
			continue
		}
		if containsString(result, mode) {
			continue
		}
		result = append(result, mode)
	}
	for _, mode := range additional {
		if mode == "" || seen[mode] {
			continue
		}
		seen[mode] = true
		result = append(result, mode)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
