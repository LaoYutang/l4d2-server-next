package main

import (
	"l4d2-manager-next/consts"
	"l4d2-manager-next/controller"
	"l4d2-manager-next/middlewares"
	"net/http"
	"os"
	"path/filepath"

	"github.com/duke-git/lancet/v2/random"
	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.StaticFS("/", http.Dir("./static"))

	// 如果本地的private.key不存在，创建一个随机HS256密钥
	const privateKeyPath = "./private.key"
	var privateKey []byte
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		privateKey = []byte(random.RandNumeralOrLetter(16))
		err = os.WriteFile(privateKeyPath, privateKey, 0600)
		if err != nil {
			panic("创建private.key失败")
		}
	} else {
		privateKey, err = os.ReadFile(privateKeyPath)
		if err != nil {
			panic("读取private.key失败")
		}
	}

	// 如果maplist.txt不存在，创建一个空的
	mapListPath := filepath.Join(consts.MapListFilePath)
	if _, err := os.Stat(mapListPath); os.IsNotExist(err) {
		err := os.WriteFile(mapListPath, []byte(""), 0755)
		if err != nil {
			panic("创建maplist.txt失败")
		}
	}

	router.MaxMultipartMemory = 1 << 25 // 限制表单内存缓存为32M
	router.POST("/auth", middlewares.Auth(privateKey), controller.Auth)
	router.POST("/auth/getTempAuthCode", middlewares.Auth(privateKey), controller.GetTempAuthCode)
	router.POST("/upload", middlewares.Auth(privateKey), controller.Upload)
	router.POST("/restart", middlewares.Auth(privateKey), controller.Restart)
	router.POST("/clear", middlewares.Auth(privateKey), controller.Clear)
	router.POST("/list", controller.List)
	router.POST("/remove", middlewares.Auth(privateKey), controller.Remove)
	router.POST("/rcon/maplist", middlewares.Auth(privateKey), controller.GetRconMapList)
	router.POST("/rcon/changemap", middlewares.Auth(privateKey), controller.ChangeMap)
	router.POST("/rcon/getstatus", middlewares.Auth(privateKey), controller.GetStatus)
	router.POST("/rcon/kickuser", middlewares.Auth(privateKey), controller.KickUser)
	router.POST("/rcon/banuser", middlewares.Auth(privateKey), controller.BanUser)
	router.POST("/rcon/changedifficulty", middlewares.Auth(privateKey), controller.ChangeDifficulty)
	router.POST("/rcon/changegamemode", middlewares.Auth(privateKey), controller.ChangeGameMode)
	router.POST("/download/add", middlewares.Auth(privateKey), controller.AddDownloadTask)
	router.POST("/download/clear", middlewares.Auth(privateKey), controller.ClearTasks)
	router.POST("/download/list", middlewares.Auth(privateKey), controller.GetDownloadTasksInfo)
	router.POST("/download/cancel", middlewares.Auth(privateKey), controller.CancelDownloadTask)
	router.POST("/download/restart", middlewares.Auth(privateKey), controller.RestartDownloadTask)
	router.POST("/getUserPlaytime", middlewares.Auth(privateKey), controller.GetUserPlaytime)
	router.POST("/monitor/status", middlewares.Auth(privateKey), controller.GetMonitorStatus)
	router.POST("/rcon", middlewares.Auth(privateKey), controller.Rcon)
	router.POST("/server-info/get", middlewares.Auth(privateKey), controller.GetServerInfo)
	router.POST("/server-info/update", middlewares.Auth(privateKey), controller.UpdateServerInfo)
	router.POST("/getVersion", controller.GetVersion)

	plugins := router.Group("/plugins", middlewares.Auth(privateKey))
	{
		plugins.POST("/list", controller.GetPlugins)
		plugins.POST("/upload", controller.UploadPlugin)
		plugins.POST("/enable", controller.EnablePlugin)
		plugins.POST("/disable", controller.DisablePlugin)
		plugins.POST("/delete", controller.DeletePlugin)
		plugins.POST("/config", controller.GetPluginConfig)
		plugins.POST("/config/update", controller.UpdatePluginConfig)
	}

	admins := router.Group("/admins", middlewares.Auth(privateKey))
	{
		admins.POST("/list", controller.GetAdmins)
		admins.POST("/add", controller.AddAdmin)
		admins.POST("/delete", controller.DeleteAdmin)
	}

	port := os.Getenv("L4D2_MANAGER_PORT")
	if port == "" {
		port = "27020"
	}
	router.Run(":" + port)
}
