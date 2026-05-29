package logic

import (
	"fmt"
	"l4d2-manager-next/utility"
	"regexp"
	"strconv"
	"strings"
)

type User struct {
	Name     string
	Id       int
	SteamId  string
	Ip       string
	Location string
	Status   string
	Delay    int
	Loss     int
	Duration string
	LinkRate int
}

type Status struct {
	Users       []User
	Players     string
	Map         string
	Hostname    string
	Difficulty  string
	GameMode    string
	PlayerCount int `json:"-"`
	MaxPlayers  int `json:"-"`
}

func ParseStatus(statusText string) *Status {
	status := &Status{}
	lines := strings.Split(statusText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "hostname: ") {
			status.Hostname = strings.TrimPrefix(line, "hostname: ")
		}

		if strings.HasPrefix(line, "map     : ") {
			status.Map = strings.TrimPrefix(line, "map     : ")
		}

		if strings.HasPrefix(line, "players : ") {
			re := regexp.MustCompile(`(\d+) humans, \d+ bots \((\d+) max\)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 2 {
				currentPlayers := matches[1]
				maxPlayers := matches[2]
				status.Players = fmt.Sprintf("%s/%s", currentPlayers, maxPlayers)
				status.PlayerCount, _ = strconv.Atoi(currentPlayers)
				status.MaxPlayers, _ = strconv.Atoi(maxPlayers)
			}
		}

		if strings.HasPrefix(line, "# ") && !strings.Contains(line, "userid name") && !strings.Contains(line, "end") {
			user := ParseUser(line)
			if user != nil {
				status.Users = append(status.Users, *user)
			}
		}
	}

	return status
}

func ParseDifficulty(difficultyText string) string {
	re := regexp.MustCompile(`"z_difficulty"\s*=\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(difficultyText)

	if len(matches) > 1 {
		difficulty := matches[1]

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

func ParseGameMode(gameModeText string) string {
	reSM := regexp.MustCompile(`\[SM\]\s*Value of cvar "mp_gamemode":\s*"([^"]+)"`)
	matches := reSM.FindStringSubmatch(gameModeText)

	if len(matches) > 1 {
		gameMode := matches[1]
		return TranslateGameMode(gameMode)
	}

	re := regexp.MustCompile(`"mp_gamemode"\s*=\s*"([^"]+)"`)
	matches = re.FindStringSubmatch(gameModeText)

	if len(matches) > 1 {
		gameMode := matches[1]
		return TranslateGameMode(gameMode)
	}

	return "未知"
}

func TranslateGameMode(gameMode string) string {
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

func ParseUser(line string) *User {
	re := regexp.MustCompile(`^#\s*(\d+)\s+(\d+)\s+"([^"]+)"\s+([A-Z_:0-9]+)\s+(\d+(?::\d+)+)\s+(\d+)\s+(\d+)\s+(\w+)\s+(\d+)\s+([0-9.]+:\d+)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 11 {
		return nil
	}

	userid, _ := strconv.Atoi(matches[1])
	delay, _ := strconv.Atoi(matches[6])
	loss, _ := strconv.Atoi(matches[7])
	linkRate, _ := strconv.Atoi(matches[9])
	ip := matches[10]

	location := utility.GetLocation(ip)
	if location == "" {
		location = "未知"
	}

	return &User{
		Name:     matches[3],
		Id:       userid,
		SteamId:  matches[4],
		Ip:       ip,
		Location: location,
		Status:   matches[8],
		Delay:    delay,
		Loss:     loss,
		Duration: matches[5],
		LinkRate: linkRate,
	}
}
