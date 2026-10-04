package main

import (
	"l4d2-manager-next/consts"
	"l4d2-manager-next/controller"
	"l4d2-manager-next/db"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"
	"l4d2-manager-next/utility"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func main() {
	if err := logic.InitPluginMetadata(); err != nil {
		log.Printf("plugin metadata initialization: %v", err)
	}
	// Initialize DB if enabled
	db.InitDB()
	db.InitPlayerStatsDB()
	db.InitAuditDB()
	logic.StartAuditWriter()
	if err := logic.InitAuthCodes(); err != nil {
		log.Printf("authorization store initialization: %v", err)
	} else {
		defer logic.GetAuthCodeStore().Close()
	}

	// Initialize Monitor
	go controller.StartMonitor()
	go controller.StartPlayerStatsCollector()

	// Initialize Chunk Upload Cleaner
	go controller.StartChunkUploadCleaner()

	// Clean leftover plugin download temp directories from previous runs
	go logic.CleanDownloadTemp()
	go logic.CleanPluginExportTemp()
	go logic.CleanVPKTrimTemp()
	go logic.CleanVPKScriptEditTemp()

	logic.InitAccessControl()
	if err := logic.EnsureGameBanPersistenceConfig(); err != nil {
		log.Printf("game ban persistence configuration is not ready: %v", err)
	}

	router := gin.Default()
	if err := router.SetTrustedProxies(nil); err != nil {
		panic("配置 Gin 可信代理失败: " + err.Error())
	}

	// Initialize GeoIP
	// Initialize independently so IP/CIDR access rules still work without GeoIP.
	if utility.InitGeoIP("./ip2region_v4.xdb", "./ip2region_v6.xdb") {
		defer utility.CloseGeoIP()
	}
	router.Use(middlewares.AccessControl())

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

	// 如果maplist.txt不存在，创建一个空的
	mapListPath := filepath.Join(consts.MapListFilePath)
	if _, err := os.Stat(mapListPath); os.IsNotExist(err) {
		err := os.WriteFile(mapListPath, []byte(""), 0755)
		if err != nil {
			panic("创建maplist.txt失败")
		}
	}

	router.MaxMultipartMemory = 1 << 25 // 限制表单内存缓存为32M

	router.POST("/auth", middlewares.Auth(), controller.Auth)
	authCodes := router.Group("/auth-codes", middlewares.Auth())
	{
		authCodes.POST("/list", controller.ListAuthCodes)
		authCodes.POST("/create", controller.CreateAuthCode)
		authCodes.POST("/update", controller.UpdateAuthCode)
		authCodes.POST("/revoke", controller.RevokeAuthCode)
		authCodes.POST("/delete", controller.DeleteAuthCode)
		authCodes.POST("/cleanup-expired", controller.CleanupExpiredAuthCodes)
	}

	router.POST("/self-service/status", controller.GetSelfServiceStatus)
	router.POST("/self-service/generate", controller.GenerateSelfServiceCode)
	router.POST("/config/self-service", middlewares.Auth(), controller.SetSelfServiceConfig)
	router.POST("/config/player-stats", middlewares.Auth(), controller.SetPlayerStatsConfig)
	router.POST("/config/monitor-history", middlewares.Auth(), controller.SetMonitorConfig)
	router.POST("/vpk-trim/config", middlewares.Auth(), controller.GetVPKTrimConfig)
	router.POST("/config/vpk-trim", middlewares.Auth(), controller.SetVPKTrimConfig)
	router.POST("/disk-usage/config", middlewares.Auth(), controller.GetDiskUsageConfig)
	router.POST("/config/disk-usage", middlewares.Auth(), controller.SetDiskUsageConfig)

	// Root Level Protected Routes (Misc)
	router.POST("/upload/init", middlewares.Auth(), controller.UploadInit)
	router.POST("/upload/chunk", middlewares.Auth(), controller.UploadChunk)
	router.POST("/upload/status", middlewares.Auth(), controller.UploadStatus)
	router.POST("/upload/merge", middlewares.Auth(), controller.UploadMerge)
	router.POST("/upload/cancel", middlewares.Auth(), controller.UploadCancel)
	router.POST("/restart", middlewares.Auth(), controller.Restart)
	router.POST("/clear", middlewares.Auth(), controller.Clear)
	router.POST("/list", middlewares.Auth(), controller.List)
	router.POST("/maps/hot-reload", middlewares.Auth(), controller.HotReloadMaps)
	router.POST("/maps/hot-reload/status", middlewares.Auth(), controller.GetMapHotReloadStatus)
	router.POST("/maps/hot-reload/config", middlewares.Auth(), controller.GetMapHotReloadConfig)
	router.POST("/maps/hot-reload/config/update", middlewares.Auth(), controller.SetMapHotReloadConfig)
	router.POST("/maps/detail", middlewares.Auth(), controller.GetMapMissionDetail)
	router.POST("/maps/summary", middlewares.Auth(), controller.GetMapSummaries)
	router.POST("/maps/inspection/global-scripts", middlewares.Auth(), controller.GetMapGlobalScripts)
	router.POST("/maps/inspection/script-overrides", middlewares.Auth(), controller.GetMapScriptOverrides)
	router.POST("/maps/inspection/global-scripts/update", middlewares.Auth(), controller.UpdateMapGlobalScript)
	router.POST("/maps/trim", middlewares.Auth(), controller.TrimMap)

	// Map Queue Group
	mapQueue := router.Group("/maps/queue", middlewares.Auth())
	{
		mapQueue.POST("/snapshot", controller.GetMapQueueSnapshot)
		mapQueue.POST("/add", controller.AddMapQueueItem)
		mapQueue.POST("/remove", controller.RemoveMapQueueItems)
		mapQueue.POST("/start", controller.StartMapQueue)
		mapQueue.POST("/pause", controller.PauseMapQueue)
		mapQueue.POST("/skip", controller.SkipMapQueueItem)
		mapQueue.POST("/clear", controller.ClearMapQueue)
	}

	router.POST("/remove", middlewares.Auth(), controller.Remove)
	router.POST("/rename", middlewares.Auth(), controller.RenameMap)
	router.POST("/getUserPlaytime", middlewares.Auth(), controller.GetUserPlaytime)
	router.POST("/getVersion", controller.GetVersion) // Public

	// RCON Group
	rcon := router.Group("/rcon", middlewares.Auth())
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
	download := router.Group("/download", middlewares.Auth())
	{
		download.POST("/add", controller.AddDownloadTask)
		download.POST("/clear", controller.ClearTasks)
		download.POST("/list", controller.GetDownloadTasksInfo)
		download.POST("/cancel", controller.CancelDownloadTask)
		download.POST("/restart", controller.RestartDownloadTask)
		download.POST("/link/parse", controller.ParseDownloadLink)
		download.POST("/workshop/parse", controller.ParseWorkshopDownloadLink)
		download.POST("/config", controller.GetDownloadConfig)
		download.POST("/config/update", controller.SetDownloadConfig)
	}

	// Monitor Group
	monitor := router.Group("/monitor", middlewares.Auth())
	{
		monitor.POST("/status", controller.GetMonitorStatus)
		monitor.POST("/config", controller.GetMonitorConfig)
		monitor.POST("/history", controller.GetMonitorHistory)
	}

	playerStats := router.Group("/player-stats", middlewares.Auth())
	{
		playerStats.POST("/config", controller.GetPlayerStatsConfig)
		playerStats.POST("/hourly", controller.GetPlayerStatsHourly)
		playerStats.POST("/players/search", controller.SearchPlayerStatsPlayers)
		playerStats.POST("/player-days", controller.GetPlayerStatsPlayerDays)
	}

	// Server Info Group
	serverInfo := router.Group("/server-info", middlewares.Auth())
	{
		serverInfo.POST("/get", controller.GetServerInfo)
		serverInfo.POST("/update", controller.UpdateServerInfo)
	}

	// Server Config Group
	serverConfig := router.Group("/server-config", middlewares.Auth())
	{
		serverConfig.POST("/get", controller.GetServerConfig)
		serverConfig.POST("/update", controller.UpdateServerConfig)
	}

	// Plugins Group
	plugins := router.Group("/plugins", middlewares.Auth())
	{
		plugins.POST("/list", controller.GetPlugins)
		plugins.POST("/upload", controller.UploadPlugin)
		plugins.POST("/export-all/start", controller.StartExportAllPlugins)
		plugins.POST("/export-all/status", controller.GetExportAllPluginsStatus)
		plugins.POST("/export-all/download", controller.DownloadExportAllPlugins)
		plugins.POST("/export-all/cancel", controller.CancelExportAllPlugins)
		plugins.POST("/enable", controller.EnablePlugin)
		plugins.POST("/enable-and-load", controller.EnableAndLoadPlugin)
		plugins.POST("/load", controller.LoadPlugin)
		plugins.POST("/reload", controller.ReloadPlugin)
		plugins.POST("/unload", controller.UnloadPlugin)
		plugins.POST("/enable-batch", controller.EnablePlugins)
		plugins.POST("/disable", controller.DisablePlugin)
		plugins.POST("/disable-and-unload", controller.DisableAndUnloadPlugin)
		plugins.POST("/disable-batch", controller.DisablePlugins)
		plugins.POST("/delete", controller.DeletePlugin)
		plugins.POST("/config", controller.GetPluginConfig)
		plugins.POST("/config/update", controller.UpdatePluginConfig)
		plugins.POST("/text-config/list", controller.ListPluginTextConfigs)
		plugins.POST("/text-config/read", controller.ReadPluginTextConfig)
		plugins.POST("/text-config/update", controller.UpdatePluginTextConfig)
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
	admins := router.Group("/admins", middlewares.Auth())
	{
		admins.POST("/list", controller.GetAdmins)
		admins.POST("/add", controller.AddAdmin)
		admins.POST("/delete", controller.DeleteAdmin)
	}

	// Logs Group
	logs := router.Group("/logs", middlewares.Auth())
	{
		logs.POST("/list", controller.ListSourceModLogs)
		logs.POST("/cleanup/preview", controller.PreviewSourceModLogCleanup)
		logs.POST("/delete", controller.DeleteSourceModLogs)
	}
	router.GET("/logs/stream", middlewares.Auth(), controller.StreamSourceModLog)

	// Audit Group
	audit := router.Group("/audit", middlewares.Auth())
	{
		audit.POST("/list", controller.ListAuditLogs)
	}

	// Panel Access Control Group (admin-only checks are enforced in controllers)
	accessControl := router.Group("/access-control", middlewares.Auth())
	{
		accessControl.POST("/config", controller.GetAccessControlConfig)
		accessControl.POST("/preview", controller.PreviewAccessControl)
		accessControl.POST("/panel-rules/update", controller.UpdatePanelAccessRules)
		accessControl.POST("/trusted-proxies/update", controller.UpdateAccessControlTrustedProxies)
		accessControl.POST("/game-bans/list", controller.ListGameBans)
		accessControl.POST("/game-bans/add", controller.AddGameBan)
		accessControl.POST("/game-bans/remove", controller.RemoveGameBan)
	}

	// Static files fallback: serve from ./static for unmatched routes (SPA)
	router.NoRoute(gin.WrapH(http.FileServer(http.Dir("./static"))))

	port := os.Getenv("L4D2_MANAGER_PORT")
	if port == "" {
		port = "27020"
	}
	router.Run(":" + port)
}
