package controller

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"l4d2-manager-next/logic"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/axgle/mahonia"
	"github.com/gin-gonic/gin"
)

type LogListResponse struct {
	Installed  bool                                `json:"installed"`
	Message    string                              `json:"message,omitempty"`
	Categories map[string][]logic.SourceModLogFile `json:"categories,omitempty"`
}

type sourceModLogDeleteRequest struct {
	Files []logic.SourceModLogDeleteTarget `json:"files"`
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
	scan, err := logic.ScanSourceModLogs(time.Now())
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "读取日志目录失败: %v", err)
		return
	}
	if !scan.Installed {
		c.JSON(http.StatusOK, LogListResponse{
			Installed: false,
			Message:   "请安装 SourceMod",
		})
		return
	}

	categories := map[string][]logic.SourceModLogFile{
		"L":      {},
		"errors": {},
		"other":  {},
	}
	for _, file := range scan.Files {
		categories[file.Category] = append(categories[file.Category], file)
	}

	c.JSON(http.StatusOK, LogListResponse{
		Installed:  true,
		Categories: categories,
	})
}

func PreviewSourceModLogCleanup(c *gin.Context) {
	if !requireSourceModLogAdmin(c) {
		return
	}

	var filter logic.SourceModLogCleanupFilter
	if err := c.ShouldBindJSON(&filter); err != nil {
		c.String(http.StatusBadRequest, "请求参数无效")
		return
	}
	preview, err := logic.PreviewSourceModLogCleanup(time.Now(), filter)
	if err != nil {
		if errors.Is(err, logic.ErrInvalidSourceModLogCleanupFilter) {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		FailWithError(c, http.StatusInternalServerError, "预览日志清理失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

func DeleteSourceModLogs(c *gin.Context) {
	if !requireSourceModLogAdmin(c) {
		return
	}

	var request sourceModLogDeleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.String(http.StatusBadRequest, "请求参数无效")
		return
	}
	defer LogOp(c, sourceModLogDeleteAuditDetail(request.Files))()

	result, err := logic.DeleteSourceModLogs(time.Now(), request.Files)
	if err != nil {
		if errors.Is(err, logic.ErrInvalidSourceModLogName) {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		FailWithError(c, http.StatusInternalServerError, "删除日志失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func requireSourceModLogAdmin(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role != "admin" {
		c.String(http.StatusForbidden, "仅管理员可以管理日志文件")
		return false
	}
	return true
}

func sourceModLogDeleteAuditDetail(files []logic.SourceModLogDeleteTarget) string {
	const maxNames = 10
	names := make([]string, 0, min(len(files), maxNames))
	for i, file := range files {
		if i >= maxNames {
			break
		}
		names = append(names, file.Name)
	}
	detail := fmt.Sprintf("删除 SourceMod 日志文件，共 %d 个", len(files))
	if len(names) > 0 {
		detail += ": " + strings.Join(names, ", ")
	}
	if len(files) > maxNames {
		detail += fmt.Sprintf(" 等 %d 个", len(files))
	}
	return detail
}

func StreamSourceModLog(c *gin.Context) {
	filename := c.Query("file")

	if err := logic.ValidateSourceModLogName(filename); err != nil {
		c.String(http.StatusBadRequest, "invalid log filename")
		return
	}

	root, err := os.OpenRoot(logic.SourceModLogsDir())
	if err != nil {
		c.String(http.StatusInternalServerError, "open log directory: %v", err)
		return
	}
	defer root.Close()
	info, err := root.Lstat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			c.String(http.StatusNotFound, "log file not found")
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if !info.Mode().IsRegular() {
		c.String(http.StatusBadRequest, "only regular log files are allowed")
		return
	}

	file, err := root.Open(filename)
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
	if err := scanner.Err(); err != nil {
		sendSSELine(c, fmt.Sprintf("读取日志失败: %v", err), flusher)
		return
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
			if err := s.Err(); err != nil {
				sendSSELine(c, fmt.Sprintf("读取日志失败: %v", err), flusher)
				continue
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}
