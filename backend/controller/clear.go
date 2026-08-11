package controller

import (
	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func Clear(c *gin.Context) {
	defer LogOp(c, "清空所有地图文件")()
	mutex.Lock()
	defer mutex.Unlock()

	fileBytes, err := os.ReadFile(consts.MapListFilePath)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "获取地图记录文件失败: %v", err)
		return
	}

	fileList := strings.Split(string(fileBytes), "\n")
	errFileList := []string{}
	for _, file := range fileList {
		if len(file) == 0 {
			continue
		}
		if err := os.Remove(filepath.Join(consts.AddonsBasePath, file)); err != nil && !os.IsNotExist(err) {
			errFileList = append(errFileList, file)
		}
	}

	if len(errFileList) > 0 {
		if err := os.WriteFile(consts.MapListFilePath, []byte(strings.Join(errFileList, "\n")+"\n"), 0666); err != nil {
			FailWithError(c, http.StatusInternalServerError, "写入剩余地图记录失败: %v", err)
			return
		}
		if err := logic.RetainMapVPKInspections(errFileList); err != nil {
			log.Printf("更新地图 VPK 检测记录失败: %v", err)
		}
		FailWithError(c, http.StatusInternalServerError, "以下文件删除失败：%s", strings.Join(errFileList, ","))
		return
	}

	if err := os.WriteFile(consts.MapListFilePath, []byte{}, 0666); err != nil {
		FailWithError(c, http.StatusInternalServerError, "清空记录文件失败: %v", err)
		return
	}
	if err := logic.RetainMapVPKInspections(nil); err != nil {
		log.Printf("清空地图 VPK 检测记录失败: %v", err)
	}

	c.String(http.StatusOK, "清空成功！")
}
