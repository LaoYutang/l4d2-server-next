package logic

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"l4d2-manager-next/consts"
)

func TestMapImportPolicyDefaultsAndPersistence(t *testing.T) {
	setupManagerConfigTest(t)
	if GetMapImportPolicy() != (MapImportPolicy{}) {
		t.Fatal("missing config must be warning-only")
	}
	if err := consts.EnsureManagerDataPath(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consts.ManagerConfigPath, []byte(`{"enable_self_service":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	LoadManagerConfig()
	if GetMapImportPolicy() != (MapImportPolicy{}) {
		t.Fatal("legacy config must be warning-only")
	}
	policy := MapImportPolicy{true, true, true, true}
	if err := SetMapImportPolicy(policy); err != nil {
		t.Fatal(err)
	}
	LoadManagerConfig()
	if GetMapImportPolicy() != policy || !GetSelfServiceConfig().EnableSelfService {
		t.Fatal("policy or unrelated setting lost on reload")
	}
	originalPath := consts.ManagerConfigPath
	consts.ManagerConfigPath = filepath.Join(t.TempDir(), "directory")
	if err := os.Mkdir(consts.ManagerConfigPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := SetMapImportPolicy(MapImportPolicy{}); err == nil {
		t.Fatal("save to directory should fail")
	}
	consts.ManagerConfigPath = originalPath
	if GetMapImportPolicy() != policy {
		t.Fatal("failed save changed live policy")
	}
}

func TestMapImportPolicyIndependentRulesAndMixedChapterErrors(t *testing.T) {
	inspection := MapVPKInspection{
		Dictionary: DictionaryInspection{Status: DictionaryStatusMissing, Chapters: []DictionaryChapterInspection{
			{BSPPath: "maps/missing.bsp", Status: DictionaryChapterMissing},
			{BSPPath: "maps/broken.bsp", Status: DictionaryChapterUnreadable, Message: "BSP 头部不完整"},
		}},
		GlobalScripts:   GlobalScriptsInspection{Status: GlobalScriptsStatusDetected, Files: []string{"scripts/vscripts/mapspawn_addon.nut"}},
		ScriptOverrides: ScriptOverridesInspection{Status: ScriptOverridesStatusDetected, Files: []string{"scripts/gamemodes.txt"}},
	}
	for _, test := range []struct {
		name   string
		policy MapImportPolicy
		rules  []string
	}{
		{"disabled", MapImportPolicy{}, []string{}},
		{"dictionary", MapImportPolicy{RejectMissingDictionary: true}, []string{"missing_dictionary"}},
		{"global", MapImportPolicy{RejectGlobalScripts: true}, []string{"global_scripts"}},
		{"overrides", MapImportPolicy{RejectScriptOverrides: true}, []string{"script_overrides"}},
		{"error even when aggregate is missing", MapImportPolicy{RejectInspectionError: true}, []string{"inspection_error"}},
		{"all", MapImportPolicy{true, true, true, true}, []string{"missing_dictionary", "inspection_error", "global_scripts", "script_overrides"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			violations := EvaluateMapImport("map.vpk", inspection, test.policy)
			rules := make([]string, 0, len(violations))
			for _, v := range violations {
				if v.Map != "map.vpk" || len(v.Paths) == 0 {
					t.Fatalf("missing location: %+v", v)
				}
				rules = append(rules, v.Rule)
			}
			if !reflect.DeepEqual(rules, test.rules) {
				t.Fatalf("rules = %v, want %v", rules, test.rules)
			}
		})
	}
}

func TestMapImportPolicyInspectionFailureAndNoBSP(t *testing.T) {
	policy := MapImportPolicy{true, true, true, true}
	path := filepath.Join(t.TempDir(), "broken.vpk")
	if err := os.WriteFile(path, []byte("not a supported vpk layout"), 0644); err != nil {
		t.Fatal(err)
	}
	violations := EvaluateMapImport("broken.vpk", InspectMapVPK(path), policy)
	if len(violations) != 1 || violations[0].Rule != "inspection_error" || violations[0].Message == "" {
		t.Fatalf("parse failure = %+v", violations)
	}
	writeMapInspectionTestVPK(t, path, map[string][]byte{"missions/test.txt": []byte("metadata")})
	if violations := EvaluateMapImport("no-bsp.vpk", InspectMapVPK(path), policy); len(violations) != 0 {
		t.Fatalf("no BSP is not a missing dictionary: %+v", violations)
	}
}
