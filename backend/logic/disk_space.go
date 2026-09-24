package logic

import (
	"fmt"
	"l4d2-manager-next/consts"

	"github.com/shirou/gopsutil/v3/disk"
)

const (
	DefaultDiskUsageLimitPercent = 90
	MinDiskUsageLimitPercent     = 80
	MaxDiskUsageLimitPercent     = 99

	// UploadSpaceFactor 估算上传峰值占用：分片临时文件 + 合并/解压产物。
	UploadSpaceFactor = 2
)

// DiskSpaceBlockReason 说明放行判定被哪种原因拦截。空字符串表示放行。
type DiskSpaceBlockReason string

const (
	// DiskBlockUsageExceeded 使用率超过配置上限，管理员可以二次确认后强制上传。
	DiskBlockUsageExceeded DiskSpaceBlockReason = "disk_usage_exceeded"
	// DiskBlockFreeInsufficient 预判剩余空间不足，任何角色都不能继续。
	DiskBlockFreeInsufficient DiskSpaceBlockReason = "disk_space_insufficient"
)

// DiskUsage 是 addons 目录所在分区的使用情况快照。
type DiskUsage struct {
	UsedPercent float64
	Total       uint64
	Free        uint64
}

// DiskSpaceDecision 是磁盘检查结论，附带拼装错误响应所需的原始数值。
type DiskSpaceDecision struct {
	BlockReason   DiskSpaceBlockReason
	UsedPercent   float64
	LimitPercent  int
	FreeBytes     uint64
	TotalBytes    uint64
	RequiredBytes uint64
}

// Blocked 表示本次上传或下载被拒绝。
func (d DiskSpaceDecision) Blocked() bool {
	return d.BlockReason != ""
}

// QueryAddonsDiskUsage 读取 addons 目录所在分区的使用情况。
func QueryAddonsDiskUsage() (DiskUsage, error) {
	stat, err := disk.Usage(consts.AddonsBasePath)
	if err != nil {
		return DiskUsage{}, err
	}
	return DiskUsage{
		UsedPercent: stat.UsedPercent,
		Total:       stat.Total,
		Free:        stat.Free,
	}, nil
}

func NormalizeDiskUsageLimitPercent(percent int) (int, error) {
	if percent < MinDiskUsageLimitPercent || percent > MaxDiskUsageLimitPercent {
		return 0, fmt.Errorf("磁盘使用率上限必须在 %d - %d 之间", MinDiskUsageLimitPercent, MaxDiskUsageLimitPercent)
	}
	return percent, nil
}

// IsDiskUsageLimitPercentValid 判断持久化的阈值是否落在允许范围内。
func IsDiskUsageLimitPercentValid(percent int) bool {
	_, err := NormalizeDiskUsageLimitPercent(percent)
	return err == nil
}

// GetDiskUsageLimitPercent 读取配置的磁盘使用率上限，越界值按默认值处理。
func GetDiskUsageLimitPercent() int {
	managerConfigMutex.RLock()
	defer managerConfigMutex.RUnlock()

	if !IsDiskUsageLimitPercentValid(managerConfig.DiskUsageLimitPercent) {
		return DefaultDiskUsageLimitPercent
	}
	return managerConfig.DiskUsageLimitPercent
}

// SetDiskUsageLimitPercent 校验并持久化磁盘使用率上限。
func SetDiskUsageLimitPercent(percent int) (int, error) {
	normalized, err := NormalizeDiskUsageLimitPercent(percent)
	if err != nil {
		return 0, err
	}

	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	managerConfig.DiskUsageLimitPercent = normalized
	return normalized, saveManagerConfig()
}

// EvaluateDiskSpace 判定本次写入是否放行，不涉及角色与 IO：
//   - fileSize > 0 时按 UploadSpaceFactor 估算峰值占用，剩余空间不够直接拒绝，
//     这种情况属于预判写满，adminForced 也不能绕过；
//   - 使用率未超过阈值时放行（恰好等于阈值视为未超过）；
//   - 使用率超过阈值时只有 adminForced 才能放行。
//
// fileSize 为 0 表示调用方无法预知文件大小，跳过硬拦截。
func EvaluateDiskSpace(usage DiskUsage, limitPercent int, fileSize int64, adminForced bool) DiskSpaceDecision {
	decision := DiskSpaceDecision{
		UsedPercent:  usage.UsedPercent,
		LimitPercent: limitPercent,
		FreeBytes:    usage.Free,
		TotalBytes:   usage.Total,
	}

	if fileSize > 0 {
		decision.RequiredBytes = uint64(fileSize) * UploadSpaceFactor
		if usage.Free < decision.RequiredBytes {
			decision.BlockReason = DiskBlockFreeInsufficient
			return decision
		}
	}

	if usage.UsedPercent <= float64(limitPercent) {
		return decision
	}

	if adminForced {
		return decision
	}

	decision.BlockReason = DiskBlockUsageExceeded
	return decision
}
