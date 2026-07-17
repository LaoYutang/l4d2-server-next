package controller

import "testing"

func TestFormatPluginConfigAuditDetailIncludesSortedChanges(t *testing.T) {
	detail := formatPluginConfigAuditDetail("example.cfg", map[string]string{
		"z_cvar": "value with spaces",
		"a_cvar": "42",
	})
	want := `更新插件配置: example.cfg，修改项: a_cvar="42", z_cvar="value with spaces"`
	if detail != want {
		t.Fatalf("unexpected audit detail:\nwant: %s\n got: %s", want, detail)
	}
}
