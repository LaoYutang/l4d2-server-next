package controller

import (
	"fmt"
	"time"

	"l4d2-manager-next/logic"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func Auth(c *gin.Context) {
	// 中间件已经验证密码
	role, _ := c.Get("role")
	c.JSON(200, gin.H{
		"status": "ok",
		"role":   role,
	})
}

func GetTempAuthCode(c *gin.Context) {
	privateKey, exist := c.Get("privateKey")
	if !exist {
		c.String(400, "请使用密码生成授权码")
		return
	}

	expired := 1 // 默认1小时
	if c.PostForm("expired") != "" {
		ex, err := convertor.ToInt(c.PostForm("expired"))
		if err == nil {
			expired = int(ex)
		}
	}
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expired) * time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		c.String(500, "生成授权码失败: %v", err)
		return
	}
	c.String(200, tokenString)
}

func GetSelfServiceStatus(c *gin.Context) {
	config := logic.GetSelfServiceConfig()
	inCooldown := false
	remaining := 0

	if config.EnableSelfService {
		elapsed := time.Since(config.LastSelfServiceTime)
		if elapsed < time.Hour {
			inCooldown = true
			remaining = int((time.Hour - elapsed).Seconds())
		}
	}

	c.JSON(200, gin.H{
		"enabled":             config.EnableSelfService,
		"in_cooldown":         inCooldown,
		"remaining_seconds":   remaining,
		"last_generated_time": config.LastSelfServiceTime,
	})
}

func GenerateSelfServiceCode(c *gin.Context) {
	config := logic.GetSelfServiceConfig()

	if !config.EnableSelfService {
		c.JSON(403, gin.H{"error": "自助授权功能未开启"})
		return
	}

	elapsed := time.Since(config.LastSelfServiceTime)
	if elapsed < time.Hour {
		c.JSON(429, gin.H{
			"error":     "系统冷却中",
			"remaining": int((time.Hour - elapsed).Seconds()),
		})
		return
	}

	// 从中间件获取 privateKey (需要确保中间件已设置，即使是guest或无auth路径也需注入key，或者从全局获取)
	// 注意：此接口是公开的，没有经过Auth中间件验证密码，但需要 privateKey 来签名
	// 我们可以从 gin.Context 中获取，如果是在 main.go 中通过中间件注入的
	// 或者这里直接读取 key (不太好，最好通过 context 传递)
	// 假设 main.go 中 public 路由也使用了类似 Auth 的中间件但不强制验证，或者我们需要单独处理 key
	// 暂时假设 key 通过某种方式传递，或者我们在这里重新读取 key (不推荐)
	// 更好的方式：在 main.go 中注册路由时，确保有一个中间件注入了 privateKey 但不拦截请求

	privateKey, exist := c.Get("privateKey")
	if !exist {
		// 如果没有 privateKey，尝试从文件读取或者报错
		// 这里为了简单，假设 main.go 会调整以注入 key
		c.JSON(500, gin.H{"error": "系统配置错误: 密钥缺失"})
		return
	}

	// 生成 1 小时有效期的 token
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(privateKey) // privateKey 应该是 []byte
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("生成授权码失败: %v", err)})
		return
	}

	if err := logic.UpdateLastSelfServiceTime(); err != nil {
		c.JSON(500, gin.H{"error": "保存状态失败"})
		return
	}

	c.JSON(200, gin.H{
		"code": tokenString,
	})
}

func SetSelfServiceConfig(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(403, gin.H{"error": "需要管理员权限"})
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}

	if err := logic.SetSelfServiceEnable(req.Enable); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "ok"})
}
