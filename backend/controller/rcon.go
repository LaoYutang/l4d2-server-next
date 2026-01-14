package controller

import (
	"fmt"
	"l4d2-manager-next/logic"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorcon/rcon"
)

func GetRconMapList(c *gin.Context) {
	c.JSON(http.StatusOK, logic.GetChapterList())
}

func ChangeMap(c *gin.Context) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON链接")
		return
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON密码")
		return
	}

	mapName := c.PostForm("mapName")
	if mapName == "" {
		c.String(http.StatusBadRequest, "地图名称不能为空")
		return
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON连接失败: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Execute("changelevel " + mapName)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, "地图切换成功")
}

func GetStatus(c *gin.Context) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON链接")
		return
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON密码")
		return
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON连接失败: %v", err)
		return
	}
	defer conn.Close()

	// 获取服务器状态
	res, err := conn.Execute("status")
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON命令执行失败: %v", err)
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

	status := parseStatus(res)
	status.Difficulty = parseDifficulty(difficultyRes)
	status.GameMode = parseGameMode(gameModeRes)

	c.JSON(http.StatusOK, status)
}

type User struct {
	Name     string
	Id       int
	SteamId  string
	Ip       string
	Status   string
	Delay    int
	Loss     int
	Duration string
	LinkRate int
}

type Status struct {
	Users      []User
	Players    string
	Map        string
	Hostname   string
	Difficulty string
	GameMode   string
}

func parseStatus(statusText string) *Status {
	status := &Status{}
	lines := strings.Split(statusText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 解析hostname
		if strings.HasPrefix(line, "hostname: ") {
			status.Hostname = strings.TrimPrefix(line, "hostname: ")
		}

		// 解析map
		if strings.HasPrefix(line, "map     : ") {
			status.Map = strings.TrimPrefix(line, "map     : ")
		}

		// 解析players count
		if strings.HasPrefix(line, "players : ") {
			// 匹配格式: "1 humans, 0 bots (18 max)"
			re := regexp.MustCompile(`(\d+) humans, \d+ bots \((\d+) max\)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 2 {
				currentPlayers := matches[1]
				maxPlayers := matches[2]
				status.Players = fmt.Sprintf("%s/%s", currentPlayers, maxPlayers)
			}
		}

		// 解析用户信息 (以#开头的行，但不是注释行)
		if strings.HasPrefix(line, "# ") && !strings.Contains(line, "userid name") && !strings.Contains(line, "end") {
			user := parseUser(line)
			if user != nil {
				status.Users = append(status.Users, *user)
			}
		}
	}

	return status
}

func parseDifficulty(difficultyText string) string {
	// 解析z_difficulty命令的返回值
	// 格式类似: "z_difficulty" = "Easy" ( def. "Normal" )
	//          game replicated
	//          - Difficulty of the current game (Easy, Normal, Hard, Impossible)

	// 使用正则表达式提取难度值
	re := regexp.MustCompile(`"z_difficulty"\s*=\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(difficultyText)

	if len(matches) > 1 {
		difficulty := matches[1]

		// 转换为中文显示
		switch strings.ToLower(difficulty) {
		case "easy":
			return "简单"
		case "normal":
			return "普通"
		case "hard":
			return "高级"
		case "impossible":
			return "专家"
		default:
			return difficulty
		}
	}

	return "未知"
}

func parseGameMode(gameModeText string) string {
	// 解析sm_cvar mp_gamemode命令的返回值
	// 格式类似: [SM] Value of cvar "mp_gamemode": "coop"
	// 或者: "mp_gamemode" = "coop" ( def. "coop" )

	// 先尝试匹配 [SM] 格式
	reSM := regexp.MustCompile(`\[SM\]\s*Value of cvar "mp_gamemode":\s*"([^"]+)"`)
	matches := reSM.FindStringSubmatch(gameModeText)

	if len(matches) > 1 {
		gameMode := matches[1]
		return translateGameMode(gameMode)
	}

	// 再尝试匹配标准格式
	re := regexp.MustCompile(`"mp_gamemode"\s*=\s*"([^"]+)"`)
	matches = re.FindStringSubmatch(gameModeText)

	if len(matches) > 1 {
		gameMode := matches[1]
		return translateGameMode(gameMode)
	}

	return "未知"
}

func translateGameMode(gameMode string) string {
	// 转换为中文显示
	switch strings.ToLower(gameMode) {
	case "coop":
		return "合作"
	case "realism":
		return "写实"
	case "survival":
		return "生存"
	case "versus":
		return "对抗"
	case "scavenge":
		return "拾荒"
	case "holdout":
		return "坚守"
	case "mutation1":
		return "地球上最后一人"
	case "mutation2":
		return "爆头！"
	case "mutation3":
		return "血流不止"
	case "mutation4":
		return "绝境求生"
	case "mutation5":
		return "四剑客"
	case "mutation7":
		return "链锯屠杀"
	case "mutation8":
		return "铁人"
	case "mutation9":
		return "地球上最后侏儒"
	case "mutation10":
		return "仅容一人"
	case "mutation11":
		return "医疗末日"
	case "mutation12":
		return "写实对抗"
	case "mutation13":
		return "跟随公升"
	case "mutation14":
		return "碎尸盛宴"
	case "mutation15":
		return "对抗生存"
	case "mutation16":
		return "猎杀派对"
	case "mutation17":
		return "孤胆枪手"
	case "mutation18":
		return "失血对抗"
	case "mutation19":
		return "无尽坦克！"
	case "mutation20":
		return "治疗侏儒"
	case "community1":
		return "特感速递"
	case "community2":
		return "流感季节"
	case "community3":
		return "骑乘派对"
	case "community4":
		return "梦魇"
	case "community5":
		return "死亡之门"
	case "community6":
		return "Confogl"
	default:
		return gameMode
	}
}

func parseUser(line string) *User {
	// 使用正则表达式解析用户信息
	// # 125 5 "LaoYutang" STEAM_1:1:85790159  2:23:17 41 0 active 60000 61.141.153.96:52904
	// 时间格式: \d+(?::\d+)+ 可以匹配 2:45, 2:45:54, 1:2:45:54 等
	re := regexp.MustCompile(`^#\s*(\d+)\s+(\d+)\s+"([^"]+)"\s+([A-Z_:0-9]+)\s+(\d+(?::\d+)+)\s+(\d+)\s+(\d+)\s+(\w+)\s+(\d+)\s+([0-9.]+:\d+)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 11 {
		return nil
	}

	userid, _ := strconv.Atoi(matches[1])
	delay, _ := strconv.Atoi(matches[6])
	loss, _ := strconv.Atoi(matches[7])
	linkRate, _ := strconv.Atoi(matches[9])

	return &User{
		Name:     matches[3],
		Id:       userid,
		SteamId:  matches[4],
		Ip:       matches[10],
		Status:   matches[8],
		Delay:    delay,
		Loss:     loss,
		Duration: matches[5],
		LinkRate: linkRate,
	}
}

func KickUser(c *gin.Context) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON链接")
		return
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON密码")
		return
	}

	// 优先接收用户名，如果没有则接收用户ID
	userName := c.PostForm("userName")
	userId := c.PostForm("userId")

	var kickTarget string
	if userName != "" {
		kickTarget = `"` + userName + `"` // 用户名需要用引号包围
	} else if userId != "" {
		kickTarget = userId
	} else {
		c.String(http.StatusBadRequest, "用户名或用户ID不能为空")
		return
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON连接失败: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Execute("kick " + kickTarget)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, "用户踢出成功")
}

func BanUser(c *gin.Context) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON链接")
		return
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON密码")
		return
	}

	// BanUser 需要 SteamID (status中的uniqueid) 或 UserID
	// 推荐使用 SteamID 进行永久封禁，因为 UserID 会变
	// banid <minutes> <userid | uniqueid> { kick }
	// minutes=0 表示永久
	userId := c.PostForm("userId")
	steamId := c.PostForm("steamId")
	kick := c.PostForm("kick") == "true"

	var banTarget string
	if steamId != "" {
		banTarget = steamId
	} else if userId != "" {
		banTarget = userId
	} else {
		c.String(http.StatusBadRequest, "SteamID或用户ID不能为空")
		return
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON连接失败: %v", err)
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
		c.String(http.StatusInternalServerError, "RCON封禁命令执行失败: %v", err)
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
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON链接")
		return
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON密码")
		return
	}

	difficulty := c.PostForm("difficulty")
	if difficulty == "" {
		c.String(http.StatusBadRequest, "难度不能为空")
		return
	}

	// 验证难度值
	validDifficulties := map[string]string{
		"简单": "Easy",
		"普通": "Normal",
		"高级": "Hard",
		"专家": "Impossible",
	}

	englishDifficulty, ok := validDifficulties[difficulty]
	if !ok {
		c.String(http.StatusBadRequest, "无效的难度值")
		return
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON连接失败: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Execute("z_difficulty " + englishDifficulty)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, "难度切换成功")
}

func ChangeGameMode(c *gin.Context) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON链接")
		return
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON密码")
		return
	}

	gameMode := c.PostForm("gameMode")
	if gameMode == "" {
		c.String(http.StatusBadRequest, "游戏模式不能为空")
		return
	}

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
		c.String(http.StatusBadRequest, "无效的游戏模式值")
		return
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON连接失败: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Execute("sm_cvar mp_gamemode " + englishGameMode)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, "游戏模式切换成功")
}

func Rcon(c *gin.Context) {
	url := os.Getenv("L4D2_RCON_URL")
	if url == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON链接")
		return
	}
	pwd := os.Getenv("L4D2_RCON_PASSWORD")
	if pwd == "" {
		c.String(http.StatusInternalServerError, "服务端未配置RCON密码")
		return
	}
	cmd := c.PostForm("cmd")
	if cmd == "" {
		c.String(http.StatusBadRequest, "命令不能为空")
		return
	}

	conn, err := rcon.Dial(url, pwd)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON连接失败: %v", err)
		return
	}
	defer conn.Close()

	res, err := conn.Execute(cmd)
	if err != nil {
		c.String(http.StatusInternalServerError, "RCON命令执行失败: %v", err)
		return
	}
	c.String(http.StatusOK, res)
}
