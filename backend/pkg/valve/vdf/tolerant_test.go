package vdf

import (
	"strings"
	"testing"
)

func TestReadTolerantAcceptsUnquotedKeysValuesCommentsAndMissingBraces(t *testing.T) {
	root, err := ReadTolerant(strings.NewReader(`mission
{
	DisplayTitle "Tolerant Campaign" // inline comment
	modes
	{
		coop
		{
			1
			{
				Map c1m1_alpha
				"DisplayName" Alpha
			}
		}
		versus
		{
			1
			{
				Map "c1m1_alpha"
				DisplayName "Alpha Versus"
			}
		}
`))
	if err != nil {
		t.Fatalf("ReadTolerant() error = %v", err)
	}

	if root.Key != "mission" {
		t.Fatalf("root.Key = %q, want mission", root.Key)
	}
	title := root.FindKey("DisplayTitle")
	if title == nil || title.Value != "Tolerant Campaign" {
		t.Fatalf("DisplayTitle = %+v, want Tolerant Campaign", title)
	}
	coopMap := root.FindKey("modes/coop/1/Map")
	if coopMap == nil || coopMap.Value != "c1m1_alpha" {
		t.Fatalf("coop Map = %+v, want c1m1_alpha", coopMap)
	}
	coopName := root.FindKey("modes/coop/1/DisplayName")
	if coopName == nil || coopName.Value != "Alpha" {
		t.Fatalf("coop DisplayName = %+v, want Alpha", coopName)
	}
	versusName := root.FindKey("modes/versus/1/DisplayName")
	if versusName == nil || versusName.Value != "Alpha Versus" {
		t.Fatalf("versus DisplayName = %+v, want Alpha Versus", versusName)
	}
}
