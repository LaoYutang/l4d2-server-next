package logic

import (
	"fmt"
	"strings"
)

// MapImportPolicy applies to new panel imports, including downloaded maps.
// Zero values preserve the existing warning-only behavior.
type MapImportPolicy struct {
	RejectMissingDictionary bool `json:"reject_missing_dictionary"`
	RejectGlobalScripts     bool `json:"reject_global_scripts"`
	RejectScriptOverrides   bool `json:"reject_script_overrides"`
	RejectInspectionError   bool `json:"reject_inspection_error"`
}

type MapImportViolation struct {
	Map     string   `json:"map"`
	Rule    string   `json:"rule"`
	Message string   `json:"message"`
	Paths   []string `json:"paths"`
}

type MapImportRejectedError struct {
	Violations []MapImportViolation `json:"violations"`
}

func (e *MapImportRejectedError) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, violation := range e.Violations {
		part := fmt.Sprintf("%s：%s", violation.Map, violation.Message)
		if len(violation.Paths) > 0 {
			part += "（" + strings.Join(violation.Paths, "、") + "）"
		}
		parts = append(parts, part)
	}
	return "风险检查未通过，本次导入未安装任何地图：" + strings.Join(parts, "；")
}

func GetMapImportPolicy() MapImportPolicy {
	managerConfigMutex.RLock()
	defer managerConfigMutex.RUnlock()
	return managerConfig.MapImportPolicy
}

func SetMapImportPolicy(policy MapImportPolicy) error {
	managerConfigMutex.Lock()
	defer managerConfigMutex.Unlock()
	previous := managerConfig.MapImportPolicy
	managerConfig.MapImportPolicy = policy
	if err := saveManagerConfig(); err != nil {
		managerConfig.MapImportPolicy = previous
		return err
	}
	return nil
}

// EvaluateMapImport uses fresh inspection results, never the installed-map cache.
func EvaluateMapImport(mapName string, inspection MapVPKInspection, policy MapImportPolicy) []MapImportViolation {
	violations := make([]MapImportViolation, 0)
	add := func(rule, message string, paths ...string) {
		violations = append(violations, MapImportViolation{Map: mapName, Rule: rule, Message: message, Paths: append([]string{}, paths...)})
	}
	unreadableChapter := false
	for _, chapter := range inspection.Dictionary.Chapters {
		if policy.RejectMissingDictionary && chapter.Status == DictionaryChapterMissing {
			add("missing_dictionary", "缺失 stringtable_dictionary.dct 字典", chapter.BSPPath)
		}
		if chapter.Status == DictionaryChapterUnreadable {
			unreadableChapter = true
			if policy.RejectInspectionError {
				message := chapter.Message
				if message == "" {
					message = "无法读取 BSP 或 Pakfile"
				}
				add("inspection_error", message, chapter.BSPPath)
			}
		}
	}
	if policy.RejectGlobalScripts && inspection.GlobalScripts.Status == GlobalScriptsStatusDetected {
		add("global_scripts", "包含全局自动执行脚本", inspection.GlobalScripts.Files...)
	}
	if policy.RejectScriptOverrides && inspection.ScriptOverrides.Status == ScriptOverridesStatusDetected {
		add("script_overrides", "包含游戏模式或武器脚本覆盖", inspection.ScriptOverrides.Files...)
	}
	if policy.RejectInspectionError {
		if inspection.Error != "" {
			add("inspection_error", inspection.Error)
		} else if (!unreadableChapter && inspection.Dictionary.Status == DictionaryStatusUnreadable) ||
			inspection.GlobalScripts.Status == GlobalScriptsStatusUnreadable || inspection.ScriptOverrides.Status == ScriptOverridesStatusUnreadable ||
			inspection.Dictionary.Status == DictionaryStatusNotChecked || inspection.GlobalScripts.Status == GlobalScriptsStatusNotChecked || inspection.ScriptOverrides.Status == ScriptOverridesStatusNotChecked {
			add("inspection_error", "无法完成 VPK 风险检查")
		}
	}
	return violations
}
