package middlewares

import (
	"errors"
	"fmt"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/utility"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	clientIPContextKey     = "resolvedClientIP"
	clientIPInfoContextKey = "resolvedClientIPInfo"
)

// AccessControl resolves the request IP once and enforces the current panel policy.
func AccessControl() gin.HandlerFunc {
	return func(c *gin.Context) {
		snapshot := logic.CurrentAccessControlSnapshot()
		info, err := snapshot.ResolveClientIP(ClientIPInput(c))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_client_ip",
				"message": err.Error(),
			})
			return
		}

		c.Set(clientIPContextKey, info.ClientIP)
		c.Set(clientIPInfoContextKey, info)

		decision, err := snapshot.Evaluate(info.ClientIP, utility.LookupRawRegion)
		if errors.Is(err, logic.ErrAccessControlRecoveryMode) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":   "access_control_recovery_mode",
				"message": "访问控制配置损坏，仅允许本机回环地址访问",
			})
			return
		}
		if errors.Is(err, logic.ErrAccessControlGeoIPUnavailable) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":   "geoip_unavailable",
				"message": "GeoIP 服务不可用，无法安全执行地区访问规则",
			})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "access_control_error",
				"message": err.Error(),
			})
			return
		}
		if !decision.Allowed {
			fmt.Printf("[ACCESS] blocked IP %s: %s\n", info.ClientIP, decision.Reason)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "access_denied",
				"message": "当前 IP 不允许访问管理面板",
			})
			return
		}

		c.Next()
	}
}

func ClientIPInput(c *gin.Context) logic.ClientIPInput {
	return logic.ClientIPInput{
		RemoteAddr:    c.Request.RemoteAddr,
		XForwardedFor: c.GetHeader("X-Forwarded-For"),
		XRealIP:       c.GetHeader("X-Real-IP"),
	}
}

// GetClientIP returns the canonical IP resolved by AccessControl. Tests and
// isolated handlers fall back to the direct TCP peer without trusting headers.
func GetClientIP(c *gin.Context) string {
	if value, exists := c.Get(clientIPContextKey); exists {
		if ip, ok := value.(string); ok && ip != "" {
			return ip
		}
	}
	return directPeerIP(c.Request.RemoteAddr)
}

func GetClientIPInfo(c *gin.Context) logic.ClientIPInfo {
	if value, exists := c.Get(clientIPInfoContextKey); exists {
		if info, ok := value.(logic.ClientIPInfo); ok {
			return info
		}
	}
	ip := directPeerIP(c.Request.RemoteAddr)
	return logic.ClientIPInfo{PeerIP: ip, ClientIP: ip, Source: "remote_addr"}
}

func directPeerIP(remoteAddress string) string {
	value := strings.TrimSpace(remoteAddress)
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(value, "[]")
}
