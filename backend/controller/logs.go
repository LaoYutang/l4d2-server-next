package controller

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"l4d2-manager-next/consts"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/axgle/mahonia"
	"github.com/gin-gonic/gin"
)

type LogFileInfo struct {
	Name string `json:"name"`
	Date string `json:"date"`
	Size int64  `json:"size"`
}

type LogListResponse struct {
	Installed  bool                     `json:"installed"`
	Message    string                   `json:"message,omitempty"`
	Categories map[string][]LogFileInfo `json:"categories,omitempty"`
}

var (
	lLogPattern     = regexp.MustCompile(`^L(\d{8})\.log$`)
	errorLogPattern = regexp.MustCompile(`^errors_(\d{8})\.log$`)
)

type tempLogFile struct {
	LogFileInfo
	modTime time.Time
}

func decodeLogLine(line []byte) string {
	line = bytes.TrimRight(line, "\r")
	if utf8.Valid(line) {
		return string(line)
	}
	decoder := mahonia.NewDecoder("gbk")
	return decoder.ConvertString(string(line))
}

func sendSSELine(c *gin.Context, line string, flusher http.Flusher) {
	data, _ := json.Marshal(map[string]string{"line": line})
	fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
	flusher.Flush()
}

func ListSourceModLogs(c *gin.Context) {
	sourceModPath := filepath.Join(consts.GamePath, "addons", "sourcemod")
	if _, err := os.Stat(sourceModPath); os.IsNotExist(err) {
		c.JSON(http.StatusOK, LogListResponse{
			Installed: false,
			Message:   "请安装 SourceMod",
		})
		return
	}

	logsDir := filepath.Join(sourceModPath, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, LogListResponse{
				Installed:  true,
				Categories: map[string][]LogFileInfo{"L": {}, "errors": {}, "other": {}},
			})
			return
		}
		FailWithError(c, http.StatusInternalServerError, "读取日志目录失败: %v", err)
		return
	}

	var lFiles, errorFiles, otherFiles []tempLogFile

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		temp := tempLogFile{
			LogFileInfo: LogFileInfo{
				Name: name,
				Size: info.Size(),
			},
			modTime: info.ModTime(),
		}

		if matches := lLogPattern.FindStringSubmatch(name); matches != nil {
			temp.Date = matches[1]
			lFiles = append(lFiles, temp)
		} else if matches := errorLogPattern.FindStringSubmatch(name); matches != nil {
			temp.Date = matches[1]
			errorFiles = append(errorFiles, temp)
		} else {
			temp.Date = info.ModTime().Format("20060102")
			otherFiles = append(otherFiles, temp)
		}
	}

	// Sort L and errors by date descending
	sort.Slice(lFiles, func(i, j int) bool {
		return lFiles[i].Date > lFiles[j].Date
	})
	sort.Slice(errorFiles, func(i, j int) bool {
		return errorFiles[i].Date > errorFiles[j].Date
	})
	// Sort other by modTime descending
	sort.Slice(otherFiles, func(i, j int) bool {
		return otherFiles[i].modTime.After(otherFiles[j].modTime)
	})

	categories := map[string][]LogFileInfo{
		"L":      {},
		"errors": {},
		"other":  {},
	}
	for _, f := range lFiles {
		categories["L"] = append(categories["L"], f.LogFileInfo)
	}
	for _, f := range errorFiles {
		categories["errors"] = append(categories["errors"], f.LogFileInfo)
	}
	for _, f := range otherFiles {
		categories["other"] = append(categories["other"], f.LogFileInfo)
	}

	c.JSON(http.StatusOK, LogListResponse{
		Installed:  true,
		Categories: categories,
	})
}

func StreamSourceModLog(c *gin.Context) {
	filename := c.Query("file")

	// Security: reject path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		c.String(http.StatusBadRequest, "invalid filename")
		return
	}

	// Security: only allow .log files
	if !strings.HasSuffix(filename, ".log") {
		c.String(http.StatusBadRequest, "only .log files allowed")
		return
	}

	path := filepath.Join(consts.GamePath, "addons", "sourcemod", "logs", filename)

	file, err := os.Open(path)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "streaming not supported")
		return
	}

	const maxHistory = 200 * 1024

	fileInfo, err := file.Stat()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Send recent history (last 200KB), starting from a line boundary
	if fileInfo.Size() > maxHistory {
		startPos := fileInfo.Size() - maxHistory
		file.Seek(startPos, io.SeekStart)

		// Discard until we hit a newline to start at a clean line boundary
		buf := make([]byte, 1)
		for {
			n, err := file.Read(buf)
			if n > 0 && buf[0] == '\n' {
				break
			}
			if err != nil {
				file.Seek(0, io.SeekStart)
				break
			}
		}
	}

	scanner := bufio.NewScanner(file)
	// Increase buffer to handle very long lines
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var historyLines []string
	for scanner.Scan() {
		historyLines = append(historyLines, scanner.Text())
	}

	// Only send last 200 lines to avoid overwhelming the frontend
	if len(historyLines) > 200 {
		historyLines = historyLines[len(historyLines)-200:]
	}
	for _, line := range historyLines {
		sendSSELine(c, decodeLogLine([]byte(line)), flusher)
	}

	// Now seek to end and watch for new content
	file.Seek(0, io.SeekEnd)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			info, err := file.Stat()
			if err != nil {
				continue
			}

			currentPos, err := file.Seek(0, io.SeekCurrent)
			if err != nil {
				continue
			}

			// Handle file truncation (e.g. log rotation)
			if info.Size() < currentPos {
				file.Seek(0, io.SeekStart)
				currentPos = 0
			}

			if info.Size() <= currentPos {
				continue // No new content
			}

			// Create a new scanner from current position to read new lines.
			// Scanner cannot resume after EOF, so we recreate it each tick.
			s := bufio.NewScanner(file)
			s.Buffer(make([]byte, maxCapacity), maxCapacity)
			for s.Scan() {
				line := s.Text()
				sendSSELine(c, decodeLogLine([]byte(line)), flusher)
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}
