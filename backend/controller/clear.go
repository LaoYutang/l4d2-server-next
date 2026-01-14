package controller

import (
	"l4d2-manager-next/consts"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func Clear(c *gin.Context) {
	mutex.Lock()
	defer mutex.Unlock()

	fileBytes, err := os.ReadFile(consts.MapListFilePath)
	if err != nil {
		c.String(http.StatusInternalServerError, "获取地图记录文件失败")
		return
	}

	fileList := strings.Split(string(fileBytes), "\n")
	errFileList := []string{}
	for _, file := range fileList {
		if len(file) == 0 {
			continue
		}
		if err := os.Remove(filepath.Join(consts.BasePath, file)); err != nil {
			errFileList = append(errFileList, file)
		}
	}

	if len(errFileList) > 0 {
		os.WriteFile(consts.MapListFilePath, []byte(strings.Join(errFileList, "\n")+"\n"), 0666)
		c.String(http.StatusInternalServerError, "以下文件删除失败："+strings.Join(errFileList, ","))
		return
	}

	if err := os.WriteFile(consts.MapListFilePath, []byte{}, 0666); err != nil {
		c.String(http.StatusInternalServerError, "清空记录文件失败")
		return
	}

	c.String(http.StatusOK, "清空成功！")
}
