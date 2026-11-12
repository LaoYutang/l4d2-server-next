package controller

import (
	"l4d2-manager-next/logic"

	"github.com/gin-gonic/gin"
)

func Clear(c *gin.Context) {
	defer LogOp(c, "清空未锁定地图文件")()
	result, err := logic.ClearUnlockedMaps()
	respondMapDeleteResult(c, result, err)
}
