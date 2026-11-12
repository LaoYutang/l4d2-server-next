package logic

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"l4d2-manager-next/consts"
)

func TestMapCleanupDefaultsAndLegacyConfig(t *testing.T) {
	setupManagerConfigTest(t)
	for _, data := range []string{"", `{"enable_player_stats":false}`, `{"map_cleanup":{"enabled":true}}`} {
		if data != "" {
			if err := consts.EnsureManagerDataPath(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(consts.ManagerConfigPath, []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			LoadManagerConfig()
		}
		cfg, err := GetMapCleanupConfig()
		if err != nil || cfg.RetentionDays != 3 || cfg.CheckTime != "03:00" || cfg.Enabled != strings.Contains(data, `"enabled":true`) {
			t.Fatalf("legacy %q: cfg=%+v err=%v", data, cfg, err)
		}
		if !cfg.NextCheckAt.After(time.Now()) || cfg.Timezone == "" {
			t.Fatalf("missing next schedule/timezone: %+v", cfg)
		}
	}
}

func TestMapCleanupSettingsPersistWithoutChangingOtherConfig(t *testing.T) {
	setupManagerConfigTest(t)
	if err := SetVPKTrimEnable(false); err != nil {
		t.Fatal(err)
	}
	if _, err := SetMapCleanupConfig(true, 7); err != nil {
		t.Fatal(err)
	}
	LoadManagerConfig()
	cfg, err := GetMapCleanupConfig()
	if err != nil || !cfg.Enabled || cfg.RetentionDays != 7 || IsVPKTrimEnabled() {
		t.Fatalf("reload cfg=%+v err=%v trim=%v", cfg, err, IsVPKTrimEnabled())
	}
	data, err := os.ReadFile(consts.ManagerConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		MapCleanup MapCleanupSettings `json:"map_cleanup"`
	}
	if err := json.Unmarshal(data, &stored); err != nil || stored.MapCleanup != cfg.MapCleanupSettings {
		t.Fatalf("stored config=%s err=%v", data, err)
	}
	if _, err := SetMapCleanupConfig(false, 7); err != nil {
		t.Fatal(err)
	}
	LoadManagerConfig()
	cfg, err = GetMapCleanupConfig()
	if err != nil || cfg.Enabled || cfg.RetentionDays != 7 {
		t.Fatalf("disable did not preserve days: %+v, %v", cfg, err)
	}
}

func TestMapCleanupRejectsInvalidDaysAndPreservesMemoryOnSaveFailure(t *testing.T) {
	setupManagerConfigTest(t)
	for _, days := range []int{-1, 0, MaxMapCleanupRetentionDays + 1} {
		if _, err := SetMapCleanupConfig(true, days); !errors.Is(err, ErrInvalidMapCleanupConfig) {
			t.Fatalf("days=%d error=%v", days, err)
		}
	}
	if _, err := SetMapCleanupConfig(true, 5); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(consts.ManagerConfigPath, consts.ManagerConfigPath+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(consts.ManagerConfigPath, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := SetMapCleanupConfig(false, 1); err == nil {
		t.Fatal("saving onto a directory unexpectedly succeeded")
	}
	cfg, err := GetMapCleanupConfig()
	if err != nil || !cfg.Enabled || cfg.RetentionDays != 5 {
		t.Fatalf("failed save published candidate: %+v, %v", cfg, err)
	}
	if leftovers, err := filepath.Glob(filepath.Join(consts.ManagerDataPath, ".manager-config-*.tmp")); err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary config files=%v err=%v", leftovers, err)
	}
}

func TestMapCleanupLoadErrorsFailClosedAndAreVisible(t *testing.T) {
	setupManagerConfigTest(t)
	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		`{"map_cleanup":{"enabled":true,`,
		`{"map_cleanup":{"enabled":true,"retention_days":"3"}}`,
		`{"map_cleanup":{"enabled":true,"retention_days":0}}`,
		`{"map_cleanup":{"enabled":true,"retention_days":null}}`,
		`{"map_cleanup":{"enabled":null,"retention_days":3}}`,
		`{"map_cleanup":{"enabled":"yes","retention_days":3}}`,
		`{"map_cleanup":null}`,
		`null`,
	} {
		if err := os.WriteFile(consts.ManagerConfigPath, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		LoadManagerConfig()
		if _, err := GetMapCleanupConfig(); err == nil {
			t.Fatalf("invalid config accepted: %s", data)
		}
		if managerConfig.MapCleanup.Enabled {
			t.Fatalf("partly decoded config enabled cleanup: %s", data)
		}
		if _, err := SetMapCleanupConfig(true, 3); err == nil {
			t.Fatalf("silently overwrote unreadable config: %s", data)
		}
		actual, err := os.ReadFile(consts.ManagerConfigPath)
		if err != nil || string(actual) != data {
			t.Fatalf("unreadable configuration changed: %s, %v", actual, err)
		}
	}
	if err := os.Remove(consts.ManagerConfigPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(consts.ManagerConfigPath, 0755); err != nil {
		t.Fatal(err)
	}
	LoadManagerConfig()
	if _, err := GetMapCleanupConfig(); err == nil {
		t.Fatal("config read failure not exposed")
	}
}

func TestNextMapCleanupIsStrictlyFutureLocalThreeAM(t *testing.T) {
	location := time.FixedZone("test-server", 8*60*60)
	for _, tc := range []struct {
		hour, minute, second int
		day                  int
	}{
		{2, 59, 59, 12},
		{3, 0, 0, 13},
		{17, 0, 0, 13},
	} {
		now := time.Date(2026, 11, 12, tc.hour, tc.minute, tc.second, 0, location)
		due := nextMapCleanupAt(now, location)
		want := time.Date(2026, 11, tc.day, 3, 0, 0, 0, location)
		if !due.Equal(want) || !due.After(now) {
			t.Fatalf("now=%v due=%v want=%v", now, due, want)
		}
		cfg := mapCleanupConfigAt(MapCleanupSettings{RetentionDays: 3}, now, location)
		if !cfg.NextCheckAt.Equal(want) || !strings.Contains(cfg.Timezone, "UTC+08:00") || !strings.Contains(cfg.Timezone, "test-server") {
			t.Fatalf("unexpected schedule display: %+v", cfg)
		}
	}
}

func TestNextMapCleanupUsesCalendarDaysAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone database unavailable: %v", err)
	}
	for _, tc := range []struct {
		month time.Month
		day   int
		hours int
	}{
		{time.March, 7, 14},
		{time.October, 31, 16},
	} {
		now := time.Date(2026, tc.month, tc.day, 12, 0, 0, 0, location)
		due := nextMapCleanupAt(now, location)
		if due.Hour() != 3 || due.Sub(now) != time.Duration(tc.hours)*time.Hour {
			t.Fatalf("DST now=%v due=%v elapsed=%v", now, due, due.Sub(now))
		}
	}
}

func TestMapCleanupWorkerWaitsUntilNextOccurrenceWithoutStartupCatchup(t *testing.T) {
	for _, hour := range []int{1, 4} {
		current := time.Date(2026, 11, 12, hour, 0, 0, 0, time.UTC)
		initial := current
		checks, waits := 0, 0
		wait := func(_ context.Context, due time.Time) bool {
			waits++
			if waits > 1 {
				return false
			}
			if checks != 0 || !due.Equal(nextMapCleanupAt(initial, time.UTC)) {
				t.Fatalf("startup ran cleanup or wrong due: checks=%d due=%v", checks, due)
			}
			current = due
			return true
		}
		runMapCleanup(context.Background(), func() time.Time { return current }, time.UTC, wait, func(now time.Time) {
			checks++
			if !now.Equal(current) {
				t.Fatalf("cleanup used stale timestamp: %v != %v", now, current)
			}
		})
		if checks != 1 || waits != 2 {
			t.Fatalf("checks=%d waits=%d", checks, waits)
		}
	}
}

func TestMapCleanupWorkerHandlesRollbackAndCancellation(t *testing.T) {
	current := time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)
	checks, waits := 0, 0
	due := nextMapCleanupAt(current, time.UTC)
	runMapCleanup(context.Background(), func() time.Time { return current }, time.UTC, func(_ context.Context, target time.Time) bool {
		waits++
		switch waits {
		case 1:
			current = due.Add(-time.Hour)
		case 2:
			if !target.Equal(due) {
				t.Fatalf("rollback changed pending schedule: %v != %v", target, due)
			}
			current = due
		default:
			return false
		}
		return true
	}, func(time.Time) { checks++ })
	if checks != 1 {
		t.Fatalf("rollback checks=%d", checks)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runMapCleanup(ctx, func() time.Time { return current }, time.UTC, func(context.Context, time.Time) bool {
		cancel()
		return true
	}, func(time.Time) { t.Fatal("cleanup ran after cancellation") })
	if waitForMapCleanup(ctx, time.Now().Add(time.Hour)) {
		t.Fatal("canceled wait did not stop")
	}
}
