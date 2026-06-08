package logic

import (
	"l4d2-manager-next/consts"
	"l4d2-manager-next/pkg/vpkmission"
	"log"
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
