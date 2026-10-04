package controller

import (
	"errors"
	"fmt"
	"net/http"

	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"

	"github.com/gin-gonic/gin"
)

func authCodeError(c *gin.Context, err error) {
	var validation *logic.AuthValidationError
	var cooldown *logic.AuthCooldownError
	switch {
	case errors.As(err, &validation):
		FailWithError(c, http.StatusBadRequest, "%s", validation.Message)
	case errors.As(err, &cooldown):
		FailWithError(c, http.StatusTooManyRequests, "%s", cooldown.Error())
	case errors.Is(err, logic.ErrAuthDuplicate):
		FailWithError(c, http.StatusConflict, "该授权码已存在")
	case errors.Is(err, logic.ErrAuthNotFound):
		FailWithError(c, http.StatusNotFound, "授权记录不存在")
	case errors.Is(err, logic.ErrAuthInvalid):
		FailWithError(c, http.StatusUnauthorized, "授权码已失效")
	case errors.Is(err, logic.ErrSelfServiceDisabled):
		FailWithError(c, http.StatusForbidden, "自助授权功能未开启")
	default:
		FailWithError(c, http.StatusServiceUnavailable, "授权服务暂不可用")
	}
}

func requireAuthCodeAdmin(c *gin.Context) bool {
	if c.GetString("role") != middlewares.RoleAdmin {
		FailWithError(c, http.StatusForbidden, "需要管理员权限")
		return false
	}
	return true
}

func auditAuthCode(c *gin.Context, record logic.AuthCodeItem) {
	c.Set("audit_auth_code_id", record.ID)
	c.Set("audit_auth_code_remark", record.Remark)
}

func Auth(c *gin.Context) {
	defer LogOp(c, "用户登录，角色: "+c.GetString("role"))()
	response := gin.H{"status": "ok", "role": c.GetString("role")}
	if id := c.GetString("auth_code_id"); id != "" {
		record, err := logic.GetAuthCodeStore().RecordLogin(id, middlewares.GetClientIP(c))
		if err != nil {
			authCodeError(c, err)
			return
		}
		response["expires_at"] = record.ExpiresAt
	}
	c.JSON(http.StatusOK, response)
}

func ListAuthCodes(c *gin.Context) {
	if !requireAuthCodeAdmin(c) {
		return
	}
	var req logic.AuthCodeListFilter
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, 400, "参数错误")
		return
	}
	result, err := logic.GetAuthCodeStore().List(req)
	if err != nil {
		authCodeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func CreateAuthCode(c *gin.Context) {
	defer LogOp(c, "创建授权码")()
	if !requireAuthCodeAdmin(c) {
		return
	}
	var req logic.CreateAuthCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, 400, "参数错误，到期时间应使用 RFC 3339 格式")
		return
	}
	result, err := logic.GetAuthCodeStore().Create(req)
	if err != nil {
		authCodeError(c, err)
		return
	}
	auditAuthCode(c, result.AuthCodeItem)
	c.JSON(http.StatusOK, result)
}

func UpdateAuthCode(c *gin.Context) {
	defer LogOp(c, "更新授权码备注或到期时间")()
	if !requireAuthCodeAdmin(c) {
		return
	}
	var req logic.UpdateAuthCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, 400, "参数错误，到期时间应使用 RFC 3339 格式")
		return
	}
	result, err := logic.GetAuthCodeStore().Update(req)
	if err != nil {
		authCodeError(c, err)
		return
	}
	auditAuthCode(c, result)
	c.JSON(http.StatusOK, result)
}

func mutateAuthCode(c *gin.Context, revoke bool) {
	detail := "删除授权码"
	if revoke {
		detail = "撤销授权码"
	}
	defer LogOp(c, detail)()
	if !requireAuthCodeAdmin(c) {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, 400, "参数错误")
		return
	}
	var result logic.AuthCodeItem
	var err error
	if revoke {
		result, err = logic.GetAuthCodeStore().Revoke(req.ID)
	} else {
		result, err = logic.GetAuthCodeStore().Delete(req.ID)
	}
	if err != nil {
		authCodeError(c, err)
		return
	}
	auditAuthCode(c, result)
	c.JSON(http.StatusOK, result)
}

func RevokeAuthCode(c *gin.Context) { mutateAuthCode(c, true) }
func DeleteAuthCode(c *gin.Context) { mutateAuthCode(c, false) }

func CleanupExpiredAuthCodes(c *gin.Context) {
	detail := "清理过期授权码"
	defer func() { LogOp(c, detail)() }()
	if !requireAuthCodeAdmin(c) {
		return
	}
	count, err := logic.GetAuthCodeStore().CleanupExpired()
	if err != nil {
		authCodeError(c, err)
		return
	}
	detail = fmt.Sprintf("清理过期授权码，删除 %d 条", count)
	c.JSON(http.StatusOK, gin.H{"deleted_count": count})
}

func GetSelfServiceStatus(c *gin.Context) {
	status, err := logic.GetAuthCodeStore().SelfServiceStatus()
	if err != nil {
		authCodeError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func GenerateSelfServiceCode(c *gin.Context) {
	defer LogOp(c, "申请自助授权码")()
	result, err := logic.GetAuthCodeStore().GenerateSelfService()
	if err != nil {
		authCodeError(c, err)
		return
	}
	auditAuthCode(c, result.AuthCodeItem)
	c.JSON(http.StatusOK, result)
}

func SetSelfServiceConfig(c *gin.Context) {
	if !requireAuthCodeAdmin(c) {
		return
	}
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, 400, "参数错误")
		return
	}
	detail := "关闭自助授权配置"
	if req.Enable {
		detail = "开启自助授权配置"
	}
	defer LogOp(c, detail)()
	if err := logic.SetSelfServiceEnable(req.Enable); err != nil {
		FailWithError(c, 500, "保存配置失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
