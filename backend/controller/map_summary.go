package controller

import (
	"l4d2-manager-next/logic"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetMapSummaries(c *gin.Context) {
	var req struct {
		Maps []string `json:"maps"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithError(c, http.StatusBadRequest, "参数错误")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": logic.GetMapSummaries(req.Maps),
	})
}
