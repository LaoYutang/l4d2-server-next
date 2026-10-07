package middlewares

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"l4d2-manager-next/logic"

	"github.com/gin-gonic/gin"
)

type loginAttempt struct {
	count     int
	firstTime time.Time
	lockUntil time.Time
}

var (
	ipAttempts = make(map[string]*loginAttempt)
	mutex      sync.Mutex
)

const bearerPrefix = "Bearer "

const (
	RoleAdmin       = "admin"
	RoleGuest       = "guest"
	RoleMapUploader = "map_uploader"
)

var mapUploaderAllowedRequests = map[string]struct{}{
	http.MethodPost + " /auth":                   {},
	http.MethodPost + " /upload/init":            {},
	http.MethodPost + " /upload/chunk":           {},
	http.MethodPost + " /upload/status":          {},
	http.MethodPost + " /upload/merge":           {},
	http.MethodPost + " /upload/cancel":          {},
	http.MethodPost + " /maps/hot-reload":        {},
	http.MethodPost + " /maps/hot-reload/status": {},
	http.MethodPost + " /download/add":           {},
	http.MethodPost + " /download/list":          {},
	http.MethodPost + " /download/cancel":        {},
	http.MethodPost + " /download/restart":       {},
	http.MethodPost + " /download/clear":         {},
	http.MethodPost + " /download/link/parse":    {},
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := GetClientIP(c)

		mutex.Lock()
		attempt, exists := ipAttempts[ip]
		if !exists {
			attempt = &loginAttempt{}
			ipAttempts[ip] = attempt
		}

		if time.Now().Before(attempt.lockUntil) {
			mutex.Unlock()
			c.String(http.StatusTooManyRequests, "尝试次数过多，请稍后重试")
			c.Abort()
			return
		}
		mutex.Unlock()

		credential := getBearerCredential(c.GetHeader("Authorization"))
		realPassword := logic.AdministratorPassword()

		success := false
		role := ""
		if credential == realPassword {
			success = true
			role = RoleAdmin
		} else {
			record, err := logic.GetAuthCodeStore().Authenticate(credential)
			if errors.Is(err, logic.ErrAuthUnavailable) {
				c.String(http.StatusServiceUnavailable, "授权服务暂不可用")
				c.Abort()
				return
			}
			if err == nil {
				success = true
				role = RoleGuest
				if record.AccessType == logic.AuthAccessMapUpload {
					role = RoleMapUploader
				}
				c.Set("auth_code_id", record.ID)
				c.Set("auth_code_remark", record.Remark)
				c.Set("auth_expires_at", record.ExpiresAt)
			}
		}

		if success {
			mutex.Lock()
			delete(ipAttempts, ip)
			mutex.Unlock()
			c.Set("role", role)
			if role == RoleMapUploader && !isMapUploaderRequestAllowed(c.Request.Method, c.Request.URL.Path) {
				c.String(http.StatusForbidden, "该授权码仅允许上传、下载和热重载地图")
				c.Abort()
				return
			}
			c.Next()
		} else {
			mutex.Lock()
			attempt = ipAttempts[ip]
			if attempt == nil {
				attempt = &loginAttempt{}
				ipAttempts[ip] = attempt
			}
			now := time.Now()
			// 如果是第一次错误或者距离第一次错误已经超过1分钟，重置计数
			if attempt.count == 0 || now.Sub(attempt.firstTime) > time.Minute {
				attempt.count = 1
				attempt.firstTime = now
			} else {
				attempt.count++
			}

			if attempt.count > 10 {
				attempt.lockUntil = now.Add(10 * time.Minute)
				mutex.Unlock()
				c.String(http.StatusTooManyRequests, "错误次数过多，IP已被锁定")
				c.Abort()
				return
			}
			mutex.Unlock()

			c.String(http.StatusUnauthorized, "密码错误或授权码已失效")
			c.Abort()
		}
	}
}

func isMapUploaderRequestAllowed(method, path string) bool {
	_, ok := mapUploaderAllowedRequests[method+" "+path]
	return ok
}

func getBearerCredential(header string) string {
	if len(header) < len(bearerPrefix) {
		return ""
	}
	if !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(header[len(bearerPrefix):])
}

// RevalidateAuth is also used by long-lived streams and before installing an upload.
func RevalidateAuth(c *gin.Context) error {
	if c.GetString("role") == RoleAdmin {
		return nil
	}
	_, err := logic.GetAuthCodeStore().ActiveByID(c.GetString("auth_code_id"))
	return err
}
