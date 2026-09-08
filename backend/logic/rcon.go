package logic

import (
	"fmt"
	"os"

	"github.com/gorcon/rcon"
)

var executePluginRconCommand = ExecuteRconCommand

type rconSession interface {
	Execute(string) (string, error)
	Close() error
}

func openRconSession() (rconSession, error) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		return nil, fmt.Errorf("服务端未配置RCON链接")
	}
	password := os.Getenv("L4D2_RCON_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("服务端未配置RCON密码")
	}

	conn, err := rcon.Dial(url, password)
	if err != nil {
		return nil, fmt.Errorf("RCON连接失败: %v", err)
	}
	return conn, nil
}

func ExecuteRconCommand(cmd string) (string, error) {
	conn, err := openRconSession()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	res, err := conn.Execute(cmd)
	if err != nil {
		return "", fmt.Errorf("RCON命令执行失败: %v", err)
	}
	return res, nil
}
