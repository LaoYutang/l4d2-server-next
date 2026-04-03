package logic

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type CvarConfig struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Default     string `json:"default"`
	Min         string `json:"min"`
	Max         string `json:"max"`
	Description string `json:"description"`
}

type PluginConfigFile struct {
	FileName string       `json:"file_name"`
	Cvars    []CvarConfig `json:"cvars"`
}

// Regex to match "key" "value" or key "value" or key value (SourceMod cfg format)
var cvarRegex = regexp.MustCompile(`^"?([a-zA-Z0-9_]+)"?\s+"?([^"]*)"?`)

// Console command names that should never be treated as cvar names.
// These are SourceMod/Source engine commands that appear in script cfg files
// (e.g. sm_warmode_*.cfg) but are not cvar definitions.
var consoleCmdNames = map[string]bool{
	"sm":   true,
	"exec": true,
	"meta": true,
	"rcon": true,
}

// isConsoleCmdName returns true if the extracted cvar name is actually a console command.
func isConsoleCmdName(name string) bool {
	return consoleCmdNames[strings.ToLower(name)]
}

// Regex to extract meta from comments
var defaultRegex = regexp.MustCompile(`(?i)^\s*//\s*Default:\s*"(.*)"`)
var minRegex = regexp.MustCompile(`(?i)^\s*//\s*Minimum:\s*"(.*)"`)
var maxRegex = regexp.MustCompile(`(?i)^\s*//\s*Maximum:\s*"(.*)"`)

func ParseSourceModConfig(path string) ([]CvarConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cvars []CvarConfig
	scanner := bufio.NewScanner(file)

	var commentBuffer []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "//") {
			commentBuffer = append(commentBuffer, line)
			continue
		}

		// Check if it's a cvar
		matches := cvarRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			name := matches[1]
			value := matches[2]

			// Skip console commands that look like cvars due to the permissive regex
			if isConsoleCmdName(name) {
				commentBuffer = []string{}
				continue
			}
			// Parse metadata from comments
			config := CvarConfig{
				Name:  name,
				Value: value,
			}

			var descLines []string

			for _, comment := range commentBuffer {
				// Remove // prefix
				cleanComment := strings.TrimSpace(strings.TrimPrefix(comment, "//"))

				if match := defaultRegex.FindStringSubmatch(comment); len(match) > 1 {
					config.Default = match[1]
				} else if match := minRegex.FindStringSubmatch(comment); len(match) > 1 {
					config.Min = match[1]
				} else if match := maxRegex.FindStringSubmatch(comment); len(match) > 1 {
					config.Max = match[1]
				} else if cleanComment == "-" {
					// separator, ignore
				} else {
					// Assume description
					// Skip lines that look like file headers
					if strings.Contains(cleanComment, "This file was auto-generated") ||
						strings.Contains(cleanComment, "ConVars for plugin") {
						continue
					}
					descLines = append(descLines, cleanComment)
				}
			}

			config.Description = strings.Join(descLines, "\n")
			cvars = append(cvars, config)

			// Reset buffer
			commentBuffer = []string{}
		} else {
			// Not a cvar, maybe a section header or garbage, clear buffer
			// Actually SM configs are usually just comments and cvars.
			// If we hit something else, just clear buffer to be safe.
			commentBuffer = []string{}
		}
	}

	return cvars, scanner.Err()
}

func UpdateSourceModConfig(path string, updates map[string]string) error {
	// Read entire file
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// If line is a cvar definition
		matches := cvarRegex.FindStringSubmatch(trimmed)
		if len(matches) == 3 && !strings.HasPrefix(trimmed, "//") {
			name := matches[1]
			// Check if we have an update for this cvar
			if newValue, ok := updates[name]; ok {
				// Reconstruct the line preserving indentation if possible?
				// Simple approach: name "value"
				// To preserve formatting, we can try to replace just the value part
				// But regex replacement is safer to ensure correct syntax

				// Using standard format: name "value"
				newLines = append(newLines, fmt.Sprintf(`%s "%s"`, name, newValue))
			} else {
				newLines = append(newLines, line)
			}
		} else {
			newLines = append(newLines, line)
		}
	}

	output := strings.Join(newLines, "\n")
	return os.WriteFile(path, []byte(output), 0644)
}

func UpdateOrCreateSourceModConfig(path string, updates map[string]string) error {
	// Read file if exists
	var lines []string
	if _, err := os.Stat(path); err == nil {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines = strings.Split(string(content), "\n")
	}

	var newLines []string
	updatedKeys := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// If line is a cvar definition
		matches := cvarRegex.FindStringSubmatch(trimmed)
		if len(matches) == 3 && !strings.HasPrefix(trimmed, "//") {
			name := matches[1]
			// Check if we have an update for this cvar
			if newValue, ok := updates[name]; ok {
				newLines = append(newLines, fmt.Sprintf(`%s "%s"`, name, newValue))
				updatedKeys[name] = true
			} else {
				newLines = append(newLines, line)
			}
		} else {
			newLines = append(newLines, line)
		}
	}

	// Append missing keys
	for key, value := range updates {
		if !updatedKeys[key] {
			newLines = append(newLines, fmt.Sprintf(`%s "%s"`, key, value))
		}
	}

	output := strings.Join(newLines, "\n")
	return os.WriteFile(path, []byte(output), 0644)
}

// RestoreSourceModConfig restores a cfg file with full metadata.
// If the file exists, only updates values (preserving existing comments).
// If the file doesn't exist, creates it with full comments/metadata.
func RestoreSourceModConfig(path string, cvars []CvarConfig) error {
	if _, err := os.Stat(path); err == nil {
		// File exists: just update values, preserve existing structure
		updates := make(map[string]string, len(cvars))
		for _, c := range cvars {
			updates[c.Name] = c.Value
		}
		return UpdateOrCreateSourceModConfig(path, updates)
	}

	// File doesn't exist: create with full metadata
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	var lines []string
	for i, cvar := range cvars {
		if i > 0 {
			lines = append(lines, "")
		}
		// Write description
		if cvar.Description != "" {
			for _, descLine := range strings.Split(cvar.Description, "\n") {
				lines = append(lines, "// "+descLine)
			}
		}
		// Write metadata
		lines = append(lines, "// -")
		if cvar.Default != "" {
			lines = append(lines, fmt.Sprintf(`// Default: "%s"`, cvar.Default))
		}
		if cvar.Min != "" {
			lines = append(lines, fmt.Sprintf(`// Minimum: "%s"`, cvar.Min))
		}
		if cvar.Max != "" {
			lines = append(lines, fmt.Sprintf(`// Maximum: "%s"`, cvar.Max))
		}
		// Write value
		lines = append(lines, fmt.Sprintf(`%s "%s"`, cvar.Name, cvar.Value))
	}

	output := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(output), 0644)
}
