package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

const (
	DefaultMapCleanupRetentionDays = 3
	MaxMapCleanupRetentionDays     = 3650
	MapCleanupCheckTime            = "03:00"
)

var ErrInvalidMapCleanupConfig = errors.New("地图自动清理配置无效")

type MapCleanupSettings struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retention_days"`
}

type MapCleanupConfig struct {
	MapCleanupSettings
	CheckTime   string    `json:"check_time"`
	Timezone    string    `json:"timezone"`
	NextCheckAt time.Time `json:"next_check_at"`
}

func ValidateMapCleanupConfig(enabled bool, retentionDays int) error {
	if retentionDays < 1 || retentionDays > MaxMapCleanupRetentionDays {
		return fmt.Errorf("%w: 保留天数必须为 1 至 %d 的整数", ErrInvalidMapCleanupConfig, MaxMapCleanupRetentionDays)
	}
	return nil
}

// Missing fields are the legacy format; explicit nulls and wrong types are errors.
// A cleanup flag from a partly decoded file must never enable destructive work.
func validateStoredMapCleanupConfig(data []byte, settings MapCleanupSettings) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return fmt.Errorf("解析面板配置失败: 配置必须为 JSON 对象")
	}
	if raw, exists := root["map_cleanup"]; exists {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
			return fmt.Errorf("解析地图自动清理配置失败: 配置必须为 JSON 对象")
		}
		for _, name := range []string{"enabled", "retention_days"} {
			if value, ok := fields[name]; ok && string(value) == "null" {
				return fmt.Errorf("解析地图自动清理配置失败: %s 不能为 null", name)
			}
		}
	}
	return ValidateMapCleanupConfig(settings.Enabled, settings.RetentionDays)
}

func GetMapCleanupConfig() (MapCleanupConfig, error) {
	managerConfigMutex.RLock()
	defer managerConfigMutex.RUnlock()
	if managerConfigLoadError != nil {
		return MapCleanupConfig{}, managerConfigLoadError
	}
	return mapCleanupConfigAt(managerConfig.MapCleanup, time.Now(), time.Local), nil
}

func SetMapCleanupConfig(enabled bool, retentionDays int) (MapCleanupConfig, error) {
	if err := ValidateMapCleanupConfig(enabled, retentionDays); err != nil {
		return MapCleanupConfig{}, err
	}
	// The worker holds the same gate through its deletion batch. Once disabling
	// returns successfully, no deletion can still start using the old setting.
	MapLifecycleMutex.Lock()
	defer MapLifecycleMutex.Unlock()
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	if managerConfigLoadError != nil {
		return MapCleanupConfig{}, managerConfigLoadError
	}
	candidate := *managerConfig
	candidate.MapCleanup = MapCleanupSettings{Enabled: enabled, RetentionDays: retentionDays}
	if err := persistManagerConfig(&candidate); err != nil {
		return MapCleanupConfig{}, fmt.Errorf("保存地图自动清理配置失败: %w", err)
	}
	managerConfig = &candidate
	return mapCleanupConfigAt(candidate.MapCleanup, time.Now(), time.Local), nil
}

func mapCleanupConfigAt(settings MapCleanupSettings, now time.Time, location *time.Location) MapCleanupConfig {
	local := now.In(location)
	zone, offset := local.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return MapCleanupConfig{
		MapCleanupSettings: settings,
		CheckTime:          MapCleanupCheckTime,
		Timezone:           fmt.Sprintf("%s (%s, UTC%s%02d:%02d)", location.String(), zone, sign, offset/3600, offset%3600/60),
		NextCheckAt:        nextMapCleanupAt(now, location),
	}
}

// nextMapCleanupAt always returns a future local-calendar occurrence. Restarting
// after 03:00 therefore waits until the following day instead of catching up.
func nextMapCleanupAt(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	next := time.Date(local.Year(), local.Month(), local.Day(), 3, 0, 0, 0, location)
	if !next.After(now) {
		next = time.Date(local.Year(), local.Month(), local.Day()+1, 3, 0, 0, 0, location)
	}
	return next
}

// StartMapCleanup starts one worker. main calls it once after lifecycle recovery.
// Neither starting this worker nor saving settings runs a cleanup immediately.
func StartMapCleanup(ctx context.Context) {
	go runMapCleanup(ctx, time.Now, time.Local, waitForMapCleanup, runScheduledMapCleanup)
}

func waitForMapCleanup(ctx context.Context, due time.Time) bool {
	timer := time.NewTimer(time.Until(due))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

// The injected clock and waiter let tests advance days without waiting or
// touching any installed map. Recompute calendar dates rather than ticking 24h.
func runMapCleanup(ctx context.Context, now func() time.Time, location *time.Location, wait func(context.Context, time.Time) bool, cleanup func(time.Time)) {
	due := nextMapCleanupAt(now(), location)
	for {
		if ctx.Err() != nil || !wait(ctx, due) || ctx.Err() != nil {
			return
		}
		current := now()
		if current.Before(due) {
			// The wall clock moved backwards after creating the timer.
			continue
		}
		cleanup(current)
		next := nextMapCleanupAt(now(), location)
		// A clock rollback after a check must not run this local date twice.
		if !next.After(due) {
			local := due.In(location)
			next = time.Date(local.Year(), local.Month(), local.Day()+1, 3, 0, 0, 0, location)
		}
		due = next
	}
}

func runScheduledMapCleanup(now time.Time) {
	MapLifecycleMutex.Lock()
	defer MapLifecycleMutex.Unlock()
	managerConfigMutex.RLock()
	settings, configErr := managerConfig.MapCleanup, managerConfigLoadError
	managerConfigMutex.RUnlock()
	if configErr != nil {
		log.Printf("[MAP CLEANUP] 配置读取失败，本次检查已跳过: %v", configErr)
		return
	}
	if !settings.Enabled {
		return
	}
	result, err := DeleteExpiredMapsLocked(now, settings.RetentionDays)
	if err != nil {
		log.Printf("[MAP CLEANUP] 清理失败: %v", err)
		return
	}
	log.Printf("[MAP CLEANUP] 每日检查完成: %+v", result)
}
