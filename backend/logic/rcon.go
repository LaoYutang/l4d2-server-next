package logic

import (
	"fmt"
	"os"

	"github.com/gorcon/rcon"
)

var executePluginRconCommand = ExecuteRconCommand

func ExecuteRconCommand(cmd string) (string, error) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		return "", fmt.Errorf("服务端未配置RCON链接")
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		return "", fmt.Errorf("服务端未配置RCON密码")
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		return "", fmt.Errorf("RCON连接失败: %v", err)
	}
	defer conn.Close()

	res, err := conn.Execute(cmd)
	if err != nil {
		return "", fmt.Errorf("RCON命令执行失败: %v", err)
	}
	return res, nil
}
