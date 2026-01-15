package controller

import (
	"time"

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
