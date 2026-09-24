package logic

import (
	"os"
	"testing"

	"l4d2-manager-next/consts"
)

func TestNormalizeDiskUsageLimitPercent(t *testing.T) {
	valid := []int{MinDiskUsageLimitPercent, 85, DefaultDiskUsageLimitPercent, MaxDiskUsageLimitPercent}
	for _, percent := range valid {
		got, err := NormalizeDiskUsageLimitPercent(percent)
		if err != nil {
			t.Fatalf("NormalizeDiskUsageLimitPercent(%d) error = %v", percent, err)
		}
		if got != percent {
			t.Fatalf("NormalizeDiskUsageLimitPercent(%d) = %d, want %d", percent, got, percent)
		}
	}

	invalid := []int{MinDiskUsageLimitPercent - 1, MaxDiskUsageLimitPercent + 1, 0, -1, 100}
	for _, percent := range invalid {
		if got, err := NormalizeDiskUsageLimitPercent(percent); err == nil {
			t.Fatalf("NormalizeDiskUsageLimitPercent(%d) = %d, want error", percent, got)
		}
	}
}

func TestDiskUsageLimitPercentDefaultsWhenConfigMissingOrLegacy(t *testing.T) {
	setupManagerConfigTest(t)

	if got := GetDiskUsageLimitPercent(); got != DefaultDiskUsageLimitPercent {
		t.Fatalf("default disk usage limit = %d, want %d", got, DefaultDiskUsageLimitPercent)
	}

	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatalf("create manager data path: %v", err)
	}
	legacyConfig := `{"enable_self_service":true,"enable_player_stats":false}`
	if err := os.WriteFile(consts.ManagerConfigPath, []byte(legacyConfig), 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	LoadManagerConfig()
	if got := GetDiskUsageLimitPercent(); got != DefaultDiskUsageLimitPercent {
		t.Fatalf("legacy config disk usage limit = %d, want %d", got, DefaultDiskUsageLimitPercent)
	}
}

func TestLoadManagerConfigFallsBackForOutOfRangeDiskUsageLimit(t *testing.T) {
	setupManagerConfigTest(t)

	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatalf("create manager data path: %v", err)
	}

	for _, config := range []string{
		`{"disk_usage_limit_percent":120}`,
		`{"disk_usage_limit_percent":0}`,
		`{"disk_usage_limit_percent":-30}`,
	} {
		if err := os.WriteFile(consts.ManagerConfigPath, []byte(config), 0644); err != nil {
			t.Fatalf("write config %s: %v", config, err)
		}

		LoadManagerConfig()
		if got := GetDiskUsageLimitPercent(); got != DefaultDiskUsageLimitPercent {
			t.Fatalf("config %s loaded limit = %d, want %d", config, got, DefaultDiskUsageLimitPercent)
		}
	}
}

func TestSetDiskUsageLimitPercentSavesAndLoads(t *testing.T) {
	setupManagerConfigTest(t)

	saved, err := SetDiskUsageLimitPercent(85)
	if err != nil {
		t.Fatalf("SetDiskUsageLimitPercent(85) error = %v", err)
	}
	if saved != 85 {
		t.Fatalf("saved limit = %d, want 85", saved)
	}

	if _, err := SetDiskUsageLimitPercent(79); err == nil {
		t.Fatal("SetDiskUsageLimitPercent(79) succeeded, want error")
	}
	if got := GetDiskUsageLimitPercent(); got != 85 {
		t.Fatalf("limit after rejected update = %d, want 85", got)
	}

	LoadManagerConfig()
	if got := GetDiskUsageLimitPercent(); got != 85 {
		t.Fatalf("loaded limit = %d, want 85", got)
	}
}

func TestEvaluateDiskSpace(t *testing.T) {
	const gib = 1 << 30

	cases := []struct {
		name         string
		usage        DiskUsage
		limitPercent int
		fileSize     int64
		adminForced  bool
		wantReason   DiskSpaceBlockReason
		wantRequired uint64
	}{
		{
			name:         "使用率低于上限放行",
			usage:        DiskUsage{UsedPercent: 50, Total: 500 * gib, Free: 100 * gib},
			limitPercent: 90,
			fileSize:     1 * gib,
			wantRequired: 2 * gib,
		},
		{
			name:         "使用率恰好等于上限放行",
			usage:        DiskUsage{UsedPercent: 90, Total: 500 * gib, Free: 100 * gib},
			limitPercent: 90,
			fileSize:     1 * gib,
			wantRequired: 2 * gib,
		},
		{
			name:         "超过上限未确认拦截",
			usage:        DiskUsage{UsedPercent: 90.1, Total: 500 * gib, Free: 100 * gib},
			limitPercent: 90,
			fileSize:     1 * gib,
			wantReason:   DiskBlockUsageExceeded,
			wantRequired: 2 * gib,
		},
		{
			name:         "超过上限且管理员已确认放行",
			usage:        DiskUsage{UsedPercent: 95, Total: 500 * gib, Free: 100 * gib},
			limitPercent: 90,
			fileSize:     1 * gib,
			adminForced:  true,
			wantRequired: 2 * gib,
		},
		{
			name:         "预判空间不足时管理员确认也拦截",
			usage:        DiskUsage{UsedPercent: 95, Total: 500 * gib, Free: 3 * gib},
			limitPercent: 90,
			fileSize:     2 * gib,
			adminForced:  true,
			wantReason:   DiskBlockFreeInsufficient,
			wantRequired: 4 * gib,
		},
		{
			name:         "空间不足优先于使用率未超上限",
			usage:        DiskUsage{UsedPercent: 10, Total: 500 * gib, Free: 1 * gib},
			limitPercent: 90,
			fileSize:     1 * gib,
			wantReason:   DiskBlockFreeInsufficient,
			wantRequired: 2 * gib,
		},
		{
			name:         "未知文件大小时跳过空间拦截",
			usage:        DiskUsage{UsedPercent: 95, Total: 500 * gib, Free: 0},
			limitPercent: 90,
			fileSize:     0,
			wantReason:   DiskBlockUsageExceeded,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateDiskSpace(tc.usage, tc.limitPercent, tc.fileSize, tc.adminForced)

			if got.BlockReason != tc.wantReason {
				t.Fatalf("BlockReason = %q, want %q", got.BlockReason, tc.wantReason)
			}
			if got.Blocked() != (tc.wantReason != "") {
				t.Fatalf("Blocked() = %v, want %v", got.Blocked(), tc.wantReason != "")
			}
			if got.RequiredBytes != tc.wantRequired {
				t.Fatalf("RequiredBytes = %d, want %d", got.RequiredBytes, tc.wantRequired)
			}
			if got.UsedPercent != tc.usage.UsedPercent || got.FreeBytes != tc.usage.Free || got.TotalBytes != tc.usage.Total {
				t.Fatalf("decision usage = %#v, want %#v", got, tc.usage)
			}
			if got.LimitPercent != tc.limitPercent {
				t.Fatalf("LimitPercent = %d, want %d", got.LimitPercent, tc.limitPercent)
			}
		})
	}
}

func TestEvaluateDiskSpaceUsesConfiguredLimit(t *testing.T) {
	setupManagerConfigTest(t)

	if _, err := SetDiskUsageLimitPercent(80); err != nil {
		t.Fatalf("SetDiskUsageLimitPercent(80) error = %v", err)
	}

	usage := DiskUsage{UsedPercent: 85, Total: 500 << 30, Free: 100 << 30}
	if got := EvaluateDiskSpace(usage, GetDiskUsageLimitPercent(), 0, false); got.BlockReason != DiskBlockUsageExceeded {
		t.Fatalf("BlockReason = %q, want %q", got.BlockReason, DiskBlockUsageExceeded)
	}
	if got := EvaluateDiskSpace(usage, GetDiskUsageLimitPercent(), 0, true); got.Blocked() {
		t.Fatalf("confirmed decision blocked = %q, want allowed", got.BlockReason)
	}
}
