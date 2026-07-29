package vpkmission

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseMissionDataUsesOriginalStrictParserFirst(t *testing.T) {
	result, err := parseMissionData([]byte(`"mission"
{
	"DisplayTitle" "Strict Campaign"
	"modes"
	{
		"custommode"
		{
			"1"
			{
				"Map" "strict_m1"
				"DisplayName" "Strict One"
			}
		}
		"mutation1"
		{
			"1"
			{
				"Map" "strict_m1"
				"DisplayName" "Strict Mutation"
			}
		}
	}
}`))
	if err != nil {
		t.Fatalf("parseMissionData() error = %v", err)
	}
	if result.Recovered {
		t.Fatalf("parseMissionData() Recovered = true, want original strict parser result: %+v", result.Issues)
	}
	assertChapter(
		t,
		result.Campaign.Chapters[0],
		"strict_m1",
		"Strict One",
		[]string{"custommode", "mutation1"},
	)
}

func TestParseMissionRecoversBraceDamage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTitle string
		want      []Chapter
	}{
		{
			name: "multiple extra open braces and scalar wrappers",
			input: `"mission"
{
	"DisplayTitle" "Extra Open Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{{
				"Map" {{ "extra_open_m1" }}
				"DisplayName" "Extra Open One"
			}}
		}
	}
}`,
			wantTitle: "Extra Open Campaign",
			want: []Chapter{{
				Code: "extra_open_m1", Title: "Extra Open One", Modes: []string{"coop"},
			}},
		},
		{
			name: "multiple extra close braces between chapters",
			input: `"mission"
{
	"DisplayTitle" "Extra Close Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "extra_close_m1"
				"DisplayName" "Extra Close One"
			}
		}
		}
			"2"
			{
				"Map" "extra_close_m2"
				"DisplayName" "Extra Close Two"
			}
	}
}`,
			wantTitle: "Extra Close Campaign",
			want: []Chapter{
				{Code: "extra_close_m1", Title: "Extra Close One", Modes: []string{"coop"}},
				{Code: "extra_close_m2", Title: "Extra Close Two", Modes: []string{"coop"}},
			},
		},
		{
			name: "missing close brace between chapters",
			input: `"mission"
{
	"DisplayTitle" "Missing Close Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "missing_close_m1"
				"DisplayName" "Missing Close One"
			"2"
			{
				"Map" "missing_close_m2"
				"DisplayName" "Missing Close Two"
			}
		}
	}
}`,
			wantTitle: "Missing Close Campaign",
			want: []Chapter{
				{Code: "missing_close_m1", Title: "Missing Close One", Modes: []string{"coop"}},
				{Code: "missing_close_m2", Title: "Missing Close Two", Modes: []string{"coop"}},
			},
		},
		{
			name: "missing open braces for every semantic section",
			input: `mission
	DisplayTitle Openless Campaign
	modes
		coop
			1
				Map openless_m1
				DisplayName Openless First Chapter`,
			wantTitle: "Openless Campaign",
			want: []Chapter{{
				Code: "openless_m1", Title: "Openless First Chapter", Modes: []string{"coop"},
			}},
		},
		{
			name: "missing close braces at end of file",
			input: `"mission"
{
	"DisplayTitle" "Open End Campaign"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "open_end_m1"
				"DisplayName" "Open End One"`,
			wantTitle: "Open End Campaign",
			want: []Chapter{{
				Code: "open_end_m1", Title: "Open End One", Modes: []string{"coop"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseMissionData([]byte(tt.input))
			if err != nil {
				t.Fatalf("parseMissionData() error = %v", err)
			}
			if !result.Recovered {
				t.Fatal("parseMissionData() Recovered = false, want semantic recovery")
			}
			if result.Campaign.Title != tt.wantTitle {
				t.Fatalf("Title = %q, want %q", result.Campaign.Title, tt.wantTitle)
			}
			if len(result.Campaign.Chapters) != len(tt.want) {
				t.Fatalf(
					"len(Chapters) = %d, want %d: %+v; issues: %+v",
					len(result.Campaign.Chapters),
					len(tt.want),
					result.Campaign.Chapters,
					result.Issues,
				)
			}
			for i := range tt.want {
				assertChapter(
					t,
					result.Campaign.Chapters[i],
					tt.want[i].Code,
					tt.want[i].Title,
					tt.want[i].Modes,
				)
			}
		})
	}
}

func TestParseMissionRecoversMissingAndOneSidedQuotes(t *testing.T) {
	result, err := parseMissionData([]byte(`mission
{
	DisplayTitle Broken Quote Campaign
	modes
	{
		coop
		{
			1
			{
				Map quote_m1
				DisplayName "First Chapter
			}
			2
			{
				"Map quote_m2"
				DisplayName Second Chapter"
			}
			3
			{
				Map { "quote_m3" }
				"DisplayName Third Chapter"
			}
			4
			{
				Map" "quote_m4"
				"DisplayName "Fourth Chapter"
			}
		}
	}
}`))
	if err != nil {
		t.Fatalf("parseMissionData() error = %v", err)
	}
	if !result.Recovered {
		t.Fatal("parseMissionData() Recovered = false, want semantic recovery")
	}
	if result.Campaign.Title != "Broken Quote Campaign" {
		t.Fatalf("Title = %q, want Broken Quote Campaign", result.Campaign.Title)
	}
	if len(result.Campaign.Chapters) != 4 {
		t.Fatalf("len(Chapters) = %d, want 4: %+v", len(result.Campaign.Chapters), result.Campaign.Chapters)
	}
	assertChapter(t, result.Campaign.Chapters[0], "quote_m1", "First Chapter", []string{"coop"})
	assertChapter(t, result.Campaign.Chapters[1], "quote_m2", "Second Chapter", []string{"coop"})
	assertChapter(t, result.Campaign.Chapters[2], "quote_m3", "Third Chapter", []string{"coop"})
	assertChapter(t, result.Campaign.Chapters[3], "quote_m4", "Fourth Chapter", []string{"coop"})
	if !hasRecoveryIssue(result.Issues, "missing_quote") {
		t.Fatalf("issues = %+v, want missing_quote", result.Issues)
	}
}

func TestStrictSemanticValidationRecoversBalancedWrongTree(t *testing.T) {
	result, err := parseMissionData([]byte(`"mission"
{
	"DisplayTitle" "Balanced Wrong Tree"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "balanced_m1"
				"DisplayName" "Balanced One"
				"2"
				{
					"Map" "balanced_m2"
					"DisplayName" "Balanced Two"
				}
			}
		}
	}
}`))
	if err != nil {
		t.Fatalf("parseMissionData() error = %v", err)
	}
	if !result.Recovered {
		t.Fatal("parseMissionData() Recovered = false, want semantic validation fallback")
	}
	if !hasRecoveryIssue(result.Issues, "strict_semantic_validation_failed") {
		t.Fatalf("issues = %+v, want strict_semantic_validation_failed", result.Issues)
	}
	if len(result.Campaign.Chapters) != 2 {
		t.Fatalf("len(Chapters) = %d, want 2: %+v", len(result.Campaign.Chapters), result.Campaign.Chapters)
	}
	assertChapter(t, result.Campaign.Chapters[0], "balanced_m1", "Balanced One", []string{"coop"})
	assertChapter(t, result.Campaign.Chapters[1], "balanced_m2", "Balanced Two", []string{"coop"})
}

func TestParseMissionRecoversMissingCloseBraceBetweenModes(t *testing.T) {
	result, err := parseMissionData([]byte(`"mission"
{
	"DisplayTitle" "Broken Modes"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "shared_broken_m1"
				"DisplayName" "Shared Broken One"
			}
		"mutation1"
		{
			"1"
			{
				"Map" "shared_broken_m1"
				"DisplayName" "Mutation Name"
			}
		}
	}
}`))
	if err != nil {
		t.Fatalf("parseMissionData() error = %v", err)
	}
	if !result.Recovered {
		t.Fatal("parseMissionData() Recovered = false, want semantic recovery")
	}
	if len(result.Campaign.Chapters) != 1 {
		t.Fatalf("len(Chapters) = %d, want 1: %+v", len(result.Campaign.Chapters), result.Campaign.Chapters)
	}
	assertChapter(
		t,
		result.Campaign.Chapters[0],
		"shared_broken_m1",
		"Shared Broken One",
		[]string{"coop", "mutation1"},
	)
}

func TestParseMissionKeepsMapWhenMetadataIsAmbiguous(t *testing.T) {
	result, err := parseMissionData([]byte(`"mission"
{
	"DisplayName" "Loose Chapter"
	"Map" "loose_m1"
}`))
	if err != nil {
		t.Fatalf("parseMissionData() error = %v", err)
	}
	if len(result.Campaign.Chapters) != 1 {
		t.Fatalf("len(Chapters) = %d, want 1: %+v", len(result.Campaign.Chapters), result.Campaign.Chapters)
	}
	assertChapter(t, result.Campaign.Chapters[0], "loose_m1", "Loose Chapter", []string{})
}

func TestStrictParserIgnoresCommentsAndBracesInQuotedValues(t *testing.T) {
	result, err := parseMissionData([]byte(`"mission"
{
	"DisplayTitle" "Quoted { Campaign }"
	"modes"
	{
		"coop"
		{
			"1"
			{
				"Map" "quoted_m1"
				"DisplayName" "Map { } // remains text"
				// "Map" "commented_out"
			}
		}
	}
}`))
	if err != nil {
		t.Fatalf("parseMissionData() error = %v", err)
	}
	if result.Recovered {
		t.Fatalf("parseMissionData() Recovered = true, want strict result: %+v", result.Issues)
	}
	assertChapter(t, result.Campaign.Chapters[0], "quoted_m1", "Map { } // remains text", []string{"coop"})
}

func TestParseVPKSkipsUnrecoverableMissionWhenAnotherIsValid(t *testing.T) {
	vpkPath := filepathJoinForTest(t, "partial.vpk")
	writeTestVPK(t, vpkPath, map[string]string{
		"missions/bad.txt": `"mission" { "DisplayTitle" "No chapters" }`,
		"missions/good.txt": `"mission"
{
	"DisplayTitle" "Good Campaign"
	"modes"
	{
		"coop"
		{
			"1" { "Map" "good_m1" "DisplayName" "Good One" }
		}
	}
}`,
	})

	campaigns, err := ParseVPK(vpkPath)
	if err != nil {
		t.Fatalf("ParseVPK() error = %v", err)
	}
	if len(campaigns) != 1 || campaigns[0].Title != "Good Campaign" {
		t.Fatalf("campaigns = %+v, want only Good Campaign", campaigns)
	}
}

func TestParseVPKReturnsErrorWhenAllMissionFilesAreUnrecoverable(t *testing.T) {
	vpkPath := filepathJoinForTest(t, "all-bad.vpk")
	writeTestVPK(t, vpkPath, map[string]string{
		"missions/one.txt": `"mission" { "DisplayTitle" "One" }`,
		"missions/two.txt": `not a mission file`,
	})

	_, err := ParseVPK(vpkPath)
	if err == nil {
		t.Fatal("ParseVPK() error = nil, want all missions unrecoverable error")
	}
	if !strings.Contains(err.Error(), "missions/one.txt") || !strings.Contains(err.Error(), "missions/two.txt") {
		t.Fatalf("ParseVPK() error = %q, want both mission paths", err)
	}
}

func FuzzParseMissionRecovery(f *testing.F) {
	seeds := []string{
		`"mission" { "modes" { "coop" { "1" { "Map" "seed_m1" } } } }`,
		`mission { modes { coop { 1 {{ Map {"seed_m2"} } } }`,
		`mission modes coop 1 Map seed_m3 DisplayName Seed Three`,
		`"mission" }}} "modes" { "coop" { "1" { "Map" "seed_m4`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 64*1024 {
			t.Skip()
		}

		first, firstErr := parseMissionData([]byte(input))
		second, secondErr := parseMissionData([]byte(input))
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("parse result is non-deterministic: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr != nil {
			return
		}
		if !reflect.DeepEqual(first.Campaign, second.Campaign) {
			t.Fatalf("campaign is non-deterministic: first=%+v second=%+v", first.Campaign, second.Campaign)
		}

		seen := make(map[string]bool, len(first.Campaign.Chapters))
		for _, chapter := range first.Campaign.Chapters {
			if chapter == nil || chapter.Code == "" {
				t.Fatalf("invalid recovered chapter: %+v", chapter)
			}
			if seen[chapter.Code] {
				t.Fatalf("duplicate recovered chapter code %q", chapter.Code)
			}
			seen[chapter.Code] = true
		}
	})
}

func hasRecoveryIssue(issues []missionRecoveryIssue, kind string) bool {
	for _, issue := range issues {
		if issue.Kind == kind {
			return true
		}
	}
	return false
}

func filepathJoinForTest(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}
