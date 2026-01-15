package middlewares

import (
	"fmt"
	"net/http"
	"os"
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

func Auth(privateKey []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

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

		password := c.PostForm("password")
		if password == "" {
			password = c.Query("password")
		}
		realPassword := os.Getenv("L4D2_MANAGER_PASSWORD")
		if realPassword == "" {
			realPassword = "laoyutangnb"
		}

		success := false
		role := ""
		if password == realPassword {
			success = true
			role = "admin"
			c.Set("privateKey", privateKey)
		} else if parsedToken, err := jwt.Parse(password, getKeyfunc(privateKey)); err == nil && parsedToken.Valid {
			success = true
			role = "guest"
		}

		if success {
			mutex.Lock()
			delete(ipAttempts, ip)
			mutex.Unlock()
			c.Set("role", role)
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

func getKeyfunc(privateKey []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		// 校验签名方法是否匹配
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// 返回密钥
		return privateKey, nil
	}
}
