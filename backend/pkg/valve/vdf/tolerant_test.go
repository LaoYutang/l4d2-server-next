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

func TestReadTolerantClosesLineBrokenQuotedValue(t *testing.T) {
	root, err := ReadTolerant(strings.NewReader(`"mission"
{
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "zc1_m5"
				"DisplayName" "东门桥/Bridge
				"Image" "maps/z5"
			}
		}
	}
}`))
	if err != nil {
		t.Fatalf("ReadTolerant() error = %v", err)
	}

	displayName := root.FindKey("modes/coop/1/DisplayName")
	if displayName == nil || displayName.Value != "东门桥/Bridge" {
		t.Fatalf("DisplayName = %+v, want repaired value", displayName)
	}
	image := root.FindKey("modes/coop/1/Image")
	if image == nil || image.Value != "maps/z5" {
		t.Fatalf("Image = %+v, want maps/z5", image)
	}
}

func TestReadTolerantRemovesTrailingExtraCloseBraces(t *testing.T) {
	root, err := ReadTolerant(strings.NewReader(`"mission"
{
	"DisplayTitle" "Extra } Campaign"
	// A brace in a comment must not affect balancing: }
}
} // extra close brace
}`))
	if err != nil {
		t.Fatalf("ReadTolerant() error = %v", err)
	}

	if root.Key != "mission" {
		t.Fatalf("root.Key = %q, want mission", root.Key)
	}
	title := root.FindKey("DisplayTitle")
	if title == nil || title.Value != "Extra } Campaign" {
		t.Fatalf("DisplayTitle = %+v, want repaired campaign title", title)
	}
}

func TestReadTolerantRejectsExtraCloseBraceBeforeMoreContent(t *testing.T) {
	_, err := ReadTolerant(strings.NewReader(`"mission"
{
	"DisplayTitle" "Invalid Campaign"
}
}
"unexpected" "value"`))
	if err == nil {
		t.Fatal("ReadTolerant() error = nil, want non-trailing close brace error")
	}
}
