package logic

import (
	"bufio"
	"fmt"
	"os"
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

// Regex to match "key" "value"
var cvarRegex = regexp.MustCompile(`^"?([a-zA-Z0-9_]+)"?\s+"?([^"]*)"?`)

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
