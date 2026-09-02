package middlewares

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
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

type TempAuthClaims struct {
	MapUploadOnly bool `json:"map_upload_only,omitempty"`
	jwt.RegisteredClaims
}

var mapUploaderAllowedRequests = map[string]struct{}{
	http.MethodPost + " /auth":                   {},
	http.MethodPost + " /upload/init":            {},
	http.MethodPost + " /upload/chunk":           {},
	http.MethodPost + " /upload/status":          {},
	http.MethodPost + " /upload/merge":           {},
	http.MethodPost + " /upload/cancel":          {},
	http.MethodPost + " /maps/hot-reload":        {},
	http.MethodPost + " /maps/hot-reload/status": {},
}

func Auth(privateKey []byte) gin.HandlerFunc {
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
		realPassword := os.Getenv("L4D2_MANAGER_PASSWORD")
		if realPassword == "" {
			realPassword = "laoyutangnb"
		}

		success := false
		role := ""
		if credential == realPassword {
			success = true
			role = RoleAdmin
			c.Set("privateKey", privateKey)
		} else {
			claims := &TempAuthClaims{}
			parsedToken, err := jwt.ParseWithClaims(
				credential,
				claims,
				getKeyfunc(privateKey),
				jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			)
			if err == nil && parsedToken.Valid {
				success = true
				role = RoleGuest
				if claims.MapUploadOnly {
					role = RoleMapUploader
				}
			}
		}

		if success {
			mutex.Lock()
			delete(ipAttempts, ip)
			mutex.Unlock()
			c.Set("role", role)
			if role == RoleMapUploader && !isMapUploaderRequestAllowed(c.Request.Method, c.Request.URL.Path) {
				c.String(http.StatusForbidden, "该授权码仅允许上传和热重载地图")
				c.Abort()
				return
			}
			c.Next()
		} else {
			mutex.Lock()
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

			c.String(http.StatusUnauthorized, "密码错误或令牌已失效")
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

func getKeyfunc(privateKey []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// 返回密钥
		return privateKey, nil
	}
}
