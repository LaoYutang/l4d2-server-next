package logic

import (
	"bufio"
	"io"
	"l4d2-manager-next/consts"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"git.lubar.me/ben/valve/vpk"
)

// 获取章节列表
func GetChapterList() []*Campaign {
	temp := make([]*Campaign, 0, 16)

	// 扫描addons下的所有vpk文件
	entries, err := os.ReadDir(filepath.Join(consts.AddonsBasePath))
	if err != nil {
		log.Printf("读取目录失败: %v", err)
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".vpk") {
			opener := vpk.Single(filepath.Join(consts.AddonsBasePath, entry.Name()))
			defer opener.Close()

			archive, err := opener.ReadArchive()
			if err != nil {
				log.Printf("Failed to read VPK archive %s: %v", entry.Name(), err)
				continue
			}

			// 查找所有 missions/*.txt 文件
			var missionFiles []*vpk.File
			for _, file := range archive.Files {
				if strings.HasPrefix(file.Name(), "missions/") && strings.HasSuffix(file.Name(), ".txt") {
					fileCopy := file
					missionFiles = append(missionFiles, &fileCopy)
				}
			}
			if len(missionFiles) == 0 {
				log.Printf("在 VPK %s 中未找到任务文件", entry.Name())
				continue
			}

			// 解析并合并所有 mission 文件
			var campaign *Campaign
			for i, missionFile := range missionFiles {
				rc, err := missionFile.Open(opener)
				if err != nil {
					log.Printf("打开 vpk %s 中任务文件 %s 失败: %v", entry.Name(), missionFile.Name(), err)
					continue
				}

				parsedCampaign, err := parseMissionFile(rc)
				rc.Close()
				if err != nil {
					log.Printf("解析 %s 任务文件 %s 失败: %v", entry.Name(), missionFile.Name(), err)
					continue
				}

				// 第一个文件作为基础
				if i == 0 {
					campaign = parsedCampaign
				} else {
					// 合并后续文件的章节和模式
					campaign = mergeCampaigns(campaign, parsedCampaign)
				}
			}

			if campaign == nil {
				log.Printf("VPK %s 中没有成功解析任何任务文件", entry.Name())
				continue
			}

			campaign.VpkName = entry.Name()

			// 如果已经存在相同的战役，则跳过
			exist := false
			for _, cam := range temp {
				if cam.Title == campaign.Title {
					exist = true
					break
				}
			}
			if exist {
				continue
			}
			temp = append(temp, campaign)
		}
	}

	return temp
}

// mergeCampaigns 合并两个战役数据，将第二个战役的章节和模式合并到第一个中
func mergeCampaigns(base, additional *Campaign) *Campaign {
	if base == nil {
		return additional
	}
	if additional == nil {
		return base
	}

	// 如果基础战役没有标题，使用额外战役的标题
	if base.Title == "" && additional.Title != "" {
		base.Title = additional.Title
	}

	// 创建章节映射表，用于快速查找和去重
	chapterMap := make(map[string]*Chapter)
	for _, chapter := range base.Chapters {
		chapterMap[chapter.Code] = chapter
	}

	// 合并额外战役的章节
	for _, addChapter := range additional.Chapters {
		if existingChapter, exists := chapterMap[addChapter.Code]; exists {
			// 章节已存在，合并游戏模式
			existingChapter.Modes = mergeUniqueModes(existingChapter.Modes, addChapter.Modes)
			// 如果现有章节没有标题，使用新的标题
			if existingChapter.Title == "" && addChapter.Title != "" {
				existingChapter.Title = addChapter.Title
			}
		} else {
			// 新章节，直接添加
			base.Chapters = append(base.Chapters, addChapter)
			chapterMap[addChapter.Code] = addChapter
		}
	}

	return base
}

// mergeUniqueModes 合并两个模式列表，去除重复项
func mergeUniqueModes(modes1, modes2 []string) []string {
	modeSet := make(map[string]bool)
	for _, mode := range modes1 {
		modeSet[mode] = true
	}
	for _, mode := range modes2 {
		if !modeSet[mode] {
			modes1 = append(modes1, mode)
			modeSet[mode] = true
		}
	}
	return modes1
}

type Campaign struct {
	Title    string
	Chapters []*Chapter
	VpkName  string
}

type Chapter struct {
	Code  string
	Title string
	Modes []string // 支持的游戏模式
}

// 使用正则表达式来更可靠地提取键值对
var kvRegex = regexp.MustCompile(`"([^"]+)"\s+"([^"]+)"`)

func parseMissionFile(reader io.Reader) (*Campaign, error) {
	scanner := bufio.NewScanner(reader)
	campaign := &Campaign{
		Chapters: make([]*Chapter, 0, 8), // 预分配容量
	}

	inGameModeSection := false
	braceLevel := 0
	var tempMapName string
	var currentMode string
	seenChapters := make(map[string]*Chapter) // 用于去重和追加模式

	for scanner.Scan() {
		line := scanner.Text()

		if len(line) == 0 {
			continue
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//") {
			continue
		}

		// 状态机，用于进入/退出游戏模式部分
		// 支持多种游戏模式：coop, survival, halftank, brawler 等
		lowerLine := strings.ToLower(line)

		// 移除行内注释（//之后的内容）
		if commentIndex := strings.Index(lowerLine, "//"); commentIndex != -1 {
			lowerLine = strings.TrimSpace(lowerLine[:commentIndex])
		}

		if !inGameModeSection && (lowerLine == `"coop"` || lowerLine == `"survival"` ||
			lowerLine == `"halftank"` || lowerLine == `"brawler"` || lowerLine == `"versus"` ||
			lowerLine == `"scavenge"` || lowerLine == `"realism"`) {
			inGameModeSection = true
			braceLevel = 0
			// 提取模式名称，去除引号
			currentMode = strings.Trim(lowerLine, `"`)
			continue
		}

		if inGameModeSection {
			for _, char := range line {
				switch char {
				case '{':
					braceLevel++
				case '}':
					braceLevel--
				}
			}

			// 如果 braceLevel 降为 0，说明已退出游戏模式部分
			// 我们检查 braceLevel <= 0 是为了防止文件格式不规范
			if braceLevel <= 0 {
				inGameModeSection = false
				currentMode = ""
				continue
			}
		}

		// 只在必要时使用正则表达式
		if strings.Contains(line, `"`) {
			matches := kvRegex.FindStringSubmatch(line)
			if len(matches) == 3 {
				key := strings.ToLower(matches[1])
				value := matches[2]

				// 始终在文件顶层查找战役标题
				if key == "displaytitle" && campaign.Title == "" {
					campaign.Title = value
				}

				// 如果在游戏模式区域内
				if inGameModeSection {
					if key == "map" {
						tempMapName = value
					}

					if key == "displayname" && tempMapName != "" {
						// 检查是否已经添加过这个章节
						if chapter, exists := seenChapters[tempMapName]; exists {
							// 已存在，添加模式到该章节
							chapter.Modes = append(chapter.Modes, currentMode)
						} else {
							// 不存在，创建新章节
							chapter := &Chapter{
								Code:  tempMapName,
								Title: value,
								Modes: []string{currentMode},
							}
							campaign.Chapters = append(campaign.Chapters, chapter)
							seenChapters[tempMapName] = chapter
						}
						tempMapName = "" // 重置
					}
				}
			}
		}
	}

	return campaign, scanner.Err()
}
