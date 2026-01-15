package controller

import (
	"l4d2-manager-next/consts"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": consts.Version})
}
