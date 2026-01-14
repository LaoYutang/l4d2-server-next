package controller

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/gorcon/rcon"
)

func Restart(c *gin.Context) {
	// 使用RCON重启
	if os.Getenv("L4D2_RESTART_BY_RCON") == "true" {
		if err := restartByRcon(); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
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
		fmt.Println("重启失败:", err)
		c.String(http.StatusInternalServerError, "重启失败")
		return
	}

	c.String(http.StatusOK, "重启成功，请等待服务器启动")
}

func restartByRcon() error {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		return fmt.Errorf("服务端未配置RCON链接")
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		return fmt.Errorf("服务端未配置RCON密码")
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		return fmt.Errorf("RCON连接失败: %v", err)
	}

	defer conn.Close()

	_, err = conn.Execute("_restart")
	if err != nil {
		return fmt.Errorf("RCON命令执行失败: %v", err)
	}

	return nil
}
