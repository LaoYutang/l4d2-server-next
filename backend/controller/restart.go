package controller

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/gin-gonic/gin"
)

func Restart(c *gin.Context) {
	defer LogOp(c, "执行重启操作")()
	// 使用RCON重启
	if os.Getenv("L4D2_RESTART_BY_RCON") == "true" {
		if err := restartByRcon(); err != nil {
			FailWithError(c, http.StatusInternalServerError, "%s", err.Error())
		}
		return
	}

	// 使用命令重启
	restartCmd := os.Getenv("L4D2_RESTART_CMD")
	if restartCmd == "" {
		containerName := os.Getenv("L4D2_CONTAINER_NAME")
		if containerName == "" {
			containerName = "l4d2"
		}
		restartCmd = "docker restart " + containerName
	}

	var err error
	if runtime.GOOS == "windows" {
		err = exec.Command("cmd.exe", "/c", restartCmd).Run()
	} else {
		err = exec.Command("sh", "-c", restartCmd).Run()
	}
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "重启失败: %v", err)
		return
	}

	c.String(http.StatusOK, "重启成功，请等待服务器启动")
}

func restartByRcon() error {
	conn, err := getRconConnection()
	if err != nil {
		return err
	}

	defer conn.Close()

	_, err = conn.Execute("_restart")
	if err != nil {
		return fmt.Errorf("RCON命令执行失败: %v", err)
	}

	return nil
}
