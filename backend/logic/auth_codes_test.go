package logic

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testAuthCodeStore(t *testing.T) *AuthCodeStore {
	t.Helper()
	t.Setenv("L4D2_MANAGER_PASSWORD", "test-admin-password")
	dir := t.TempDir()
	store, err := OpenAuthCodeStore(filepath.Join(dir, "auth.db"), filepath.Join(dir, "auth.key"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestAuthCodeCreationValidationAndSecretStorage(t *testing.T) {
	store := testAuthCodeStore(t)
	for _, code := range []string{"abcdefgh", strings.Repeat("A", 32), "", ""} {
		created, err := store.Create(CreateAuthCodeRequest{Code: code, Remark: "地图测试", ExpiresAt: time.Now().Add(90 * 24 * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		if code == "" && len(created.Code) != 32 {
			t.Fatalf("random length = %d", len(created.Code))
		}
		if created.Code != code && code != "" {
			t.Fatal("custom credential changed")
		}
		if created.Fingerprint == created.Code || strings.Contains(created.MaskedCode, created.Code) {
			t.Fatal("credential leaked")
		}
		if _, err := store.Authenticate(created.Code); err != nil {
			t.Fatal(err)
		}
		list, err := store.List(AuthCodeListFilter{Keyword: "地图"})
		if err != nil {
			t.Fatal(err)
		}
		serialized, err := json.Marshal(list)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(serialized), created.Code) || strings.Contains(string(serialized), created.Fingerprint) {
			t.Fatal("list exposes credential or fingerprint")
		}
	}
	for _, code := range []string{"1234567", strings.Repeat("a", 33), "abc defg", "中文授权码abcdefgh", "abcd\nefgh", "test-admin-password"} {
		if _, err := store.Create(CreateAuthCodeRequest{Code: code}); err == nil {
			t.Fatalf("accepted invalid credential %q", code)
		}
	}
	if _, err := store.Create(CreateAuthCodeRequest{Code: "abcdefgh"}); !errors.Is(err, ErrAuthDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := store.Create(CreateAuthCodeRequest{AccessType: "admin"}); err == nil {
		t.Fatal("accepted admin access")
	}
	if _, err := store.Create(CreateAuthCodeRequest{ExpiresAt: time.Now().Add(-time.Hour)}); err == nil {
		t.Fatal("accepted past creation expiry")
	}
}

func TestAuthCodeLifecycleAndPersistence(t *testing.T) {
	store := testAuthCodeStore(t)
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	created, err := store.Create(CreateAuthCodeRequest{Code: "caseSensitiveCode", AccessType: AuthAccessMapUpload})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(strings.ToLower(created.Code)); !errors.Is(err, ErrAuthInvalid) {
		t.Fatal("credential is not case sensitive")
	}
	login, err := store.RecordLogin(created.ID, "192.0.2.1")
	if err != nil || login.LoginCount != 1 || login.FirstLoginAt == nil || login.LastLoginIP != "192.0.2.1" {
		t.Fatalf("login: %+v %v", login, err)
	}
	past := now.Add(-time.Second)
	if _, err := store.Update(UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &past}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(created.Code); !errors.Is(err, ErrAuthInvalid) {
		t.Fatal("expired credential accepted")
	}
	future := now.Add(180 * 24 * time.Hour)
	remark := "延期地图上传"
	updated, err := store.Update(UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &future, Remark: &remark})
	if err != nil || updated.Status != "active" {
		t.Fatalf("extension: %+v %v", updated, err)
	}
	if _, err := store.Authenticate(created.Code); err != nil {
		t.Fatal("extension did not restore original credential")
	}
	if _, err := store.Revoke(created.ID); err != nil {
		t.Fatal(err)
	}
	updated, err = store.Update(UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &future})
	if err != nil || updated.Status != "revoked" {
		t.Fatalf("revocation lost: %+v %v", updated, err)
	}
	if _, err := store.Authenticate(created.Code); !errors.Is(err, ErrAuthInvalid) {
		t.Fatal("revoked credential accepted")
	}
	var databases []struct {
		Name string
		File string
	}
	if err := store.database.Raw("PRAGMA database_list").Scan(&databases).Error; err != nil {
		t.Fatal(err)
	}
	if len(databases) == 0 {
		t.Fatal("missing database path")
	}
	databasePath := databases[0].File
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAuthCodeStore(databasePath, filepath.Join(filepath.Dir(databasePath), "auth.key"), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Authenticate(created.Code); !errors.Is(err, ErrAuthInvalid) {
		t.Fatal("revocation lost after restart")
	}
	list, err := reopened.List(AuthCodeListFilter{})
	if err != nil || len(list.Items) != 1 || list.Items[0].Remark != remark || list.Items[0].LoginCount != 1 {
		t.Fatalf("persistent record: %+v %v", list, err)
	}
	if _, err := reopened.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Authenticate(created.Code); !errors.Is(err, ErrAuthInvalid) {
		t.Fatal("deleted credential accepted")
	}
	recreated, err := reopened.Create(CreateAuthCodeRequest{Code: created.Code})
	if err != nil || recreated.ID == created.ID {
		t.Fatalf("recreation did not obtain new id: %v", err)
	}
}

func TestAuthCodeCleanupPreservesExtendedRecordsAndRemovesRevokedExpired(t *testing.T) {
	store := testAuthCodeStore(t)
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	first, err := store.Create(CreateAuthCodeRequest{Code: "expired1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(CreateAuthCodeRequest{Code: "revoked1"})
	if err != nil {
		t.Fatal(err)
	}
	third, err := store.Create(CreateAuthCodeRequest{Code: "extended"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Revoke(second.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	future := now.Add(time.Hour)
	if _, err := store.Update(UpdateAuthCodeRequest{ID: third.ID, ExpiresAt: &future}); err != nil {
		t.Fatal(err)
	}
	count, err := store.CleanupExpired()
	if err != nil || count != 2 {
		t.Fatalf("cleanup count = %d, err = %v", count, err)
	}
	if _, err := store.recordByID(first.ID); !errors.Is(err, ErrAuthNotFound) {
		t.Fatal("expired record retained")
	}
	if _, err := store.recordByID(second.ID); !errors.Is(err, ErrAuthNotFound) {
		t.Fatal("revoked expired record retained")
	}
	if _, err := store.Authenticate(third.Code); err != nil {
		t.Fatal("extended record removed")
	}
}

func TestAuthCodeConcurrentCreationAndSelfServiceCooldown(t *testing.T) {
	store := testAuthCodeStore(t)
	setupManagerConfigTest(t)
	if err := SetSelfServiceEnable(true); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 12)
	for range 12 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := store.Create(CreateAuthCodeRequest{Code: "sharedCode"})
			results <- err
		}()
	}
	group.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrAuthDuplicate) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent creations = %d", successes)
	}
	results = make(chan error, 12)
	for range 12 {
		group.Add(1)
		go func() { defer group.Done(); _, err := store.GenerateSelfService(); results <- err }()
	}
	group.Wait()
	close(results)
	successes = 0
	for err := range results {
		var cooldown *AuthCooldownError
		if err == nil {
			successes++
		} else if !errors.As(err, &cooldown) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent self-service grants = %d", successes)
	}
	status, err := store.SelfServiceStatus()
	if err != nil || !status.InCooldown || status.RemainingSeconds < 3590 {
		t.Fatalf("status: %+v %v", status, err)
	}
	list, err := store.List(AuthCodeListFilter{Source: "self_service"})
	if err != nil || list.Total != 1 || len(list.Items[0].MaskedCode) == 0 {
		t.Fatalf("self service listing: %+v %v", list, err)
	}
}

func TestAuthCodeFailedWriteDoesNotChangeRuntimeState(t *testing.T) {
	store := testAuthCodeStore(t)
	created, err := store.Create(CreateAuthCodeRequest{Code: "keepValid"})
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := store.database.DB()
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if _, err := store.Update(UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &past}); !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("write error: %v", err)
	}
	if _, err := store.Authenticate(created.Code); err != nil {
		t.Fatal("failed write changed cached authorization")
	}
}

func TestAuthCodeMissingKeyIsNotRegenerated(t *testing.T) {
	dir := t.TempDir()
	databasePath, keyPath := filepath.Join(dir, "auth.db"), filepath.Join(dir, "auth.key")
	store, err := OpenAuthCodeStore(databasePath, keyPath, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenAuthCodeStore(databasePath, keyPath, time.Time{}); err == nil {
		t.Fatal("accepted missing key")
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatal("regenerated missing key")
	}
}

func TestAuthCodeSelfServiceCooldownPersistsAfterDeletionAndRestart(t *testing.T) {
	setupManagerConfigTest(t)
	if err := SetSelfServiceEnable(true); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	databasePath, keyPath := filepath.Join(dir, "auth.db"), filepath.Join(dir, "auth.key")
	// The old manager timestamp is imported only on first creation of the state.
	last := time.Now().UTC().Add(-30 * time.Minute)
	store, err := OpenAuthCodeStore(databasePath, keyPath, last)
	if err != nil {
		t.Fatal(err)
	}
	status, err := store.SelfServiceStatus()
	if err != nil || !status.InCooldown || status.RemainingSeconds < 1790 {
		t.Fatalf("initial cooldown: %+v %v", status, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenAuthCodeStore(databasePath, keyPath, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(31 * time.Minute)
	store.now = func() time.Time { return now }
	issued, err := store.GenerateSelfService()
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if issued.Source != "self_service" || issued.AccessType != AuthAccessTemporary || len(issued.Code) != 32 || issued.ExpiresAt.Sub(issued.CreatedAt) != time.Hour {
		t.Fatal("incorrect self-service grant")
	}
	if _, err := store.Delete(issued.ID); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenAuthCodeStore(databasePath, keyPath, last)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.now = func() time.Time { return now }
	var cooldown *AuthCooldownError
	if _, err := store.GenerateSelfService(); !errors.As(err, &cooldown) {
		t.Fatalf("restart/deletion reset cooldown: %v", err)
	}
}

func TestAuthCodeConcurrentCleanupNeverRemovesCommittedExtension(t *testing.T) {
	store := testAuthCodeStore(t)
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	for range 20 {
		created, err := store.Create(CreateAuthCodeRequest{})
		if err != nil {
			t.Fatal(err)
		}
		now = now.Add(2 * time.Hour)
		future := now.Add(time.Hour)
		var updateErr, cleanupErr error
		var group sync.WaitGroup
		group.Add(2)
		go func() {
			defer group.Done()
			_, updateErr = store.Update(UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &future})
		}()
		go func() { defer group.Done(); _, cleanupErr = store.CleanupExpired() }()
		group.Wait()
		if cleanupErr != nil {
			t.Fatal(cleanupErr)
		}
		if updateErr == nil {
			if _, err := store.Authenticate(created.Code); err != nil {
				t.Fatal("cleanup removed successfully extended authorization")
			}
		} else if !errors.Is(updateErr, ErrAuthNotFound) {
			t.Fatal(updateErr)
		}
	}
}

func TestAuthCodeExpiryRejectsUTCDateOverflow(t *testing.T) {
	store := testAuthCodeStore(t)
	created, err := store.Create(CreateAuthCodeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"9999-12-31T23:59:59-05:00", "0001-01-01T00:00:00+08:00"} {
		expiresAt, err := time.Parse(time.RFC3339, text)
		if err != nil {
			t.Fatal(err)
		}
		var invalid *AuthValidationError
		if _, err := store.Create(CreateAuthCodeRequest{ExpiresAt: expiresAt}); !errors.As(err, &invalid) {
			t.Fatalf("create accepted unrepresentable UTC expiry: %v", err)
		}
		if _, err := store.Update(UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &expiresAt}); !errors.As(err, &invalid) {
			t.Fatalf("update accepted unrepresentable UTC expiry: %v", err)
		}
	}
	if _, err := store.Authenticate(created.Code); err != nil {
		t.Fatal("invalid expiry modified original authorization")
	}
}
