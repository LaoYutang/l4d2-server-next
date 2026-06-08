package vpkmission

import (
	"fmt"
	"io"
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
	root, err := vdf.ReadTolerant(r)
	if err != nil {
		return nil, fmt.Errorf("parse mission vdf: %w", err)
	}

	return campaignFromKeyValues(root), nil
}

func campaignFromKeyValues(root *vdf.KeyValues) *Campaign {
	campaign := &Campaign{
		Title:    findFirstValue(root, "displaytitle"),
		Chapters: make([]*Chapter, 0, 8),
	}

	walkKeys(root, func(node *vdf.KeyValues) {
		mode := strings.ToLower(node.Key)
		if mode == "" {
			return
		}
		for _, chapter := range chaptersFromMode(node, mode) {
			mergeChapter(campaign, chapter)
		}
	})

	return campaign
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

		campaign, parseErr := ParseMission(rc)
		closeErr := rc.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("parse mission file %s: %w", name, parseErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close mission file %s: %w", name, closeErr)
		}

		campaign.VpkName = filepath.Base(path)
		campaigns = mergeCampaignList(campaigns, campaign)
	}

	if !foundMission {
		return nil, fmt.Errorf("vpk %s contains no mission files", filepath.Base(path))
	}
	if len(campaigns) == 0 {
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

func findFirstValue(root *vdf.KeyValues, key string) string {
	var found string
	walkKeys(root, func(node *vdf.KeyValues) {
		if found != "" {
			return
		}
		found = valueForKey(node, key)
	})
	return found
}

func valueForKey(node *vdf.KeyValues, key string) string {
	value := node.FindKey(key)
	if value == nil || !value.HasValue {
		return ""
	}
	return value.Value
}

func walkKeys(root *vdf.KeyValues, visit func(*vdf.KeyValues)) {
	for node := root; node != nil; node = node.NextSubKey() {
		visit(node)
		for child := node.FirstTrueSubKey(); child != nil; child = child.NextTrueSubKey() {
			walkKeys(child, visit)
		}
	}
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
