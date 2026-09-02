package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"
	"l4d2-manager-next/model"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxAuditDetailLength = 2000

var enqueueAuditLog = logic.EnqueueAuditLog

// LogOp captures an operation and returns a deferred finalizer. The finalizer
// prints the operation immediately and enqueues the database write without
// waiting for SQLite.
// Format: [OPT] Time | Role | IP | Path | SUCCESS/FAILED | Detail
func LogOp(c *gin.Context, detail string) func() {
	startedAt := time.Now()
	ip := middlewares.GetClientIP(c)
	path := c.Request.URL.Path

	roleVal, exists := c.Get("role")
	role := middlewares.RoleGuest
	if exists {
		if r, ok := roleVal.(string); ok && (r == middlewares.RoleAdmin || r == middlewares.RoleGuest || r == middlewares.RoleMapUploader) {
			role = r
		}
	}

	detail = sanitizeAuditDetail(detail)
	return func() {
		panicValue := recover()
		success := panicValue == nil && c.Writer.Status() >= 200 && c.Writer.Status() < 300
		result := "FAILED"
		if success {
			result = "SUCCESS"
		}

		fmt.Printf(
			"[OPT] %s | %s | %s | %s | %s | %s\n",
			startedAt.Format("2006/01/02 - 15:04:05"),
			role,
			ip,
			path,
			result,
			detail,
		)
		enqueueAuditLog(model.AuditLog{
			Time:    startedAt.Unix(),
			Role:    role,
			IP:      ip,
			Path:    path,
			Success: success,
			Detail:  detail,
		})

		if panicValue != nil {
			panic(panicValue)
		}
	}
}

func sanitizeAuditDetail(detail string) string {
	detail = strings.Join(strings.Fields(detail), " ")
	runes := []rune(detail)
	if len(runes) > maxAuditDetailLength {
		return string(runes[:maxAuditDetailLength]) + "…"
	}
	return detail
}

// LogError prints an error log.
func LogError(c *gin.Context, args ...any) {
	msg := fmt.Sprint(args...)
	stack := string(debug.Stack())
	fmt.Printf("[ERR] %s\n%s\n", msg, stack)
}

// FailWithError logs the error and sends a string response.
func FailWithError(c *gin.Context, code int, format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	LogError(c, msg)
	c.String(code, msg)
}
