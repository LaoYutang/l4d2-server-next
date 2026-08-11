package logic

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"l4d2-manager-next/consts"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

const (
	AccessControlConfigVersion = 1
	maxAccessRules             = 256
	maxTrustedProxies          = 128
	maxAccessRuleValueLength   = 128
	maxAccessRuleRemarkLength  = 200
)

const (
	AccessRuleTypeKeyword = "keyword"
	AccessRuleTypeIP      = "ip"
	AccessRuleTypeCIDR    = "cidr"
)

var (
	ErrAccessControlRevisionConflict = errors.New("access control revision conflict")
	ErrAccessControlWouldLockOut     = errors.New("access control update would lock out current administrator")
	ErrAccessControlRecoveryMode     = errors.New("access control is in recovery mode")
	ErrAccessControlGeoIPUnavailable = errors.New("GeoIP lookup is required but unavailable")
	ErrAccessControlPersist          = errors.New("failed to persist access control configuration")
	ErrInvalidClientIP               = errors.New("invalid client IP")
)

type AccessRule struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Remark  string `json:"remark,omitempty"`
}

type AccessControlConfig struct {
	Version        int          `json:"version"`
	Revision       uint64       `json:"revision"`
	Enabled        bool         `json:"enabled"`
	TrustedProxies []string     `json:"trusted_proxies"`
	PanelBlacklist []AccessRule `json:"panel_blacklist"`
	PanelWhitelist []AccessRule `json:"panel_whitelist"`
}

type PanelRulesUpdate struct {
	Enabled        bool         `json:"enabled"`
	PanelBlacklist []AccessRule `json:"panel_blacklist"`
	PanelWhitelist []AccessRule `json:"panel_whitelist"`
}

type ClientIPInput struct {
	RemoteAddr    string
	XForwardedFor string
	XRealIP       string
}

type ClientIPInfo struct {
	PeerIP              string `json:"peer_ip"`
	ClientIP            string `json:"client_ip"`
	Source              string `json:"source"`
	XForwardedFor       string `json:"x_forwarded_for,omitempty"`
	XRealIP             string `json:"x_real_ip,omitempty"`
	PeerTrusted         bool   `json:"peer_trusted"`
	MatchedTrustedProxy string `json:"matched_trusted_proxy,omitempty"`
}

type AccessDecision struct {
	Allowed     bool        `json:"allowed"`
	Reason      string      `json:"reason"`
	Region      string      `json:"region,omitempty"`
	MatchedRule *AccessRule `json:"matched_rule,omitempty"`
}

type AccessControlState struct {
	Config       AccessControlConfig `json:"config"`
	RecoveryMode bool                `json:"recovery_mode"`
	LoadError    string              `json:"load_error,omitempty"`
}

type AccessControlPreviewResult struct {
	Config            AccessControlConfig `json:"config"`
	CurrentConnection ClientIPInfo        `json:"current_connection"`
	CurrentDecision   AccessDecision      `json:"current_decision"`
	TestDecision      *AccessDecision     `json:"test_decision,omitempty"`
}

type RegionLookup func(string) (string, error)

type compiledAccessRule struct {
	rule    AccessRule
	address netip.Addr
	prefix  netip.Prefix
}

type AccessControlSnapshot struct {
	config           AccessControlConfig
	trustedProxies   []netip.Prefix
	panelBlacklist   []compiledAccessRule
	panelWhitelist   []compiledAccessRule
	recoveryMode     bool
	configurationErr string
}

var (
	accessControlMutex    sync.Mutex
	accessControlConfig   AccessControlConfig
	accessControlRecovery bool
	accessControlLoadErr  string
	accessControlSnapshot atomic.Pointer[AccessControlSnapshot]
)

func defaultAccessControlConfig() AccessControlConfig {
	return AccessControlConfig{
		Version:        AccessControlConfigVersion,
		Revision:       1,
		Enabled:        false,
		TrustedProxies: []string{"127.0.0.0/8", "::1/128"},
		PanelBlacklist: []AccessRule{},
		PanelWhitelist: []AccessRule{},
	}
}

// InitAccessControl loads the persisted policy and publishes an immutable runtime snapshot.
// A malformed existing file activates loopback-only recovery mode instead of failing open.
func InitAccessControl() {
	accessControlMutex.Lock()
	defer accessControlMutex.Unlock()

	config := defaultAccessControlConfig()
	recoveryMode := false
	loadError := ""

	data, err := os.ReadFile(consts.AccessControlConfigPath)
	if err == nil {
		if err = json.Unmarshal(data, &config); err != nil {
			recoveryMode = true
			loadError = fmt.Sprintf("解析访问控制配置失败: %v", err)
			config = defaultAccessControlConfig()
		} else if config.Version != AccessControlConfigVersion {
			recoveryMode = true
			loadError = fmt.Sprintf("不支持的访问控制配置版本: %d", config.Version)
			config = defaultAccessControlConfig()
		}
	} else if !os.IsNotExist(err) {
		recoveryMode = true
		loadError = fmt.Sprintf("读取访问控制配置失败: %v", err)
	}

	canonical, snapshot, compileErr := compileAccessControlConfig(config)
	if compileErr != nil {
		recoveryMode = true
		loadError = fmt.Sprintf("访问控制配置无效: %v", compileErr)
		canonical, snapshot, _ = compileAccessControlConfig(defaultAccessControlConfig())
	}
	if recoveryMode {
		snapshot.recoveryMode = true
		snapshot.configurationErr = loadError
	}

	accessControlConfig = canonical
	accessControlRecovery = recoveryMode
	accessControlLoadErr = loadError
	accessControlSnapshot.Store(snapshot)
}

func GetAccessControlState() AccessControlState {
	accessControlMutex.Lock()
	defer accessControlMutex.Unlock()
	return AccessControlState{
		Config:       cloneAccessControlConfig(accessControlConfig),
		RecoveryMode: accessControlRecovery,
		LoadError:    accessControlLoadErr,
	}
}

func CurrentAccessControlSnapshot() *AccessControlSnapshot {
	if snapshot := accessControlSnapshot.Load(); snapshot != nil {
		return snapshot
	}
	InitAccessControl()
	return accessControlSnapshot.Load()
}

func (snapshot *AccessControlSnapshot) IsRecoveryMode() bool {
	return snapshot != nil && snapshot.recoveryMode
}

func (snapshot *AccessControlSnapshot) ConfigurationError() string {
	if snapshot == nil {
		return ""
	}
	return snapshot.configurationErr
}

func (snapshot *AccessControlSnapshot) Config() AccessControlConfig {
	if snapshot == nil {
		return defaultAccessControlConfig()
	}
	return cloneAccessControlConfig(snapshot.config)
}

func (snapshot *AccessControlSnapshot) ResolveClientIP(input ClientIPInput) (ClientIPInfo, error) {
	peer, err := parseRemoteAddress(input.RemoteAddr)
	if err != nil {
		return ClientIPInfo{}, fmt.Errorf("%w: %v", ErrInvalidClientIP, err)
	}

	info := ClientIPInfo{
		PeerIP:        peer.String(),
		ClientIP:      peer.String(),
		Source:        "remote_addr",
		XForwardedFor: strings.TrimSpace(input.XForwardedFor),
		XRealIP:       strings.TrimSpace(input.XRealIP),
	}
	if snapshot == nil || snapshot.recoveryMode {
		return info, nil
	}

	trusted, matchedPrefix := snapshot.isTrustedProxy(peer)
	info.PeerTrusted = trusted
	info.MatchedTrustedProxy = matchedPrefix
	if !trusted {
		return info, nil
	}

	if info.XForwardedFor != "" {
		client, err := snapshot.parseForwardedFor(info.XForwardedFor)
		if err != nil {
			return ClientIPInfo{}, err
		}
		info.ClientIP = client.String()
		info.Source = "x_forwarded_for"
		return info, nil
	}

	if info.XRealIP != "" {
		client, err := normalizeIP(info.XRealIP)
		if err != nil {
			return ClientIPInfo{}, fmt.Errorf("X-Real-IP 格式无效: %w", err)
		}
		info.ClientIP = client.String()
		info.Source = "x_real_ip"
	}

	return info, nil
}

func (snapshot *AccessControlSnapshot) Evaluate(clientIP string, lookup RegionLookup) (AccessDecision, error) {
	address, err := normalizeIP(clientIP)
	if err != nil {
		return AccessDecision{Allowed: false, Reason: "invalid_client_ip"}, fmt.Errorf("%w: %v", ErrInvalidClientIP, err)
	}

	if snapshot == nil {
		return AccessDecision{Allowed: true, Reason: "disabled"}, nil
	}
	if snapshot.recoveryMode {
		if address.IsLoopback() {
			return AccessDecision{Allowed: true, Reason: "loopback"}, nil
		}
		return AccessDecision{Allowed: false, Reason: "recovery_mode"}, ErrAccessControlRecoveryMode
	}
	if address.IsLoopback() {
		return AccessDecision{Allowed: true, Reason: "loopback"}, nil
	}
	if !snapshot.config.Enabled {
		return AccessDecision{Allowed: true, Reason: "disabled"}, nil
	}

	if rule := matchAddressRules(snapshot.panelBlacklist, address); rule != nil {
		return AccessDecision{Allowed: false, Reason: "blacklisted", MatchedRule: rule}, nil
	}

	region := ""
	regionLoaded := false
	loadRegion := func() error {
		if regionLoaded {
			return nil
		}
		if lookup == nil {
			return ErrAccessControlGeoIPUnavailable
		}
		value, lookupErr := lookup(address.String())
		if lookupErr != nil {
			return fmt.Errorf("%w: %v", ErrAccessControlGeoIPUnavailable, lookupErr)
		}
		region = value
		regionLoaded = true
		return nil
	}

	if hasKeywordRules(snapshot.panelBlacklist) {
		if err := loadRegion(); err != nil {
			return AccessDecision{Allowed: false, Reason: "geoip_unavailable"}, err
		}
		if rule := matchKeywordRules(snapshot.panelBlacklist, region); rule != nil {
			return AccessDecision{Allowed: false, Reason: "blacklisted", Region: region, MatchedRule: rule}, nil
		}
	}

	if len(snapshot.panelWhitelist) == 0 {
		return AccessDecision{Allowed: true, Reason: "no_whitelist", Region: region}, nil
	}
	if rule := matchAddressRules(snapshot.panelWhitelist, address); rule != nil {
		return AccessDecision{Allowed: true, Reason: "whitelisted", Region: region, MatchedRule: rule}, nil
	}
	if hasKeywordRules(snapshot.panelWhitelist) {
		if err := loadRegion(); err != nil {
			return AccessDecision{Allowed: false, Reason: "geoip_unavailable"}, err
		}
		if rule := matchKeywordRules(snapshot.panelWhitelist, region); rule != nil {
			return AccessDecision{Allowed: true, Reason: "whitelisted", Region: region, MatchedRule: rule}, nil
		}
	}

	return AccessDecision{Allowed: false, Reason: "not_whitelisted", Region: region}, nil
}

func UpdatePanelAccessRules(expectedRevision uint64, update PanelRulesUpdate, input ClientIPInput, lookup RegionLookup) (AccessControlState, error) {
	return updateAccessControl(expectedRevision, input, lookup, func(config *AccessControlConfig) {
		config.Enabled = update.Enabled
		config.PanelBlacklist = cloneAccessRules(update.PanelBlacklist)
		config.PanelWhitelist = cloneAccessRules(update.PanelWhitelist)
	})
}

func UpdateTrustedProxies(expectedRevision uint64, trustedProxies []string, input ClientIPInput, lookup RegionLookup) (AccessControlState, error) {
	return updateAccessControl(expectedRevision, input, lookup, func(config *AccessControlConfig) {
		config.TrustedProxies = append([]string(nil), trustedProxies...)
	})
}

func PreviewPanelAccessRules(update PanelRulesUpdate, input ClientIPInput, testIP string, lookup RegionLookup) (AccessControlPreviewResult, error) {
	return previewAccessControl(input, testIP, lookup, func(config *AccessControlConfig) {
		config.Enabled = update.Enabled
		config.PanelBlacklist = cloneAccessRules(update.PanelBlacklist)
		config.PanelWhitelist = cloneAccessRules(update.PanelWhitelist)
	})
}

func PreviewTrustedProxies(trustedProxies []string, input ClientIPInput, testIP string, lookup RegionLookup) (AccessControlPreviewResult, error) {
	return previewAccessControl(input, testIP, lookup, func(config *AccessControlConfig) {
		config.TrustedProxies = append([]string(nil), trustedProxies...)
	})
}

func updateAccessControl(
	expectedRevision uint64,
	input ClientIPInput,
	lookup RegionLookup,
	mutate func(*AccessControlConfig),
) (AccessControlState, error) {
	accessControlMutex.Lock()
	defer accessControlMutex.Unlock()

	if expectedRevision != accessControlConfig.Revision {
		return AccessControlState{}, ErrAccessControlRevisionConflict
	}

	candidate := cloneAccessControlConfig(accessControlConfig)
	mutate(&candidate)
	candidate.Version = AccessControlConfigVersion
	candidate.Revision = accessControlConfig.Revision + 1

	canonical, snapshot, err := compileAccessControlConfig(candidate)
	if err != nil {
		return AccessControlState{}, err
	}
	if canonical.Enabled {
		if err := ensureCurrentAdministratorAllowed(snapshot, input, lookup); err != nil {
			return AccessControlState{}, err
		}
	}
	if err := saveAccessControlConfig(canonical); err != nil {
		return AccessControlState{}, fmt.Errorf("%w: %v", ErrAccessControlPersist, err)
	}

	accessControlConfig = canonical
	accessControlRecovery = false
	accessControlLoadErr = ""
	accessControlSnapshot.Store(snapshot)
	return AccessControlState{Config: cloneAccessControlConfig(canonical)}, nil
}

func previewAccessControl(
	input ClientIPInput,
	testIP string,
	lookup RegionLookup,
	mutate func(*AccessControlConfig),
) (AccessControlPreviewResult, error) {
	accessControlMutex.Lock()
	candidate := cloneAccessControlConfig(accessControlConfig)
	accessControlMutex.Unlock()

	mutate(&candidate)
	canonical, snapshot, err := compileAccessControlConfig(candidate)
	if err != nil {
		return AccessControlPreviewResult{}, err
	}
	connection, err := snapshot.ResolveClientIP(input)
	if err != nil {
		return AccessControlPreviewResult{}, err
	}
	decision, err := snapshot.Evaluate(connection.ClientIP, lookup)
	if err != nil && !errors.Is(err, ErrAccessControlRecoveryMode) {
		return AccessControlPreviewResult{}, err
	}

	result := AccessControlPreviewResult{
		Config:            canonical,
		CurrentConnection: connection,
		CurrentDecision:   decision,
	}
	if strings.TrimSpace(testIP) != "" {
		testAddress, parseErr := normalizeIP(testIP)
		if parseErr != nil {
			return AccessControlPreviewResult{}, fmt.Errorf("测试 IP 格式无效: %w", parseErr)
		}
		testDecision, evaluateErr := snapshot.Evaluate(testAddress.String(), lookup)
		if evaluateErr != nil && !errors.Is(evaluateErr, ErrAccessControlRecoveryMode) {
			return AccessControlPreviewResult{}, evaluateErr
		}
		result.TestDecision = &testDecision
	}
	return result, nil
}

func ensureCurrentAdministratorAllowed(snapshot *AccessControlSnapshot, input ClientIPInput, lookup RegionLookup) error {
	connection, err := snapshot.ResolveClientIP(input)
	if err != nil {
		return fmt.Errorf("%w: 新可信代理配置无法解析当前连接: %v", ErrAccessControlWouldLockOut, err)
	}
	decision, err := snapshot.Evaluate(connection.ClientIP, lookup)
	if err != nil {
		return fmt.Errorf("%w: 无法确认当前管理员可继续访问: %v", ErrAccessControlWouldLockOut, err)
	}
	if !decision.Allowed {
		return fmt.Errorf("%w: %s", ErrAccessControlWouldLockOut, decision.Reason)
	}
	return nil
}

func compileAccessControlConfig(config AccessControlConfig) (AccessControlConfig, *AccessControlSnapshot, error) {
	canonical := cloneAccessControlConfig(config)
	canonical.Version = AccessControlConfigVersion
	if canonical.Revision == 0 {
		canonical.Revision = 1
	}

	trusted, normalizedTrusted, err := normalizeTrustedProxies(canonical.TrustedProxies)
	if err != nil {
		return AccessControlConfig{}, nil, err
	}
	canonical.TrustedProxies = normalizedTrusted

	seenIDs := make(map[string]struct{})
	blacklist, normalizedBlacklist, err := normalizeAccessRules("面板黑名单", canonical.PanelBlacklist, seenIDs)
	if err != nil {
		return AccessControlConfig{}, nil, err
	}
	whitelist, normalizedWhitelist, err := normalizeAccessRules("面板白名单", canonical.PanelWhitelist, seenIDs)
	if err != nil {
		return AccessControlConfig{}, nil, err
	}
	canonical.PanelBlacklist = normalizedBlacklist
	canonical.PanelWhitelist = normalizedWhitelist

	snapshot := &AccessControlSnapshot{
		config:         cloneAccessControlConfig(canonical),
		trustedProxies: trusted,
		panelBlacklist: blacklist,
		panelWhitelist: whitelist,
	}
	return canonical, snapshot, nil
}

func normalizeTrustedProxies(values []string) ([]netip.Prefix, []string, error) {
	if len(values) > maxTrustedProxies {
		return nil, nil, fmt.Errorf("可信代理不能超过 %d 条", maxTrustedProxies)
	}

	prefixes := make([]netip.Prefix, 0, len(values))
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		prefix, err := normalizeIPOrPrefix(value)
		if err != nil {
			return nil, nil, fmt.Errorf("可信代理 %q 无效: %w", value, err)
		}
		if prefix.Bits() == 0 {
			return nil, nil, fmt.Errorf("禁止信任全部地址: %s", prefix.String())
		}
		key := prefix.String()
		if _, exists := seen[key]; exists {
			return nil, nil, fmt.Errorf("可信代理重复: %s", key)
		}
		seen[key] = struct{}{}
		prefixes = append(prefixes, prefix)
		normalized = append(normalized, key)
	}
	return prefixes, normalized, nil
}

func normalizeAccessRules(name string, rules []AccessRule, seenIDs map[string]struct{}) ([]compiledAccessRule, []AccessRule, error) {
	if len(rules) > maxAccessRules {
		return nil, nil, fmt.Errorf("%s不能超过 %d 条", name, maxAccessRules)
	}

	compiled := make([]compiledAccessRule, 0, len(rules))
	normalized := make([]AccessRule, 0, len(rules))
	seenValues := make(map[string]struct{}, len(rules))
	for _, input := range rules {
		rule := input
		rule.ID = strings.TrimSpace(rule.ID)
		if rule.ID == "" {
			generatedID, err := newAccessRuleID()
			if err != nil {
				return nil, nil, err
			}
			rule.ID = generatedID
		}
		if _, exists := seenIDs[rule.ID]; exists {
			return nil, nil, fmt.Errorf("规则 ID 重复: %s", rule.ID)
		}
		seenIDs[rule.ID] = struct{}{}

		rule.Type = strings.TrimSpace(strings.ToLower(rule.Type))
		rule.Value = strings.TrimSpace(rule.Value)
		rule.Remark = strings.TrimSpace(rule.Remark)
		if rule.Value == "" {
			return nil, nil, fmt.Errorf("%s规则值不能为空", name)
		}
		if utf8.RuneCountInString(rule.Value) > maxAccessRuleValueLength {
			return nil, nil, fmt.Errorf("%s规则值不能超过 %d 个字符", name, maxAccessRuleValueLength)
		}
		if utf8.RuneCountInString(rule.Remark) > maxAccessRuleRemarkLength {
			return nil, nil, fmt.Errorf("%s备注不能超过 %d 个字符", name, maxAccessRuleRemarkLength)
		}

		compiledRule := compiledAccessRule{rule: rule}
		switch rule.Type {
		case AccessRuleTypeKeyword:
		case AccessRuleTypeIP:
			address, err := normalizeIP(rule.Value)
			if err != nil {
				return nil, nil, fmt.Errorf("%s IP %q 无效: %w", name, rule.Value, err)
			}
			rule.Value = address.String()
			compiledRule.rule.Value = rule.Value
			compiledRule.address = address
		case AccessRuleTypeCIDR:
			prefix, err := normalizePrefix(rule.Value)
			if err != nil {
				return nil, nil, fmt.Errorf("%s CIDR %q 无效: %w", name, rule.Value, err)
			}
			rule.Value = prefix.String()
			compiledRule.rule.Value = rule.Value
			compiledRule.prefix = prefix
		default:
			return nil, nil, fmt.Errorf("%s规则类型无效: %s", name, rule.Type)
		}

		valueKey := rule.Type + "\x00" + rule.Value
		if _, exists := seenValues[valueKey]; exists {
			return nil, nil, fmt.Errorf("%s规则重复: %s", name, rule.Value)
		}
		seenValues[valueKey] = struct{}{}
		normalized = append(normalized, rule)
		if rule.Enabled {
			compiledRule.rule = rule
			compiled = append(compiled, compiledRule)
		}
	}
	return compiled, normalized, nil
}

func (snapshot *AccessControlSnapshot) isTrustedProxy(address netip.Addr) (bool, string) {
	address = address.Unmap()
	for _, prefix := range snapshot.trustedProxies {
		if prefix.Contains(address) {
			return true, prefix.String()
		}
	}
	return false, ""
}

func (snapshot *AccessControlSnapshot) parseForwardedFor(header string) (netip.Addr, error) {
	parts := strings.Split(header, ",")
	addresses := make([]netip.Addr, len(parts))
	for index, part := range parts {
		address, err := normalizeIP(part)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("X-Forwarded-For 格式无效: %w", err)
		}
		addresses[index] = address
	}
	for index := len(addresses) - 1; index >= 0; index-- {
		trusted, _ := snapshot.isTrustedProxy(addresses[index])
		if index == 0 || !trusted {
			return addresses[index], nil
		}
	}
	return netip.Addr{}, fmt.Errorf("X-Forwarded-For 为空")
}

func matchAddressRules(rules []compiledAccessRule, address netip.Addr) *AccessRule {
	for _, rule := range rules {
		switch rule.rule.Type {
		case AccessRuleTypeIP:
			if rule.address == address {
				matched := rule.rule
				return &matched
			}
		case AccessRuleTypeCIDR:
			if rule.prefix.Contains(address) {
				matched := rule.rule
				return &matched
			}
		}
	}
	return nil
}

func hasKeywordRules(rules []compiledAccessRule) bool {
	for _, rule := range rules {
		if rule.rule.Type == AccessRuleTypeKeyword {
			return true
		}
	}
	return false
}

func matchKeywordRules(rules []compiledAccessRule, region string) *AccessRule {
	for _, rule := range rules {
		if rule.rule.Type == AccessRuleTypeKeyword && strings.Contains(region, rule.rule.Value) {
			matched := rule.rule
			return &matched
		}
	}
	return nil
}

func parseRemoteAddress(remoteAddress string) (netip.Addr, error) {
	value := strings.TrimSpace(remoteAddress)
	if value == "" {
		return netip.Addr{}, fmt.Errorf("连接来源为空")
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return normalizeIP(strings.Trim(host, "[]"))
	}
	return normalizeIP(strings.Trim(value, "[]"))
}

func normalizeIP(value string) (netip.Addr, error) {
	address, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil || address.Zone() != "" {
		return netip.Addr{}, fmt.Errorf("无效 IP")
	}
	return address.Unmap(), nil
}

func normalizePrefix(value string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil || prefix.Addr().Zone() != "" {
		return netip.Prefix{}, fmt.Errorf("无效 CIDR")
	}
	address := prefix.Addr()
	bits := prefix.Bits()
	if address.Is4In6() {
		if bits < 96 {
			return netip.Prefix{}, fmt.Errorf("IPv4 映射 CIDR 前缀无效")
		}
		address = address.Unmap()
		bits -= 96
	}
	return netip.PrefixFrom(address, bits).Masked(), nil
}

func normalizeIPOrPrefix(value string) (netip.Prefix, error) {
	trimmed := strings.TrimSpace(value)
	if strings.Contains(trimmed, "/") {
		return normalizePrefix(trimmed)
	}
	address, err := normalizeIP(trimmed)
	if err != nil {
		return netip.Prefix{}, err
	}
	bits := 128
	if address.Is4() {
		bits = 32
	}
	return netip.PrefixFrom(address, bits), nil
}

func newAccessRuleID() (string, error) {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("生成规则 ID 失败: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func saveAccessControlConfig(config AccessControlConfig) error {
	if err := consts.EnsureManagerDataPath(); err != nil {
		return fmt.Errorf("创建管理器数据目录失败: %w", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化访问控制配置失败: %w", err)
	}

	directory := filepath.Dir(consts.AccessControlConfigPath)
	temporary, err := os.CreateTemp(directory, ".access-control-*.tmp")
	if err != nil {
		return fmt.Errorf("创建访问控制临时文件失败: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err = temporary.Chmod(0644); err == nil {
		_, err = temporary.Write(data)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("写入访问控制临时文件失败: %w", err)
	}
	if err = os.Rename(temporaryPath, consts.AccessControlConfigPath); err != nil {
		return fmt.Errorf("替换访问控制配置失败: %w", err)
	}
	return nil
}

func cloneAccessControlConfig(config AccessControlConfig) AccessControlConfig {
	config.TrustedProxies = append([]string(nil), config.TrustedProxies...)
	config.PanelBlacklist = cloneAccessRules(config.PanelBlacklist)
	config.PanelWhitelist = cloneAccessRules(config.PanelWhitelist)
	if config.TrustedProxies == nil {
		config.TrustedProxies = []string{}
	}
	if config.PanelBlacklist == nil {
		config.PanelBlacklist = []AccessRule{}
	}
	if config.PanelWhitelist == nil {
		config.PanelWhitelist = []AccessRule{}
	}
	return config
}

func cloneAccessRules(rules []AccessRule) []AccessRule {
	if rules == nil {
		return []AccessRule{}
	}
	return append([]AccessRule(nil), rules...)
}
