package main

import (
	"l4d2-manager-next/consts"
	"l4d2-manager-next/controller"
	"l4d2-manager-next/db"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"
	"l4d2-manager-next/utility"
	"net/http"
	"os"
	"path/filepath"

	"github.com/duke-git/lancet/v2/random"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize DB if enabled
	db.InitDB()
	db.InitPlayerStatsDB()

	// Initialize Monitor
	go controller.StartMonitor()
	go controller.StartPlayerStatsCollector()

	// Initialize Chunk Upload Cleaner
	go controller.StartChunkUploadCleaner()

	// Clean leftover plugin download temp directories from previous runs
	go logic.CleanDownloadTemp()
	go logic.CleanPluginExportTemp()
	go logic.CleanVPKTrimTemp()

	router := gin.Default()

	// Initialize GeoIP
	// Try to initialize regardless of whitelist setting to enable location query
	if utility.InitGeoIP("./ip2region_v4.xdb", "./ip2region_v6.xdb") {
		defer utility.CloseGeoIP()
		// Initialize middleware state (whitelist)
		middlewares.InitGeoIPMiddleware()
		// Only apply blocking middleware if whitelist is configured
		if os.Getenv("REGION_WHITE_LIST") != "" {
			router.Use(middlewares.BlockForeignIPs())
		}
	}

	// Static files cache middleware
	router.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/" || path == "/index.html" {
			// HTML files: use ETag/Last-Modified (negotiation)
			c.Header("Cache-Control", "no-cache")
		} else if filepath.Ext(path) != "" {
			// Other static resources (js/css/etc with hash): cache for 3 days
			c.Header("Cache-Control", "public, max-age=259200")
		}
		c.Next()
	})

	// 如果本地的private.key不存在，创建一个随机HS256密钥
	privateKeyPath := consts.PrivateKeyPath
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

	// Auth Group
	auth := router.Group("/auth", middlewares.Auth(privateKey))
	{
		auth.POST("", controller.Auth)
		auth.POST("/getTempAuthCode", controller.GetTempAuthCode)
	}

	// Self-service Auth (Inject privateKey without full Auth check)
	injectKey := func(c *gin.Context) {
		c.Set("privateKey", privateKey)
		c.Next()
	}
	router.POST("/self-service/status", controller.GetSelfServiceStatus)
	router.POST("/self-service/generate", injectKey, controller.GenerateSelfServiceCode)
	router.POST("/config/self-service", middlewares.Auth(privateKey), controller.SetSelfServiceConfig)
	router.POST("/config/player-stats", middlewares.Auth(privateKey), controller.SetPlayerStatsConfig)
	router.POST("/config/monitor-history", middlewares.Auth(privateKey), controller.SetMonitorConfig)
	router.POST("/vpk-trim/config", middlewares.Auth(privateKey), controller.GetVPKTrimConfig)
	router.POST("/config/vpk-trim", middlewares.Auth(privateKey), controller.SetVPKTrimConfig)

	// Root Level Protected Routes (Misc)
	router.POST("/upload", middlewares.Auth(privateKey), controller.Upload)
	router.POST("/upload/init", middlewares.Auth(privateKey), controller.UploadInit)
	router.POST("/upload/chunk", middlewares.Auth(privateKey), controller.UploadChunk)
	router.POST("/upload/status", middlewares.Auth(privateKey), controller.UploadStatus)
	router.POST("/upload/merge", middlewares.Auth(privateKey), controller.UploadMerge)
	router.POST("/upload/cancel", middlewares.Auth(privateKey), controller.UploadCancel)
	router.POST("/restart", middlewares.Auth(privateKey), controller.Restart)
	router.POST("/clear", middlewares.Auth(privateKey), controller.Clear)
	router.POST("/list", middlewares.Auth(privateKey), controller.List)
	router.POST("/maps/detail", middlewares.Auth(privateKey), controller.GetMapMissionDetail)
	router.POST("/remove", middlewares.Auth(privateKey), controller.Remove)
	router.POST("/rename", middlewares.Auth(privateKey), controller.RenameMap)
	router.POST("/getUserPlaytime", middlewares.Auth(privateKey), controller.GetUserPlaytime)
	router.POST("/getVersion", controller.GetVersion) // Public

	// RCON Group
	rcon := router.Group("/rcon", middlewares.Auth(privateKey))
	{
		rcon.POST("", controller.Rcon) //
		rcon.POST("/maplist", controller.GetRconMapList)
		rcon.POST("/changemap", controller.ChangeMap)
		rcon.POST("/getstatus", controller.GetStatus)
		rcon.POST("/kickuser", controller.KickUser)
		rcon.POST("/banuser", controller.BanUser)
		rcon.POST("/changedifficulty", controller.ChangeDifficulty)
		rcon.POST("/changegamemode", controller.ChangeGameMode)
		rcon.POST("/setmaxplayers", controller.SetMaxPlayers)
	}

	// Download Group
	download := router.Group("/download", middlewares.Auth(privateKey))
	{
		download.POST("/add", controller.AddDownloadTask)
		download.POST("/clear", controller.ClearTasks)
		download.POST("/list", controller.GetDownloadTasksInfo)
		download.POST("/cancel", controller.CancelDownloadTask)
		download.POST("/restart", controller.RestartDownloadTask)
		download.POST("/link/parse", controller.ParseDownloadLink)
		download.POST("/workshop/parse", controller.ParseWorkshopDownloadLink)
	}

	// Monitor Group
	monitor := router.Group("/monitor", middlewares.Auth(privateKey))
	{
		monitor.POST("/status", controller.GetMonitorStatus)
		monitor.POST("/config", controller.GetMonitorConfig)
		monitor.POST("/history", controller.GetMonitorHistory)
	}

	playerStats := router.Group("/player-stats", middlewares.Auth(privateKey))
	{
		playerStats.POST("/config", controller.GetPlayerStatsConfig)
		playerStats.POST("/hourly", controller.GetPlayerStatsHourly)
		playerStats.POST("/players/search", controller.SearchPlayerStatsPlayers)
		playerStats.POST("/player-days", controller.GetPlayerStatsPlayerDays)
	}

	// Server Info Group
	serverInfo := router.Group("/server-info", middlewares.Auth(privateKey))
	{
		serverInfo.POST("/get", controller.GetServerInfo)
		serverInfo.POST("/update", controller.UpdateServerInfo)
	}

	// Server Config Group
	serverConfig := router.Group("/server-config", middlewares.Auth(privateKey))
	{
		serverConfig.POST("/get", controller.GetServerConfig)
		serverConfig.POST("/update", controller.UpdateServerConfig)
	}

	// Plugins Group
	plugins := router.Group("/plugins", middlewares.Auth(privateKey))
	{
		plugins.POST("/list", controller.GetPlugins)
		plugins.POST("/upload", controller.UploadPlugin)
		plugins.POST("/export-all/start", controller.StartExportAllPlugins)
		plugins.POST("/export-all/status", controller.GetExportAllPluginsStatus)
		plugins.POST("/export-all/download", controller.DownloadExportAllPlugins)
		plugins.POST("/export-all/cancel", controller.CancelExportAllPlugins)
		plugins.POST("/enable", controller.EnablePlugin)
		plugins.POST("/enable-and-load", controller.EnableAndLoadPlugin)
		plugins.POST("/enable-batch", controller.EnablePlugins)
		plugins.POST("/disable", controller.DisablePlugin)
		plugins.POST("/disable-and-unload", controller.DisableAndUnloadPlugin)
		plugins.POST("/disable-batch", controller.DisablePlugins)
		plugins.POST("/delete", controller.DeletePlugin)
		plugins.POST("/config", controller.GetPluginConfig)
		plugins.POST("/config/update", controller.UpdatePluginConfig)
		plugins.POST("/presets", controller.GetPresets)
		plugins.POST("/apply-preset", controller.ApplyPreset)
		plugins.POST("/readme", controller.GetPluginReadme)
		plugins.POST("/store/list", controller.GetStorePlugins)
		plugins.POST("/store/download", controller.DownloadStorePlugin)
		plugins.POST("/store/download/status", controller.GetStoreDownloadStatus)
		plugins.POST("/store/download/cancel", controller.CancelStoreDownload)
		plugins.POST("/backups/list", controller.ListBackups)
		plugins.POST("/backups/create", controller.CreateBackup)
		plugins.POST("/backups/restore", controller.RestoreBackup)
		plugins.POST("/backups/rename", controller.RenameBackup)
		plugins.POST("/backups/delete", controller.DeleteBackup)
		plugins.POST("/backups/detail/plugins", controller.GetBackupPluginsDetail)
		plugins.POST("/backups/detail/admins", controller.GetBackupAdminsDetail)
		plugins.POST("/backups/detail/server_info", controller.GetBackupServerInfoDetail)
		plugins.POST("/backups/detail/server_config", controller.GetBackupServerConfigDetail)
		plugins.POST("/backups/export", controller.ExportBackup)
		plugins.POST("/backups/export-all", controller.ExportAllBackups)
		plugins.POST("/backups/import", controller.ImportBackup)
	}

	// Admins Group
	admins := router.Group("/admins", middlewares.Auth(privateKey))
	{
		admins.POST("/list", controller.GetAdmins)
		admins.POST("/add", controller.AddAdmin)
		admins.POST("/delete", controller.DeleteAdmin)
	}

	// Logs Group
	logs := router.Group("/logs", middlewares.Auth(privateKey))
	{
		logs.POST("/list", controller.ListSourceModLogs)
	}
	router.GET("/logs/stream", middlewares.Auth(privateKey), controller.StreamSourceModLog)

	// Static files fallback: serve from ./static for unmatched routes (SPA)
	router.NoRoute(gin.WrapH(http.FileServer(http.Dir("./static"))))

	port := os.Getenv("L4D2_MANAGER_PORT")
	if port == "" {
		port = "27020"
	}
	router.Run(":" + port)
}
