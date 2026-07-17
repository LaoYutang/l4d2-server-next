package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorcon/rcon"
)

func GetRconMapList(c *gin.Context) {
	c.JSON(http.StatusOK, logic.GetChapterList())
}

func ChangeMap(c *gin.Context) {
	mapName := c.PostForm("mapName")
	if mapName == "" {
		FailWithError(c, http.StatusBadRequest, "地图名称不能为空")
		return
	}
	defer LogOp(c, "切换地图: "+mapName)()

	conn, err := getRconConnection()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "连接RCON失败: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Execute("changelevel " + mapName)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, "地图切换成功")
}

func GetStatus(c *gin.Context) {
	conn, err := getRconConnection()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "服务器连接失败: %v", err)
		return
	}
	defer conn.Close()

	// 获取服务器状态
	res, err := conn.Execute("status")
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}

	// 获取游戏难度
	difficultyRes, err := conn.Execute("z_difficulty")
	if err != nil {
		// 如果获取难度失败，不影响整体状态获取，设置为未知
		difficultyRes = "Unknown"
	}

	// 获取游戏模式
	gameModeRes, err := conn.Execute("sm_cvar mp_gamemode")
	if err != nil {
		// 如果获取模式失败，不影响整体状态获取，设置为未知
		gameModeRes = "Unknown"
	}

	status := logic.ParseStatus(res)
	status.Difficulty = logic.ParseDifficulty(difficultyRes)
	status.GameMode = logic.ParseGameMode(gameModeRes)

	c.JSON(http.StatusOK, status)
}

func KickUser(c *gin.Context) {
	// 优先接收用户名，如果没有则接收用户ID
	userName := c.PostForm("userName")
	userId := c.PostForm("userId")
	defer LogOp(c, fmt.Sprintf("踢出用户: %s (%s)", userName, userId))()

	var kickTarget string
	if userName != "" {
		kickTarget = `"` + userName + `"` // 用户名需要用引号包围
	} else if userId != "" {
		kickTarget = userId
	} else {
		FailWithError(c, http.StatusBadRequest, "用户名或用户ID不能为空")
		return
	}

	conn, err := getRconConnection()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "连接RCON失败: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Execute("kick " + kickTarget)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, "用户踢出成功")
}

func BanUser(c *gin.Context) {
	// BanUser 需要 SteamID (status中的uniqueid) 或 UserID
	// 推荐使用 SteamID 进行永久封禁，因为 UserID 会变
	// banid <minutes> <userid | uniqueid> { kick }
	// minutes=0 表示永久
	userId := c.PostForm("userId")
	steamId := c.PostForm("steamId")
	kick := c.PostForm("kick") == "true"
	defer LogOp(c, fmt.Sprintf("封禁用户: %s (%s)，立即踢出: %t", steamId, userId, kick))()

	var banTarget string
	if steamId != "" {
		banTarget = steamId
	} else if userId != "" {
		banTarget = userId
	} else {
		FailWithError(c, http.StatusBadRequest, "SteamID或用户ID不能为空")
		return
	}

	conn, err := getRconConnection()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "连接RCON失败: %v", err)
		return
	}
	defer conn.Close()

	// 1. 执行 banid 0 <target>
	// 2. 执行 writeid 保存到 cfg
	// 3. 可选：执行 kick 踢出

	cmdBan := fmt.Sprintf("banid 0 %s", banTarget)
	if kick {
		cmdBan += " kick"
	}

	_, err = conn.Execute(cmdBan)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "RCON封禁命令执行失败: %v", err)
		return
	}

	_, err = conn.Execute("writeid")
	if err != nil {
		// writeid 失败不致命，但最好记录
		fmt.Printf("Warning: writeid failed: %v\n", err)
	}

	c.String(http.StatusOK, "用户封禁成功")
}

func ChangeDifficulty(c *gin.Context) {
	difficulty := c.PostForm("difficulty")
	if difficulty == "" {
		FailWithError(c, http.StatusBadRequest, "难度不能为空")
		return
	}
	defer LogOp(c, "切换难度: "+difficulty)()

	// 验证难度值
	validDifficulties := map[string]string{
		"简单": "Easy",
		"普通": "Normal",
		"高级": "Hard",
		"专家": "Impossible",
	}

	englishDifficulty, ok := validDifficulties[difficulty]
	if !ok {
		FailWithError(c, http.StatusBadRequest, "无效的难度值")
		return
	}

	conn, err := getRconConnection()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "连接RCON失败: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Execute("z_difficulty " + englishDifficulty)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, "难度切换成功")
}

func ChangeGameMode(c *gin.Context) {
	gameMode := c.PostForm("gameMode")
	if gameMode == "" {
		FailWithError(c, http.StatusBadRequest, "游戏模式不能为空")
		return
	}
	defer LogOp(c, "切换模式: "+gameMode)()

	// 验证模式值
	validGameModes := map[string]string{
		"合作":      "coop",
		"写实":      "realism",
		"生存":      "survival",
		"对抗":      "versus",
		"拾荒":      "scavenge",
		"坚守":      "holdout",
		"地球上最后一人": "mutation1",
		"爆头！":     "mutation2",
		"血流不止":    "mutation3",
		"绝境求生":    "mutation4",
		"四剑客":     "mutation5",
		"链锯屠杀":    "mutation7",
		"铁人":      "mutation8",
		"地球上最后侏儒": "mutation9",
		"仅容一人":    "mutation10",
		"医疗末日":    "mutation11",
		"写实对抗":    "mutation12",
		"跟随公升":    "mutation13",
		"碎尸盛宴":    "mutation14",
		"对抗生存":    "mutation15",
		"猎杀派对":    "mutation16",
		"孤胆枪手":    "mutation17",
		"失血对抗":    "mutation18",
		"无尽坦克！":   "mutation19",
		"治疗侏儒":    "mutation20",
		"特感速递":    "community1",
		"流感季节":    "community2",
		"骑乘派对":    "community3",
		"梦魇":      "community4",
		"死亡之门":    "community5",
		"Confogl": "community6",
	}

	englishGameMode, ok := validGameModes[gameMode]
	if !ok {
		FailWithError(c, http.StatusBadRequest, "无效的游戏模式值")
		return
	}

	conn, err := getRconConnection()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "连接RCON失败: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Execute("sm_cvar mp_gamemode " + englishGameMode)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, "游戏模式切换成功")
}

func SetMaxPlayers(c *gin.Context) {
	maxPlayersStr := c.PostForm("maxPlayers")
	maxPlayers, err := strconv.Atoi(maxPlayersStr)
	if err != nil || maxPlayers < 4 || maxPlayers > 30 {
		FailWithError(c, http.StatusBadRequest, "人数必须在 4-30 之间")
		return
	}
	defer LogOp(c, "设置最大人数: "+maxPlayersStr)()

	conn, err := getRconConnection()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "连接RCON失败: %v", err)
		return
	}
	defer conn.Close()

	// sv_visiblemaxplayers first, then sv_maxplayers
	_, err = conn.Execute(fmt.Sprintf("sv_visiblemaxplayers %d", maxPlayers))
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "设置可见人数失败: %v", err)
		return
	}

	_, err = conn.Execute(fmt.Sprintf("sv_maxplayers %d", maxPlayers))
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "设置最大人数失败: %v", err)
		return
	}

	c.String(http.StatusOK, "人数设置成功")
}

func Rcon(c *gin.Context) {
	cmd := c.PostForm("cmd")
	if cmd == "" {
		FailWithError(c, http.StatusBadRequest, "命令不能为空")
		return
	}
	defer LogOp(c, "执行RCON命令: "+cmd)()

	conn, err := getRconConnection()
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "连接RCON失败: %v", err)
		return
	}
	defer conn.Close()

	res, err := conn.Execute(cmd)
	if err != nil {
		FailWithError(c, http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, res)
}

func getRconConnection() (*rcon.Conn, error) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		return nil, fmt.Errorf("服务端未配置RCON链接")
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		return nil, fmt.Errorf("服务端未配置RCON密码")
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		return nil, fmt.Errorf("RCON连接失败: %v", err)
	}
	return conn, nil
}
