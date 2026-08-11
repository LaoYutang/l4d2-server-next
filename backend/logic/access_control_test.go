package logic

import (
	"errors"
	"l4d2-manager-next/consts"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func setupAccessControlTest(t *testing.T) {
	t.Helper()
	oldManagerDataPath := consts.ManagerDataPath
	oldAccessControlConfigPath := consts.AccessControlConfigPath

	consts.ManagerDataPath = filepath.Join(t.TempDir(), "data")
	consts.AccessControlConfigPath = filepath.Join(consts.ManagerDataPath, "access_control.json")
	InitAccessControl()

	t.Cleanup(func() {
		consts.ManagerDataPath = oldManagerDataPath
		consts.AccessControlConfigPath = oldAccessControlConfigPath
		InitAccessControl()
	})
}

func enabledRule(ruleType, value string) AccessRule {
	return AccessRule{Enabled: true, Type: ruleType, Value: value}
}

func TestAccessControlDefaultsAndExplicitEmptyTrustedProxiesPersist(t *testing.T) {
	t.Setenv("REGION_WHITE_LIST", "198.51.100.10")
	setupAccessControlTest(t)

	state := GetAccessControlState()
	if state.Config.Enabled {
		t.Fatal("access control enabled by default")
	}
	wantDefaults := []string{"127.0.0.0/8", "::1/128"}
	if !reflect.DeepEqual(state.Config.TrustedProxies, wantDefaults) {
		t.Fatalf("default trusted proxies = %v, want %v", state.Config.TrustedProxies, wantDefaults)
	}
	if len(state.Config.PanelWhitelist) != 0 {
		t.Fatalf("legacy REGION_WHITE_LIST unexpectedly populated panel whitelist: %v", state.Config.PanelWhitelist)
	}

	updated, err := UpdateTrustedProxies(
		state.Config.Revision,
		[]string{},
		ClientIPInput{RemoteAddr: "127.0.0.1:27020"},
		nil,
	)
	if err != nil {
		t.Fatalf("UpdateTrustedProxies(empty) error = %v", err)
	}
	if len(updated.Config.TrustedProxies) != 0 {
		t.Fatalf("updated trusted proxies = %v, want empty", updated.Config.TrustedProxies)
	}

	InitAccessControl()
	reloaded := GetAccessControlState()
	if len(reloaded.Config.TrustedProxies) != 0 {
		t.Fatalf("reloaded trusted proxies = %v, want explicit empty", reloaded.Config.TrustedProxies)
	}
}

func TestResolveClientIPTrustsOnlyConfiguredPeers(t *testing.T) {
	config := defaultAccessControlConfig()
	_, snapshot, err := compileAccessControlConfig(config)
	if err != nil {
		t.Fatalf("compile default config: %v", err)
	}

	direct, err := snapshot.ResolveClientIP(ClientIPInput{
		RemoteAddr:    "203.0.113.20:40000",
		XForwardedFor: "127.0.0.1",
		XRealIP:       "192.168.1.10",
	})
	if err != nil {
		t.Fatalf("resolve untrusted peer: %v", err)
	}
	if direct.ClientIP != "203.0.113.20" || direct.Source != "remote_addr" || direct.PeerTrusted {
		t.Fatalf("untrusted peer resolved as %+v", direct)
	}

	forwarded, err := snapshot.ResolveClientIP(ClientIPInput{
		RemoteAddr:    "127.0.0.2:40000",
		XForwardedFor: "198.51.100.9, 127.0.0.3",
	})
	if err != nil {
		t.Fatalf("resolve trusted XFF chain: %v", err)
	}
	if forwarded.ClientIP != "198.51.100.9" || forwarded.Source != "x_forwarded_for" || !forwarded.PeerTrusted {
		t.Fatalf("trusted XFF resolved as %+v", forwarded)
	}

	config.TrustedProxies = append(config.TrustedProxies, "10.0.0.0/8")
	_, multiProxySnapshot, err := compileAccessControlConfig(config)
	if err != nil {
		t.Fatalf("compile multi-proxy config: %v", err)
	}
	multiLayer, err := multiProxySnapshot.ResolveClientIP(ClientIPInput{
		RemoteAddr:    "10.0.0.3:40000",
		XForwardedFor: "198.51.100.9, 203.0.113.5, 10.0.0.2",
	})
	if err != nil {
		t.Fatalf("resolve multi-layer XFF chain: %v", err)
	}
	if multiLayer.ClientIP != "203.0.113.5" {
		t.Fatalf("multi-layer XFF crossed an untrusted hop: %+v", multiLayer)
	}

	realIP, err := snapshot.ResolveClientIP(ClientIPInput{
		RemoteAddr: "[::1]:40000",
		XRealIP:    "2001:db8::20",
	})
	if err != nil {
		t.Fatalf("resolve X-Real-IP: %v", err)
	}
	if realIP.ClientIP != "2001:db8::20" || realIP.Source != "x_real_ip" {
		t.Fatalf("X-Real-IP resolved as %+v", realIP)
	}

	if _, err := snapshot.ResolveClientIP(ClientIPInput{
		RemoteAddr:    "127.0.0.1:40000",
		XForwardedFor: "not-an-ip",
	}); err == nil {
		t.Fatal("malformed X-Forwarded-For accepted from trusted peer")
	}
	if _, err := snapshot.ResolveClientIP(ClientIPInput{
		RemoteAddr: "127.0.0.1:40000",
		XRealIP:    "not-an-ip",
	}); err == nil {
		t.Fatal("malformed X-Real-IP accepted from trusted peer")
	}
}

func TestPanelAccessDecisionOrderAndPrivateNetworkBehavior(t *testing.T) {
	config := defaultAccessControlConfig()
	config.Enabled = true
	config.PanelBlacklist = []AccessRule{enabledRule(AccessRuleTypeCIDR, "203.0.113.0/24")}
	config.PanelWhitelist = []AccessRule{
		enabledRule(AccessRuleTypeCIDR, "203.0.113.0/24"),
		enabledRule(AccessRuleTypeIP, "198.51.100.10"),
	}
	_, snapshot, err := compileAccessControlConfig(config)
	if err != nil {
		t.Fatalf("compile config: %v", err)
	}

	decision, err := snapshot.Evaluate("203.0.113.5", nil)
	if err != nil || decision.Allowed || decision.Reason != "blacklisted" {
		t.Fatalf("blacklist did not win: decision=%+v err=%v", decision, err)
	}
	decision, err = snapshot.Evaluate("198.51.100.10", nil)
	if err != nil || !decision.Allowed || decision.Reason != "whitelisted" {
		t.Fatalf("explicit whitelist did not allow: decision=%+v err=%v", decision, err)
	}
	decision, err = snapshot.Evaluate("192.168.1.10", nil)
	if err != nil || decision.Allowed || decision.Reason != "not_whitelisted" {
		t.Fatalf("private IP was implicitly allowed: decision=%+v err=%v", decision, err)
	}
	decision, err = snapshot.Evaluate("127.0.0.8", nil)
	if err != nil || !decision.Allowed || decision.Reason != "loopback" {
		t.Fatalf("loopback was not allowed: decision=%+v err=%v", decision, err)
	}
}

func TestPanelRulesSupportKeywordAndIPv6CIDR(t *testing.T) {
	config := defaultAccessControlConfig()
	config.Enabled = true
	config.PanelBlacklist = []AccessRule{enabledRule(AccessRuleTypeKeyword, "广东")}
	config.PanelWhitelist = []AccessRule{
		enabledRule(AccessRuleTypeCIDR, "192.168.10.0/24"),
		enabledRule(AccessRuleTypeCIDR, "2001:db8::/32"),
	}
	_, snapshot, err := compileAccessControlConfig(config)
	if err != nil {
		t.Fatalf("compile keyword/IPv6 config: %v", err)
	}
	lookup := func(ip string) (string, error) {
		if ip == "198.51.100.8" {
			return "中国|广东省|深圳市|电信|CN", nil
		}
		return "美国|0|0|0|US", nil
	}

	blocked, err := snapshot.Evaluate("198.51.100.8", lookup)
	if err != nil || blocked.Allowed || blocked.Reason != "blacklisted" || blocked.Region == "" {
		t.Fatalf("keyword blacklist decision=%+v err=%v", blocked, err)
	}
	privateAllowed, err := snapshot.Evaluate("192.168.10.42", lookup)
	if err != nil || !privateAllowed.Allowed || privateAllowed.Reason != "whitelisted" {
		t.Fatalf("explicit private CIDR decision=%+v err=%v", privateAllowed, err)
	}
	ipv6Allowed, err := snapshot.Evaluate("2001:db8:abcd::1", lookup)
	if err != nil || !ipv6Allowed.Allowed || ipv6Allowed.Reason != "whitelisted" {
		t.Fatalf("IPv6 CIDR decision=%+v err=%v", ipv6Allowed, err)
	}

	config.PanelBlacklist = []AccessRule{}
	config.PanelWhitelist = []AccessRule{}
	_, emptyWhitelistSnapshot, err := compileAccessControlConfig(config)
	if err != nil {
		t.Fatalf("compile empty whitelist config: %v", err)
	}
	allowed, err := emptyWhitelistSnapshot.Evaluate("203.0.113.22", nil)
	if err != nil || !allowed.Allowed || allowed.Reason != "no_whitelist" {
		t.Fatalf("empty whitelist decision=%+v err=%v", allowed, err)
	}
}

func TestGeoIPLookupRunsOnlyWhenDecisionDependsOnKeyword(t *testing.T) {
	config := defaultAccessControlConfig()
	config.Enabled = true
	config.PanelWhitelist = []AccessRule{
		enabledRule(AccessRuleTypeIP, "198.51.100.10"),
		enabledRule(AccessRuleTypeKeyword, "中国"),
	}
	_, snapshot, err := compileAccessControlConfig(config)
	if err != nil {
		t.Fatalf("compile config: %v", err)
	}

	lookupCalls := 0
	failingLookup := func(string) (string, error) {
		lookupCalls++
		return "", errors.New("GeoIP offline")
	}
	decision, err := snapshot.Evaluate("198.51.100.10", failingLookup)
	if err != nil || !decision.Allowed || lookupCalls != 0 {
		t.Fatalf("exact IP unnecessarily depended on GeoIP: decision=%+v err=%v calls=%d", decision, err, lookupCalls)
	}

	decision, err = snapshot.Evaluate("203.0.113.10", failingLookup)
	if !errors.Is(err, ErrAccessControlGeoIPUnavailable) || decision.Reason != "geoip_unavailable" || lookupCalls != 1 {
		t.Fatalf("keyword dependency did not fail closed: decision=%+v err=%v calls=%d", decision, err, lookupCalls)
	}
}

func TestPanelRuleUpdateRejectsCurrentAdministratorLockout(t *testing.T) {
	setupAccessControlTest(t)
	state := GetAccessControlState()
	input := ClientIPInput{RemoteAddr: "198.51.100.20:45000"}

	_, err := UpdatePanelAccessRules(state.Config.Revision, PanelRulesUpdate{
		Enabled:        true,
		PanelBlacklist: []AccessRule{},
		PanelWhitelist: []AccessRule{enabledRule(AccessRuleTypeIP, "203.0.113.10")},
	}, input, nil)
	if !errors.Is(err, ErrAccessControlWouldLockOut) {
		t.Fatalf("whitelist lockout error = %v, want ErrAccessControlWouldLockOut", err)
	}
	if got := GetAccessControlState().Config.Revision; got != state.Config.Revision {
		t.Fatalf("revision changed after rejected update: got %d want %d", got, state.Config.Revision)
	}
	if _, statErr := os.Stat(consts.AccessControlConfigPath); !os.IsNotExist(statErr) {
		t.Fatalf("rejected update persisted a config file: %v", statErr)
	}

	_, err = UpdatePanelAccessRules(state.Config.Revision, PanelRulesUpdate{
		Enabled:        true,
		PanelBlacklist: []AccessRule{enabledRule(AccessRuleTypeIP, "198.51.100.20")},
		PanelWhitelist: []AccessRule{},
	}, input, nil)
	if !errors.Is(err, ErrAccessControlWouldLockOut) {
		t.Fatalf("blacklist lockout error = %v, want ErrAccessControlWouldLockOut", err)
	}

	updated, err := UpdatePanelAccessRules(state.Config.Revision, PanelRulesUpdate{
		Enabled:        false,
		PanelBlacklist: []AccessRule{enabledRule(AccessRuleTypeIP, "198.51.100.20")},
		PanelWhitelist: []AccessRule{},
	}, input, nil)
	if err != nil {
		t.Fatalf("disabled draft was rejected: %v", err)
	}
	if updated.Config.Enabled || updated.Config.Revision != state.Config.Revision+1 {
		t.Fatalf("disabled draft state = %+v", updated.Config)
	}
}

func TestTrustedProxyUpdateRejectsReparsedAdministratorLockout(t *testing.T) {
	setupAccessControlTest(t)
	state := GetAccessControlState()
	input := ClientIPInput{
		RemoteAddr:    "10.0.0.2:45000",
		XForwardedFor: "203.0.113.5",
	}

	enabled, err := UpdatePanelAccessRules(state.Config.Revision, PanelRulesUpdate{
		Enabled:        true,
		PanelBlacklist: []AccessRule{},
		PanelWhitelist: []AccessRule{enabledRule(AccessRuleTypeIP, "10.0.0.2")},
	}, input, nil)
	if err != nil {
		t.Fatalf("enable panel rules for direct peer: %v", err)
	}
	before, err := os.ReadFile(consts.AccessControlConfigPath)
	if err != nil {
		t.Fatalf("read config before rejected proxy update: %v", err)
	}

	_, err = UpdateTrustedProxies(enabled.Config.Revision, []string{"10.0.0.0/8"}, input, nil)
	if !errors.Is(err, ErrAccessControlWouldLockOut) {
		t.Fatalf("proxy reparse lockout error = %v, want ErrAccessControlWouldLockOut", err)
	}
	after, err := os.ReadFile(consts.AccessControlConfigPath)
	if err != nil {
		t.Fatalf("read config after rejected proxy update: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rejected trusted proxy update changed persisted configuration")
	}
	current := GetAccessControlState()
	if current.Config.Revision != enabled.Config.Revision || !reflect.DeepEqual(current.Config.TrustedProxies, []string{"127.0.0.0/8", "::1/128"}) {
		t.Fatalf("rejected trusted proxy update changed runtime state: %+v", current.Config)
	}
}

func TestIndependentUpdatesPreserveOtherSection(t *testing.T) {
	setupAccessControlTest(t)
	state := GetAccessControlState()
	rule := enabledRule(AccessRuleTypeKeyword, "中国")

	rulesState, err := UpdatePanelAccessRules(state.Config.Revision, PanelRulesUpdate{
		Enabled:        false,
		PanelBlacklist: []AccessRule{rule},
		PanelWhitelist: []AccessRule{},
	}, ClientIPInput{RemoteAddr: "127.0.0.1:27020"}, nil)
	if err != nil {
		t.Fatalf("update panel rules: %v", err)
	}
	proxyState, err := UpdateTrustedProxies(
		rulesState.Config.Revision,
		[]string{"10.0.0.5"},
		ClientIPInput{RemoteAddr: "127.0.0.1:27020"},
		nil,
	)
	if err != nil {
		t.Fatalf("update trusted proxies: %v", err)
	}
	if len(proxyState.Config.PanelBlacklist) != 1 || proxyState.Config.PanelBlacklist[0].Value != "中国" {
		t.Fatalf("proxy update changed panel rules: %+v", proxyState.Config.PanelBlacklist)
	}
	if !reflect.DeepEqual(proxyState.Config.TrustedProxies, []string{"10.0.0.5/32"}) {
		t.Fatalf("trusted proxies = %v", proxyState.Config.TrustedProxies)
	}
	if proxyState.Config.Revision != state.Config.Revision+2 {
		t.Fatalf("revision = %d, want %d", proxyState.Config.Revision, state.Config.Revision+2)
	}
}

func TestConcurrentAccessControlHotUpdates(t *testing.T) {
	setupAccessControlTest(t)

	var readers sync.WaitGroup
	errorsSeen := make(chan error, 8)
	for worker := 0; worker < 8; worker++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for iteration := 0; iteration < 500; iteration++ {
				snapshot := CurrentAccessControlSnapshot()
				connection, err := snapshot.ResolveClientIP(ClientIPInput{
					RemoteAddr:    "127.0.0.1:45000",
					XForwardedFor: "198.51.100.10",
				})
				if err != nil {
					errorsSeen <- err
					return
				}
				if _, err = snapshot.Evaluate(connection.ClientIP, nil); err != nil {
					errorsSeen <- err
					return
				}
			}
		}()
	}

	for iteration := 0; iteration < 40; iteration++ {
		state := GetAccessControlState()
		proxies := []string{"127.0.0.0/8", "::1/128"}
		if iteration%2 == 1 {
			proxies = []string{}
		}
		if _, err := UpdateTrustedProxies(
			state.Config.Revision,
			proxies,
			ClientIPInput{RemoteAddr: "127.0.0.1:45000"},
			nil,
		); err != nil {
			t.Fatalf("hot update %d: %v", iteration, err)
		}
	}
	readers.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatalf("concurrent snapshot reader failed: %v", err)
	}
}

func TestMalformedConfigEntersLoopbackRecoveryMode(t *testing.T) {
	setupAccessControlTest(t)
	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatalf("create manager data directory: %v", err)
	}
	if err := os.WriteFile(consts.AccessControlConfigPath, []byte("{invalid"), 0644); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}
	InitAccessControl()

	state := GetAccessControlState()
	if !state.RecoveryMode || state.LoadError == "" {
		t.Fatalf("malformed config state = %+v", state)
	}
	snapshot := CurrentAccessControlSnapshot()
	loopback, err := snapshot.Evaluate("127.0.0.1", nil)
	if err != nil || !loopback.Allowed {
		t.Fatalf("loopback blocked in recovery mode: decision=%+v err=%v", loopback, err)
	}
	external, err := snapshot.Evaluate("198.51.100.10", nil)
	if !errors.Is(err, ErrAccessControlRecoveryMode) || external.Allowed {
		t.Fatalf("external IP allowed in recovery mode: decision=%+v err=%v", external, err)
	}

	recovered, err := UpdateTrustedProxies(
		state.Config.Revision,
		[]string{"127.0.0.1"},
		ClientIPInput{RemoteAddr: "127.0.0.1:27020"},
		nil,
	)
	if err != nil {
		t.Fatalf("save recovery config: %v", err)
	}
	if recovered.RecoveryMode || GetAccessControlState().RecoveryMode {
		t.Fatal("valid save did not exit recovery mode")
	}
}

func TestTrustedProxyValidationRejectsTrustAllAndDuplicates(t *testing.T) {
	for _, values := range [][]string{
		{"0.0.0.0/0"},
		{"::/0"},
		{"127.0.0.1", "127.0.0.1/32"},
	} {
		config := defaultAccessControlConfig()
		config.TrustedProxies = values
		if _, _, err := compileAccessControlConfig(config); err == nil {
			t.Fatalf("trusted proxies %v accepted, want error", values)
		}
	}
}
