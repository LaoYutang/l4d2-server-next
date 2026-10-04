package logic

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/db"
	"l4d2-manager-next/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	AuthAccessTemporary = "temporary"
	AuthAccessMapUpload = "map_upload_only"
)

var (
	ErrAuthUnavailable     = errors.New("授权服务暂不可用")
	ErrAuthInvalid         = errors.New("密码错误或授权码已失效")
	ErrAuthNotFound        = errors.New("授权记录不存在")
	ErrAuthDuplicate       = errors.New("该授权码已存在")
	ErrSelfServiceDisabled = errors.New("自助授权功能未开启")
	defaultAuthCodes       atomic.Pointer[AuthCodeStore]
)

type AuthValidationError struct{ Message string }

func (e *AuthValidationError) Error() string { return e.Message }

type AuthCooldownError struct{ RemainingSeconds int }

func (e *AuthCooldownError) Error() string {
	return fmt.Sprintf("系统冷却中，请等待 %d 秒", e.RemainingSeconds)
}

type AuthCodeItem struct {
	model.AuthCode
	Status string `json:"status"`
}

type CreatedAuthCode struct {
	AuthCodeItem
	Code string `json:"code"`
}

type CreateAuthCodeRequest struct {
	Code       string    `json:"code"`
	Remark     string    `json:"remark"`
	AccessType string    `json:"access_type"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type UpdateAuthCodeRequest struct {
	ID        string     `json:"id"`
	Remark    *string    `json:"remark"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type AuthCodeListFilter struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Status   string `json:"status"`
	Source   string `json:"source"`
	Keyword  string `json:"keyword"`
}

type AuthCodeListResult struct {
	Items        []AuthCodeItem `json:"items"`
	Total        int            `json:"total"`
	Page         int            `json:"page"`
	PageSize     int            `json:"page_size"`
	Counts       map[string]int `json:"counts"`
	CleanupCount int            `json:"cleanup_count"`
	ServerTime   time.Time      `json:"server_time"`
}

type SelfServiceStatus struct {
	Enabled           bool      `json:"enabled"`
	InCooldown        bool      `json:"in_cooldown"`
	RemainingSeconds  int       `json:"remaining_seconds"`
	LastGeneratedTime time.Time `json:"last_generated_time"`
}

type AuthCodeStore struct {
	database *gorm.DB
	key      []byte
	// Writers serialize commit and publication; authentication never waits on disk IO.
	writeMu             sync.Mutex
	mu                  sync.RWMutex
	byID                map[string]model.AuthCode
	byFingerprint       map[string]string
	lastSelfServiceTime time.Time
	closed              bool
	now                 func() time.Time
}

func GetAuthCodeStore() *AuthCodeStore                     { return defaultAuthCodes.Load() }
func SetAuthCodeStore(store *AuthCodeStore) *AuthCodeStore { return defaultAuthCodes.Swap(store) }

func InitAuthCodes() error {
	store, err := OpenAuthCodeStore(consts.AuthDBPath, consts.AuthKeyPath, GetSelfServiceConfig().LastSelfServiceTime)
	SetAuthCodeStore(store)
	return err
}

func OpenAuthCodeStore(databasePath, keyPath string, lastSelfServiceTime time.Time) (*AuthCodeStore, error) {
	key, err := loadAuthKey(databasePath, keyPath)
	if err != nil {
		return nil, err
	}
	database, err := db.OpenAuthDB(databasePath)
	if err != nil {
		return nil, err
	}
	store := &AuthCodeStore{database: database, key: key, byID: make(map[string]model.AuthCode), byFingerprint: make(map[string]string), now: time.Now}
	state := model.AuthCodeState{ID: 1, LastSelfServiceTime: lastSelfServiceTime.UTC()}
	err = database.Transaction(func(tx *gorm.DB) error {
		if err := tx.FirstOrCreate(&state, model.AuthCodeState{ID: 1}).Error; err != nil {
			return err
		}
		var records []model.AuthCode
		if err := tx.Find(&records).Error; err != nil {
			return err
		}
		for _, record := range records {
			store.byID[record.ID] = record
			store.byFingerprint[record.Fingerprint] = record.ID
		}
		return nil
	})
	if err != nil {
		store.Close()
		return nil, err
	}
	store.lastSelfServiceTime = state.LastSelfServiceTime
	return store, nil
}

func loadAuthKey(databasePath, keyPath string) ([]byte, error) {
	key, err := os.ReadFile(keyPath)
	if err == nil {
		if len(key) != 32 {
			return nil, errors.New("授权指纹密钥格式无效")
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if _, err := os.Stat(databasePath); err == nil {
		return nil, errors.New("授权数据库存在但 auth.key 缺失，请恢复对应密钥")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	if _, err = file.Write(key); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(keyPath)
		return nil, err
	}
	return key, nil
}

func (s *AuthCodeStore) Close() error {
	if s == nil {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	connection, err := s.database.DB()
	if err != nil {
		return err
	}
	return connection.Close()
}

func AdministratorPassword() string {
	if password := os.Getenv("L4D2_MANAGER_PASSWORD"); password != "" {
		return password
	}
	return "laoyutangnb"
}

func ValidateAuthCode(code string) error {
	if len(code) < 8 || len(code) > 32 {
		return &AuthValidationError{"自定义授权码必须为 8～32 位"}
	}
	for _, char := range code {
		if char < '!' || char > '~' {
			return &AuthValidationError{"授权码只能包含 ASCII 可见字符，不能包含空白"}
		}
	}
	if code == AdministratorPassword() {
		return &AuthValidationError{"授权码不能与管理员密码相同"}
	}
	return nil
}

func normalizeAuthRemark(remark string) (string, error) {
	remark = strings.TrimSpace(remark)
	if utf8.RuneCountInString(remark) > 200 || strings.ContainsRune(remark, '\x00') {
		return "", &AuthValidationError{"备注不能超过 200 个字符或包含空字符"}
	}
	return remark, nil
}

func randomAuthCode() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	code := make([]byte, 32)
	for i := range code {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		code[i] = alphabet[value.Int64()]
	}
	return string(code), nil
}

func (s *AuthCodeStore) fingerprint(code string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *AuthCodeStore) ready() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.closed
}

func authCodeItem(record model.AuthCode, now time.Time) AuthCodeItem {
	status := "active"
	if record.RevokedAt != nil {
		status = "revoked"
	} else if !record.ExpiresAt.After(now) {
		status = "expired"
	}
	return AuthCodeItem{AuthCode: record, Status: status}
}

func (s *AuthCodeStore) Authenticate(code string) (model.AuthCode, error) {
	if len(code) < 8 || len(code) > 32 {
		return model.AuthCode{}, ErrAuthInvalid
	}
	if !s.ready() {
		return model.AuthCode{}, ErrAuthUnavailable
	}
	s.mu.RLock()
	id := s.byFingerprint[s.fingerprint(code)]
	record, ok := s.byID[id]
	s.mu.RUnlock()
	if !ok || record.RevokedAt != nil || !record.ExpiresAt.After(s.now()) {
		return model.AuthCode{}, ErrAuthInvalid
	}
	return record, nil
}

func (s *AuthCodeStore) ActiveByID(id string) (model.AuthCode, error) {
	if !s.ready() {
		return model.AuthCode{}, ErrAuthUnavailable
	}
	s.mu.RLock()
	record, ok := s.byID[id]
	s.mu.RUnlock()
	if !ok || record.RevokedAt != nil || !record.ExpiresAt.After(s.now()) {
		return model.AuthCode{}, ErrAuthInvalid
	}
	return record, nil
}

func (s *AuthCodeStore) recordByID(id string) (model.AuthCode, error) {
	if !s.ready() {
		return model.AuthCode{}, ErrAuthUnavailable
	}
	s.mu.RLock()
	record, ok := s.byID[id]
	s.mu.RUnlock()
	if !ok {
		return model.AuthCode{}, ErrAuthNotFound
	}
	return record, nil
}

func (s *AuthCodeStore) publish(record model.AuthCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[record.ID] = record
	s.byFingerprint[record.Fingerprint] = record.ID
}

func (s *AuthCodeStore) newRecord(req CreateAuthCodeRequest, source string) (model.AuthCode, string, error) {
	now := s.now().UTC()
	if req.AccessType == "" {
		req.AccessType = AuthAccessTemporary
	}
	if req.AccessType != AuthAccessTemporary && req.AccessType != AuthAccessMapUpload {
		return model.AuthCode{}, "", &AuthValidationError{"无效的授权类型"}
	}
	remark, err := normalizeAuthRemark(req.Remark)
	if err != nil {
		return model.AuthCode{}, "", err
	}
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = now.Add(time.Hour)
	}
	req.ExpiresAt = req.ExpiresAt.UTC()
	if !req.ExpiresAt.After(now) || req.ExpiresAt.Year() < 1 || req.ExpiresAt.Year() > 9999 {
		return model.AuthCode{}, "", &AuthValidationError{"创建授权时到期时间必须晚于服务器当前时间"}
	}
	code := req.Code
	for attempt := 0; attempt < 5; attempt++ {
		if req.Code == "" {
			code, err = randomAuthCode()
			if err != nil {
				return model.AuthCode{}, "", err
			}
		}
		if err := ValidateAuthCode(code); err != nil {
			return model.AuthCode{}, "", err
		}
		fingerprint := s.fingerprint(code)
		s.mu.RLock()
		_, exists := s.byFingerprint[fingerprint]
		s.mu.RUnlock()
		if exists {
			if req.Code != "" {
				return model.AuthCode{}, "", ErrAuthDuplicate
			}
			continue
		}
		return model.AuthCode{ID: uuid.NewString(), Fingerprint: fingerprint, MaskedCode: code[:2] + "••••" + code[len(code)-2:], Remark: remark, AccessType: req.AccessType, Source: source, CreatedAt: now, ExpiresAt: req.ExpiresAt.UTC()}, code, nil
	}
	return model.AuthCode{}, "", ErrAuthDuplicate
}

func (s *AuthCodeStore) Create(req CreateAuthCodeRequest) (CreatedAuthCode, error) {
	if !s.ready() {
		return CreatedAuthCode{}, ErrAuthUnavailable
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if !s.ready() {
		return CreatedAuthCode{}, ErrAuthUnavailable
	}
	record, code, err := s.newRecord(req, "manual")
	if err != nil {
		return CreatedAuthCode{}, err
	}
	if err := s.database.Create(&record).Error; err != nil {
		return CreatedAuthCode{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	s.publish(record)
	return CreatedAuthCode{AuthCodeItem: authCodeItem(record, s.now()), Code: code}, nil
}

func (s *AuthCodeStore) Update(req UpdateAuthCodeRequest) (AuthCodeItem, error) {
	if !s.ready() {
		return AuthCodeItem{}, ErrAuthUnavailable
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	record, err := s.recordByID(req.ID)
	if err != nil {
		return AuthCodeItem{}, err
	}
	updates := make(map[string]any)
	if req.Remark != nil {
		record.Remark, err = normalizeAuthRemark(*req.Remark)
		if err != nil {
			return AuthCodeItem{}, err
		}
		updates["remark"] = record.Remark
	}
	if req.ExpiresAt != nil {
		expiresAt := req.ExpiresAt.UTC()
		if expiresAt.IsZero() || expiresAt.Year() < 1 || expiresAt.Year() > 9999 {
			return AuthCodeItem{}, &AuthValidationError{"到期时间必须为具体日期"}
		}
		record.ExpiresAt = expiresAt
		updates["expires_at"] = record.ExpiresAt
	}
	if len(updates) == 0 {
		return AuthCodeItem{}, &AuthValidationError{"请提供备注或到期时间"}
	}
	if err := s.database.Model(&model.AuthCode{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
		return AuthCodeItem{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	s.publish(record)
	return authCodeItem(record, s.now()), nil
}

func (s *AuthCodeStore) Revoke(id string) (AuthCodeItem, error) {
	if !s.ready() {
		return AuthCodeItem{}, ErrAuthUnavailable
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	record, err := s.recordByID(id)
	if err != nil {
		return AuthCodeItem{}, err
	}
	if record.RevokedAt == nil {
		now := s.now().UTC()
		record.RevokedAt = &now
		if err := s.database.Model(&model.AuthCode{}).Where("id = ?", id).Update("revoked_at", now).Error; err != nil {
			return AuthCodeItem{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
		}
		s.publish(record)
	}
	return authCodeItem(record, s.now()), nil
}

func (s *AuthCodeStore) remove(records []model.AuthCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range records {
		delete(s.byID, record.ID)
		delete(s.byFingerprint, record.Fingerprint)
	}
}

func (s *AuthCodeStore) Delete(id string) (AuthCodeItem, error) {
	if !s.ready() {
		return AuthCodeItem{}, ErrAuthUnavailable
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	record, err := s.recordByID(id)
	if err != nil {
		return AuthCodeItem{}, err
	}
	if err := s.database.Where("id = ?", id).Delete(&model.AuthCode{}).Error; err != nil {
		return AuthCodeItem{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	s.remove([]model.AuthCode{record})
	return authCodeItem(record, s.now()), nil
}

func (s *AuthCodeStore) CleanupExpired() (int, error) {
	if !s.ready() {
		return 0, ErrAuthUnavailable
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if !s.ready() {
		return 0, ErrAuthUnavailable
	}
	var records []model.AuthCode
	now := s.now().UTC()
	err := s.database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", now).Find(&records).Error; err != nil {
			return err
		}
		return tx.Where("expires_at <= ?", now).Delete(&model.AuthCode{}).Error
	})
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	s.remove(records)
	return len(records), nil
}

func (s *AuthCodeStore) RecordLogin(id, ip string) (AuthCodeItem, error) {
	if !s.ready() {
		return AuthCodeItem{}, ErrAuthUnavailable
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	record, err := s.ActiveByID(id)
	if err != nil {
		return AuthCodeItem{}, err
	}
	now := s.now().UTC()
	record.LastLoginAt = &now
	record.LastLoginIP = ip
	record.LoginCount++
	if record.FirstLoginAt == nil {
		record.FirstLoginAt = &now
	}
	updates := map[string]any{"last_login_at": now, "first_login_at": record.FirstLoginAt, "last_login_ip": ip, "login_count": record.LoginCount}
	if err := s.database.Model(&model.AuthCode{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return AuthCodeItem{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	s.publish(record)
	return authCodeItem(record, now), nil
}

func (s *AuthCodeStore) List(filter AuthCodeListFilter) (AuthCodeListResult, error) {
	if !s.ready() {
		return AuthCodeListResult{}, ErrAuthUnavailable
	}
	if filter.Status != "" && filter.Status != "active" && filter.Status != "expired" && filter.Status != "revoked" {
		return AuthCodeListResult{}, &AuthValidationError{"无效的授权状态"}
	}
	if filter.Source != "" && filter.Source != "manual" && filter.Source != "self_service" {
		return AuthCodeListResult{}, &AuthValidationError{"无效的授权来源"}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	now := s.now().UTC()
	result := AuthCodeListResult{Items: []AuthCodeItem{}, Page: filter.Page, PageSize: filter.PageSize, Counts: map[string]int{"all": 0, "active": 0, "expired": 0, "revoked": 0}, ServerTime: now}
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	s.mu.RLock()
	for _, record := range s.byID {
		item := authCodeItem(record, now)
		result.Counts["all"]++
		result.Counts[item.Status]++
		if !record.ExpiresAt.After(now) {
			result.CleanupCount++
		}
		if filter.Status != "" && item.Status != filter.Status || filter.Source != "" && record.Source != filter.Source || keyword != "" && !strings.Contains(strings.ToLower(record.Remark+" "+record.ID), keyword) {
			continue
		}
		result.Items = append(result.Items, item)
	}
	s.mu.RUnlock()
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].CreatedAt.Equal(result.Items[j].CreatedAt) {
			return result.Items[i].ID > result.Items[j].ID
		}
		return result.Items[i].CreatedAt.After(result.Items[j].CreatedAt)
	})
	result.Total = len(result.Items)
	// Reject out-of-range pages before multiplication can overflow.
	if filter.Page-1 > result.Total/filter.PageSize {
		result.Items = []AuthCodeItem{}
		return result, nil
	}
	start := (filter.Page - 1) * filter.PageSize
	if start < 0 || start >= result.Total {
		result.Items = []AuthCodeItem{}
	} else {
		end := start + filter.PageSize
		if end > result.Total {
			end = result.Total
		}
		result.Items = result.Items[start:end]
	}
	return result, nil
}

func (s *AuthCodeStore) SelfServiceStatus() (SelfServiceStatus, error) {
	if !s.ready() {
		return SelfServiceStatus{}, ErrAuthUnavailable
	}
	s.mu.RLock()
	last := s.lastSelfServiceTime
	s.mu.RUnlock()
	status := SelfServiceStatus{Enabled: GetSelfServiceConfig().EnableSelfService, LastGeneratedTime: last}
	remaining := time.Hour - s.now().Sub(last)
	if status.Enabled && remaining > 0 {
		status.InCooldown = true
		status.RemainingSeconds = int((remaining + time.Second - 1) / time.Second)
	}
	return status, nil
}

func (s *AuthCodeStore) GenerateSelfService() (CreatedAuthCode, error) {
	if !s.ready() {
		return CreatedAuthCode{}, ErrAuthUnavailable
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if !s.ready() {
		return CreatedAuthCode{}, ErrAuthUnavailable
	}
	if !GetSelfServiceConfig().EnableSelfService {
		return CreatedAuthCode{}, ErrSelfServiceDisabled
	}
	record, code, err := s.newRecord(CreateAuthCodeRequest{Remark: "自助领取"}, "self_service")
	if err != nil {
		return CreatedAuthCode{}, err
	}
	err = s.database.Transaction(func(tx *gorm.DB) error {
		var state model.AuthCodeState
		if err := tx.First(&state, 1).Error; err != nil {
			return err
		}
		remaining := time.Hour - record.CreatedAt.Sub(state.LastSelfServiceTime)
		if remaining > 0 {
			return &AuthCooldownError{int((remaining + time.Second - 1) / time.Second)}
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		return tx.Model(&state).Update("last_self_service_time", record.CreatedAt).Error
	})
	if err != nil {
		var cooldown *AuthCooldownError
		if errors.As(err, &cooldown) {
			return CreatedAuthCode{}, err
		}
		return CreatedAuthCode{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	s.mu.Lock()
	s.byID[record.ID] = record
	s.byFingerprint[record.Fingerprint] = record.ID
	s.lastSelfServiceTime = record.CreatedAt
	s.mu.Unlock()
	return CreatedAuthCode{AuthCodeItem: authCodeItem(record, s.now()), Code: code}, nil
}
