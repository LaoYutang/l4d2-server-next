#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#pragma dynamic 331072	//增加堆栈空间.
#include <sourcemod>
#include <dhooks>
#include <left4dhooks>
#include <l4d2_ems_hud>
#undef REQUIRE_PLUGIN	//标记为可选开始.
#include <l4d2_simulation>
#define REQUIRE_PLUGIN	//标记为可选结束.

#define CVAR_FLAGS		FCVAR_NOTIFY
#define PLUGIN_VERSION	"2.19.23"

#define	Number_1		(1 << 0)
#define Number_2		(1 << 1)
#define Number_4		(1 << 2)
#define Number_8		(1 << 3)
#define Number_16		(1 << 4)
#define Number_32		(1 << 5)
#define Number_64		(1 << 6)
#define Number_128		(1 << 7)
#define Number_256		(1 << 8)

//对抗模式.
char g_sModeVersus[][] = 
{
	"versus",		//对抗模式
	"teamversus ",	//团队对抗
	"scavenge",		//团队清道夫
	"teamscavenge",	//团队清道夫
	"community3",	//骑师派对
	"community6",	//药抗模式
	"mutation11",	//没有救赎
	"mutation12",	//写实对抗
	"mutation13",	//清道肆虐
	"mutation15",	//生存对抗
	"mutation18",	//失血对抗
	"mutation19"	//坦克派对?
};

//单人模式.
char g_sModeSingle[][] = 
{
	"mutation1", //孤身一人
	"mutation17" //孤胆枪手
};

//设置数组数量(最大值:16).
#define ArrayNumber	16

bool  g_bMapRunTime, g_bShowHUD, g_bShowServerName, g_bDisplayNumber, g_bSwitchHud = true, g_bLibraries;
float g_fMeleeNerf2, g_fTempValue, g_fVsBossBuff, g_fMapMaxFlow;
float g_fCoord[] = {0.00, 0.055, 0.110, 0.160, 0.215, 0.265, 0.315, 0.365, 0.415};//设置每个HUD类型的预设坐标.

int    g_iDisplayNumber, g_iPlayerNum, g_iChapterTotal[2], g_iCumulativeTotal[2], g_iKillSpecial[MAXPLAYERS+1], g_iHeadSpecial[MAXPLAYERS+1], g_iKillZombie[MAXPLAYERS+1], g_iDmgHealth[2][MAXPLAYERS+1], g_iPlayerHP[MAXPLAYERS+1];

int    g_iSurvivorHealth, g_iMaxReviveCount, g_iPlayersNumber, g_iShowServerName, g_iShowServerTimer, g_iShowServerNumber, g_iShowServerTime, g_iFakeRanking, g_iReportTime, g_iTypeRanking, g_iDispRanking, g_iInfoRanking;
ConVar g_hSurvivorHealth, g_hMaxReviveCount, g_hPlayersNumber, g_hShowServerName, g_hShowServerTimer, g_hShowServerNumber, g_hShowServerTime, g_hFakeRanking, g_hTypeRanking, g_hDispRanking, g_hInfoRanking;
int g_iMaxChapters, g_iCurrentChapter;
ConVar g_hHostName, g_cVsBossBuff;

char g_sDifficultyName[][] = {"简单", "普通", "高级", "专家"};
char g_sDifficultyCode[][] = {"Easy", "Normal", "Hard", "Impossible"};
char g_sTitle[][] = {"状态", "难度", "血量", "特感", "爆头", "丧尸", "被黑", "友伤", "名字"};//这里最好不要改变长度,否则自动对齐可能怪怪的.
char g_sWeekName[][] = {"一", "二", "三", "四", "五", "六", "日"};

Handle g_hTimerHUD, g_hDisplayNumber;

public Plugin myinfo = 
{
	name 			= "l4d2_emshud_info",
	author 			= "豆瓣酱な | HUD的include提供者:sorallll",
	description 	= "HUD显示各种信息.",
	version 		= PLUGIN_VERSION,
	url 			= "N/A"
}

public void OnPluginStart()
{
	LoadGameCFG();
	
	HookEvent("round_end",		Event_RoundEnd);	//回合结束.
	HookEvent("round_start",	Event_RoundStart);	//回合开始.
	HookEvent("player_hurt",	Event_PlayerHurt);	//玩家受伤.
	HookEvent("player_death",	Event_PlayerDeath);	//玩家死亡.
	HookEvent("player_incapacitated", Event_Incapacitate, EventHookMode_Pre);//玩家倒下.

	RegConsoleCmd("sm_hud", CommandSwitchHud, "开启或关闭所有HUD.");
	
	g_hHostName			= FindConVar("hostname");
	g_cVsBossBuff		= FindConVar("versus_boss_buffer");
	g_hSurvivorHealth	= FindConVar("survivor_limp_health");
	g_hMaxReviveCount	= FindConVar("survivor_max_incapacitated_count");

	g_hPlayersNumber	= CreateConVar("l4d2_emshud_show_players_number", "127", "显示玩家数量信息(需要启用的功能数字相加,值为<0>时自动隐藏). 0=禁用, 1=连接, 2=闲置, 4=旁观, 8=丧尸, 16=女巫, 32=特感, 64=生还, 127=全部.", CVAR_FLAGS);
	g_hShowServerName	= CreateConVar("l4d2_emshud_show_server_name_type", "2", "设置服务器名称的显示方式. -1=居中显示, 0=禁用, 1=从右到左显示, >1=连续显示多少次.", CVAR_FLAGS);
	g_hShowServerTimer	= CreateConVar("l4d2_emshud_show_server_name_time", "10", "设置服务器名字的显示间隔(秒).", CVAR_FLAGS);
	g_hShowServerNumber	= CreateConVar("l4d2_emshud_show_server_name_number", "31", "显示服务器人数,路程,坦克和女巫刷新百分比. 0=禁用, 1=服务器人数, 2=地图章节数量, 4=路程显示, 8=刷坦克路程(需要额外配套插件支持), 16=刷女巫路程(需要额外配套插件支持), 31=全部.", CVAR_FLAGS);
	g_hShowServerTime	= CreateConVar("l4d2_emshud_show_server_time", "7", "显示服务器时间(需要启用的功能数字相加). 0=禁用, 1=日期, 2=时间, 4=星期.", CVAR_FLAGS);
	g_hFakeRanking		= CreateConVar("l4d2_emshud_ranking_Fake", "0", "排行榜显示电脑幸存者. 0=显示, 1=忽略.", CVAR_FLAGS);
	g_hInfoRanking		= CreateConVar("l4d2_emshud_ranking_Info", "15", "排行榜总共显示多少行(最多15行). 0=禁用.", CVAR_FLAGS);
	g_hTypeRanking		= CreateConVar("l4d2_emshud_ranking_Type", "511", "排行榜显示那些内容(需要启用的功能数字相加). 0=禁用, 1=状态, 2=难度, 4=血量, 8=特感, 16=爆头, 32=丧尸, 64=被黑, 128=友伤, 256=名字, 511=全部.", CVAR_FLAGS);
	g_hDispRanking		= CreateConVar("l4d2_emshud_ranking_disp", "63", "那些内容的值都为<0>时自动隐藏(需要自动隐藏的功能数字相加). 0=显示, 1=血量, 2=特感, 4=爆头, 8=丧尸, 16=被黑, 32=友伤, 63=全部.", CVAR_FLAGS);
	
	g_hSurvivorHealth.AddChangeHook(ConVarChanged);
	g_hMaxReviveCount.AddChangeHook(ConVarChanged);
	g_cVsBossBuff.AddChangeHook(ConVarChanged);
	g_hPlayersNumber.AddChangeHook(ConVarChanged);
	g_hShowServerName.AddChangeHook(ConVarChanged);
	g_hShowServerTimer.AddChangeHook(ConVarChanged);
	g_hShowServerNumber.AddChangeHook(ConVarChanged);
	g_hShowServerTime.AddChangeHook(ConVarChanged);
	g_hFakeRanking.AddChangeHook(ConVarChanged);
	g_hInfoRanking.AddChangeHook(ConVarChanged);
	g_hTypeRanking.AddChangeHook(ConVarChanged);
	g_hDispRanking.AddChangeHook(ConVarChanged);
	
	AutoExecConfig(true, "l4d2_emshud_info");//生成指定文件名的CFG.
}
/* https://github.com/lakwsh */
void LoadGameCFG()
{
	GameData hGameData = new GameData("l4d2_emshud_info");
	if(!hGameData) 
		SetFailState("Failed to load 'l4d2_emshud_info.txt' gamedata.");
	DHookSetup hDetour = DHookCreateFromConf(hGameData, "HibernationUpdate");
	CloseHandle(hGameData);
	if(!hDetour || !DHookEnableDetour(hDetour, true, OnHibernationUpdate)) 
		SetFailState("Failed to hook HibernationUpdate");
}

//服务器里没人后触发一次.
public MRESReturn OnHibernationUpdate(DHookParam hParams)
{
	bool hibernating = DHookGetParam(hParams, 1);

	if(!hibernating) 
		return MRES_Ignored;
		
	g_bMapRunTime = false;
	return MRES_Handled;
}

public void OnConfigsExecuted()
{
	if (g_bMapRunTime == false)
	{
		g_bMapRunTime = true;
	}
}
//所有插件加载完成后执行一次(延迟加载插件也会执行一次).
public void OnAllPluginsLoaded()   
{
	g_bLibraries = LibraryExists("l4d2_simulation");
}
//库被加载时.
public void OnLibraryAdded(const char[] name) 
{
	if (strcmp(name, "l4d2_simulation") == 0)
		g_bLibraries = true;
}
//库被卸载时.
public void OnLibraryRemoved(const char[] name) 
{
	if (strcmp(name, "l4d2_simulation") == 0)
		g_bLibraries = false;
}
public void ConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	GetCvars();
}

void GetCvars()
{
	g_iSurvivorHealth	= g_hSurvivorHealth.IntValue;
	g_iMaxReviveCount	= g_hMaxReviveCount.IntValue;
	g_iPlayersNumber	= g_hPlayersNumber.IntValue;
	g_iShowServerName	= g_hShowServerName.IntValue;
	g_iShowServerTimer	= g_hShowServerTimer.IntValue;
	g_iShowServerNumber	= g_hShowServerNumber.IntValue;
	g_iShowServerTime	= g_hShowServerTime.IntValue;
	g_iFakeRanking		= g_hFakeRanking.IntValue;
	g_iInfoRanking		= g_hInfoRanking.IntValue;
	g_iTypeRanking		= g_hTypeRanking.IntValue;
	g_iDispRanking		= g_hDispRanking.IntValue;
	g_fVsBossBuff		= g_cVsBossBuff.FloatValue;
	
	if( g_iReportTime < 5)
		g_iReportTime = 5;
	if( g_iInfoRanking > ArrayNumber - 1)
		g_iInfoRanking = ArrayNumber - 1;
}

public Action CommandSwitchHud(int client, int args)
{ 
	if(bCheckClientAccess(client))
		IsDisplayHud(client);
	else
		ReplyToCommand(client, "\x04[提示]\x05你无权使用该指令.");
	return Plugin_Handled;
}

void IsDisplayHud(int client)
{
	if(g_bSwitchHud == true)
	{
		g_bSwitchHud = false;//重新赋值.
		delete g_hTimerHUD;//停止计时器.
		RequestFrame(IsRequestRemoveHUD);//延迟一帧清除HUD.
		ReplyToCommand(client, "\x04[提示]\x03已关闭\x05所有\x04HUD\x05显示\x04.");
	}
	else
	{
		IsShowAllHUD();//显示指定的HUD.
		g_bSwitchHud = true;//重新赋值.
		delete g_hTimerHUD;//停止计时器.
		g_hTimerHUD = CreateTimer(1.0, DisplayInfo, _, TIMER_REPEAT);//创建计时器
		ReplyToCommand(client, "\x04[提示]\x03已开启\x05所有\x04HUD\x05显示\x04.");
	}
}

//延迟一帧清除HUD.
void IsRequestRemoveHUD()
{
	IsRemoveListHUD();//清除排行榜HUD.
	IsRemoveOtherHUD();//清除其它的HUD.
}
public void OnClientPutInServer(int client)
{
	SDKHook(client, SDKHook_PostThink, IsTankThink);
}
void IsTankThink(int client)
{
	if (IsValidClient(client) && GetClientTeam(client) == 2 && IsPlayerAlive(client))
	{
		g_iPlayerHP[client] = GetClientHealth(client) + GetPlayerTempHealth(client);
	}
}
//玩家受伤.
public void Event_PlayerHurt(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int attacker = GetClientOfUserId(event.GetInt("attacker"));
	int iDmg = event.GetInt("dmg_health");
	
	if(IsValidClient(client) && GetClientTeam(client) == 2)
	{
		if(IsValidClient(attacker) && GetClientTeam(attacker) == 2)
		{
			int iBot[2];
			iBot[0] = IsClientIdle(client);
			iBot[1] = IsClientIdle(attacker);
			int temp = GetClientHealth(client) + GetPlayerTempHealth(client);//记录玩家剩余的血量.

			if(temp > 0)
				g_iPlayerHP[client] = temp;
			else
				iDmg = g_iPlayerHP[client];

			if(iBot[0] != 0)
			{
				if(IsValidClient(iBot[0]))
					g_iDmgHealth[0][iBot[0]] += iDmg;
			}
			else
				g_iDmgHealth[0][client] += iDmg;

			if(iBot[1] != 0)
			{
				if(IsValidClient(iBot[1]))
					g_iDmgHealth[1][iBot[1]] += iDmg;
			}
			else
				g_iDmgHealth[1][attacker] += iDmg;

			//PrintToChatAll("\x04[提示2]\x05(%s)(%s)(%d)(%d).", GetTrueName(client), GetTrueName(attacker), g_iPlayerHP[client], iDmg);//聊天窗提示.
		}
	}
}
//玩家倒下.
public void Event_Incapacitate(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int attacker = GetClientOfUserId(event.GetInt("attacker"));

	//char sWeapon[32];
	//event.GetString("weapon", sWeapon, sizeof(sWeapon));
	
	if (IsValidClient(attacker) && GetClientTeam(attacker) == 2)
	{
		if (IsValidClient(client) && GetClientTeam(client) == 2 && IsPlayerAlive(client) && GetClientHealth(client) + GetPlayerTempHealth(client))
		{
			int iBot[2];
			iBot[0] = IsClientIdle(client);
			iBot[1] = IsClientIdle(attacker);

			if(iBot[0] != 0)
			{
				if(IsValidClient(iBot[0]))
					g_iDmgHealth[0][iBot[0]] += g_iPlayerHP[client];
			}
			else
				g_iDmgHealth[0][client] += g_iPlayerHP[client];
			
			if(iBot[1] != 0)
			{
				if(IsValidClient(iBot[1]))
					g_iDmgHealth[1][iBot[1]] += g_iPlayerHP[client];
			}
			else
				g_iDmgHealth[1][attacker] += g_iPlayerHP[client];

			//PrintToChatAll("\x04[提示1]\x05(%s)(%s)(%d).", GetTrueName(client), GetTrueName(attacker), g_iPlayerHP[client]);//聊天窗提示.
		}
	}
}
//玩家死亡.
public void Event_PlayerDeath(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int attacker = GetClientOfUserId(event.GetInt("attacker"));
	int iHeadshot = GetEventInt(event, "headshot");

	if(IsValidClient(attacker))
	{
		switch (GetClientTeam(attacker))
		{
			case 2:
			{
				int iBot[2];
				char classname[32];
				iBot[0] = IsClientIdle(client);
				iBot[1] = IsClientIdle(attacker);
				int entity = GetEventInt(event, "entityid");
				GetEdictClassname(entity, classname, sizeof(classname));
				if (IsValidEdict(entity) && strcmp(classname, "infected") == 0)
				{
					g_iChapterTotal[0] += 1;
					g_iCumulativeTotal[0] += 1;
					g_iKillZombie[iBot[1] != 0 ? iBot[1] : attacker] += 1;
				}
				if(IsValidClient(client))
				{
					switch (GetClientTeam(client))
					{
						//case 2://测试发现生还者受伤事件里致命伤也能触发受伤事件.
						//{
						//	if(iBot[0] != 0)
						//	{
						//		if(IsValidClient(iBot[0]))
						//			g_iDmgHealth[0][iBot[0]] += g_iPlayerHP[client];
						//	}
						//	else
						//		g_iDmgHealth[0][client] += g_iPlayerHP[client];
						//	
						//	if(iBot[1] != 0)
						//	{
						//		if(IsValidClient(iBot[1]))
						//			g_iDmgHealth[1][iBot[1]] += g_iPlayerHP[client];
						//	}
						//	else
						//		g_iDmgHealth[1][attacker] += g_iPlayerHP[client];
						//}
						case 3:
						{
							g_iChapterTotal[1] += 1;
							g_iCumulativeTotal[1] += 1;
					
							if(iHeadshot)
								g_iHeadSpecial[iBot[1] != 0 ? iBot[1] : attacker] += 1;
							g_iKillSpecial[iBot[1] != 0 ? iBot[1] : attacker] += 1;
						}
					}
				}
			}
		}
	}
}
//玩家连接
public void OnClientConnected(int client)
{   
	g_iKillSpecial[client] = 0;
	g_iHeadSpecial[client] = 0;
	g_iKillZombie[client] = 0;

	if (!IsFakeClient(client))
		g_iPlayerNum += 1;
}

//玩家离开.
public void OnClientDisconnect(int client)
{   
	g_iKillSpecial[client] = 0;
	g_iHeadSpecial[client] = 0;
	g_iKillZombie[client] = 0;

	if (!IsFakeClient(client))
		g_iPlayerNum -= 1;
}

//地图开始
public void OnMapStart()
{
	GetCvars();
	EnableHUD();
	g_iPlayerNum = 0;
	g_iDisplayNumber = 0;
	delete g_hDisplayNumber;
	g_bDisplayNumber = true;
	g_bShowServerName = false;
}

//回合开始.
public void Event_RoundStart(Event event, const char[] name, bool dontBroadcast)
{
	g_fMeleeNerf2 = 1.0;
	g_bShowHUD = false;
	//创建计时器显示HUD.
	IsCreateTimerShowHUD();
	//重置章节击杀特感和丧尸数量.
	for (int i = 0; i < sizeof(g_iChapterTotal); i++)
		g_iChapterTotal[i] = 0;//重置章节击杀特感和丧尸数量.
	
	//重置玩家友伤,击杀特感和丧尸数量.
	for (int i = 1; i <= MaxClients; i++)
	{
		for (int y = 0; y < sizeof(g_iDmgHealth); y++)
			g_iDmgHealth[y][i] = 0;
		g_iKillSpecial[i] = 0;
		g_iHeadSpecial[i] = 0;
		g_iKillZombie[i] = 0;
	}
}

//回合结束.
public void Event_RoundEnd(Event event, const char[] name, bool dontBroadcast)
{
	g_bShowHUD = true;
	IsRemoveListHUD();//清除排行榜HUD.
	g_fMeleeNerf2 = 1.0;
}

//创建计时器.
void IsCreateTimerShowHUD()
{
	if(g_hTimerHUD == null)//不存在计时器才创建.
		g_hTimerHUD = CreateTimer(1.0, DisplayInfo, _, TIMER_REPEAT);
}

public Action DisplayInfo(Handle timer)
{
	g_iMaxChapters = L4D_GetMaxChapters();
	g_iCurrentChapter = L4D_GetCurrentChapter();
	g_fMapMaxFlow = L4D2Direct_GetMapMaxFlowDistance();
	
	if(g_bSwitchHud == true)
	{
		IsRemoveListHUD();//清除排行榜HUD.
		IsRemoveOtherHUD();//清除其它的HUD.
		IsShowAllHUD();//显示指定的HUD.
	}
	return Plugin_Continue;
}

//清除其它的HUD.
void IsRemoveOtherHUD()
{
	//删除玩家人数HUD.
	if(HUDSlotIsUsed(HUD_SCORE_1))
		RemoveHUD(HUD_SCORE_1);
	
	//删除当前时间HUD.
	if(HUDSlotIsUsed(HUD_MID_TOP))
		RemoveHUD(HUD_MID_TOP);

	//删除玩家数量HUD.
	if(HUDSlotIsUsed(HUD_SCORE_4))
		RemoveHUD(HUD_SCORE_4);
	
	//删除服名HUD.
	if(HUDSlotIsUsed(HUD_LEFT_TOP))
		RemoveHUD(HUD_LEFT_TOP);

	//删除难度HUD.
	if(HUDSlotIsUsed(HUD_MID_BOX))
		RemoveHUD(HUD_MID_BOX);
}

//清除排行榜HUD.
void IsRemoveListHUD()
{
	/* 以下是排行榜相关HUD. */
	//删除玩家状态HUD.
	if(HUDSlotIsUsed(HUD_LEFT_BOT))
		RemoveHUD(HUD_LEFT_BOT);
	
	//删除击杀数量HUD.
	if(HUDSlotIsUsed(HUD_MID_BOT))
		RemoveHUD(HUD_MID_BOT);

	//删除爆头数量HUD.
	if(HUDSlotIsUsed(HUD_RIGHT_TOP))
		RemoveHUD(HUD_RIGHT_TOP);

	//删除玩家血量HUD.
	if(HUDSlotIsUsed(HUD_SCORE_3))
		RemoveHUD(HUD_SCORE_3);

	//删除丧尸统计HUD.
	if(HUDSlotIsUsed(HUD_TICKER))
		RemoveHUD(HUD_TICKER);

	//删除被黑统计HUD.
	if(HUDSlotIsUsed(HUD_FAR_RIGHT))
		RemoveHUD(HUD_FAR_RIGHT);
	
	//删除友伤统计HUD.
	if(HUDSlotIsUsed(HUD_SCORE_2))
		RemoveHUD(HUD_SCORE_2);
		
	//删除玩家名字HUD.
	if(HUDSlotIsUsed(HUD_RIGHT_BOT))
		RemoveHUD(HUD_RIGHT_BOT);

	//删除玩家名字HUD.
	if(HUDSlotIsUsed(HUD_SCORE_TITLE))
		RemoveHUD(HUD_SCORE_TITLE);
}

//显示指定的HUD.
void IsShowAllHUD()
{
	//显示服务器时间.
	if(g_iShowServerTime > 0)
		IsShowServerTime();
	//显示连接,闲置,旁观,特感和幸存者数量.
	if(g_iPlayersNumber > 0)
		IsPlayersNumber();
	//显示服务器名字.
	if(g_iShowServerName <= -1)
		IsShowServerName();
	//显示服务器人数.
	if(g_iShowServerNumber > 0)
		IsShowServersNumber();
	//显示击杀特感排行榜.
	if(g_iInfoRanking > 0)
		IsKillLeaderboards();
}

//开局提示.
public void OnClientPostAdminCheck(int client)
{
	if(!IsFakeClient(client) && g_bShowServerName == false)
	{
		g_fTempValue = IsDynamicVariable(GetConVarInt(FindConVar("sv_maxupdaterate")), RoundToNearest(1.0 / GetTickInterval()));
		g_bShowServerName = true;
	}
}

float IsDynamicVariable(int iSvMaxUpdateRate, int iTickInterval)
{
	float g_fTemp = 0.003;
	if(iSvMaxUpdateRate > 30 && iTickInterval == 30)
		iSvMaxUpdateRate = iTickInterval;
	for (int i = 0; i < iSvMaxUpdateRate; i++)
		g_fTemp -= 0.00002;
	return g_fTemp;
}

public void OnGameFrame()
{
	if(g_iShowServerName <= 0 || g_bShowServerName == false || g_bDisplayNumber == false || g_bSwitchHud == false)
		return;

	IsShowServerName();
}

//显示服务器名字.
void IsShowServerName()
{
	if(g_iShowServerName <= -1)
	{
		HUDSetLayout(HUD_LEFT_TOP, HUD_FLAG_ALIGN_CENTER|HUD_FLAG_NOBG|HUD_FLAG_TEXT|HUD_FLAG_BLINK, GetHostName());
		HUDPlace(HUD_LEFT_TOP, 0.00,0.03, 1.0,0.03);
	}
	else
	{
		g_fMeleeNerf2 -= g_fTempValue;
		if(g_fMeleeNerf2 <= 0)
		{
			g_fMeleeNerf2 = 1.0;
			if(g_iShowServerName > 1)
			{
				g_iDisplayNumber += 1;
				if(g_iDisplayNumber >= g_iShowServerName)
				{
					g_bDisplayNumber = false;
					if(HUDSlotIsUsed(HUD_LEFT_TOP))
						RemoveHUD(HUD_LEFT_TOP);
					if(g_hDisplayNumber == null)
						g_hDisplayNumber = CreateTimer(float(g_iShowServerTimer), IsDisplayNumberTimer);
					return;
				}
			}
		}
			
		HUDSetLayout(HUD_LEFT_TOP, HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEXT|HUD_FLAG_BLINK, GetHostName());
		HUDPlace(HUD_LEFT_TOP, g_fMeleeNerf2,0.03, 1.0,0.03);
	}
}

public Action IsDisplayNumberTimer(Handle timer)
{
	g_iDisplayNumber = 0;
	g_bDisplayNumber = true;
	g_hDisplayNumber = null;
	return Plugin_Continue;
}

//显示击杀特感排行榜.
void IsKillLeaderboards()
{
	if (g_bShowHUD || g_iTypeRanking == 0 || GetPlayersMaxNumber(2, false) <= 0)//没有幸存者或禁用时直接返回，不执行后面的操作.
		return;
	
	int iValue[3] = {3,2,2};//第一个是需要对齐的数量,第二个是从第几个开始使用对齐,第三个是数组减1和排除最后的玩家名字.
	int ranking_count = 1, assister_count, iHudCoord;
	int[] temp = new int[sizeof(g_sTitle) - iValue[0]];//更改为动态大小的数组.
	int[] iMax = new int[sizeof(g_sTitle) - iValue[0]];//更改为动态大小的数组.
	int[][] assisters = new int[MaxClients][sizeof(g_sTitle) - 1];//更改为动态大小的数组.
	
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientInGame(i) && GetClientTeam(i) == 2)
		{
			int iBot = IsClientIdle(i);

			if (iBot != 0 && IsClientConnected(i) && !IsClientInGame(iBot) || g_iFakeRanking != 0 && IsFakeClient(iBot == 0 ? i : iBot))//这里判断玩家是否在游戏中或是否显示电脑幸存者.
				continue;

			assisters[assister_count][0] = !iBot ? i : iBot;
			assisters[assister_count][1] = GetSurvivorHP(i);
			assisters[assister_count][2] = g_iKillSpecial[!iBot ? i : iBot] > 9999 ? 9999 : g_iKillSpecial[!iBot ? i : iBot];
			assisters[assister_count][3] = g_iHeadSpecial[!iBot ? i : iBot] > 9999 ? 9999 : g_iHeadSpecial[!iBot ? i : iBot];
			assisters[assister_count][4] = g_iKillZombie[!iBot ? i : iBot] > 9999 ? 9999 : g_iKillZombie[!iBot ? i : iBot];
			assisters[assister_count][5] = g_iDmgHealth[0][!iBot ? i : iBot] > 9999 ? 9999 : g_iDmgHealth[0][!iBot ? i : iBot];
			assisters[assister_count][6] = g_iDmgHealth[1][!iBot ? i : iBot] > 9999 ? 9999 : g_iDmgHealth[1][!iBot ? i : iBot];
			assister_count+=1;
		}
	}
	//以最大击杀数排序.
	SortCustom2D(assisters, assister_count, ClientValue2DSortDesc);

	int maxStringLength = 128;
	char[][][] sData = new char[sizeof(g_sTitle)][assister_count + 1][maxStringLength];//创建动态大小的字符串.

	for (int z = 0; z < sizeof(g_sTitle); z++)
		strcopy(sData[z][0], maxStringLength, g_sTitle[z]);
	
	if (assister_count > g_iInfoRanking)
		assister_count = g_iInfoRanking;

	ArrayList aClients = new ArrayList();

	for (int x = 0; x < assister_count; x++)
	{
		int j = x + 1;
		int client	= assisters[x][0];
		int iHealth	= assisters[x][1];
		int iKill	= assisters[x][2];
		int iHead	= assisters[x][3];
		int iZombie	= assisters[x][4];
		int iHurt	= assisters[x][5];
		int iDmg	= assisters[x][6];
		
		if (IsValidClient(client))//因为要显示闲置玩家的数据,所以这里不要判断团队.
		{
			int iBot = iGetBotOfIdlePlayer(client);
			strcopy(sData[0][j],	maxStringLength, GetSurvivorStatus(iBot != 0 ? iBot : client));
			strcopy(sData[1][j],	maxStringLength, GetSurvivorDifficulty(iBot != 0 ? iBot : client));

			IntToString(iHealth,	sData[2][j], maxStringLength);
			IntToString(iKill,		sData[3][j], maxStringLength);
			IntToString(iHead,		sData[4][j], maxStringLength);
			IntToString(iZombie,	sData[5][j], maxStringLength);
			IntToString(iHurt,		sData[6][j], maxStringLength);
			IntToString(iDmg,		sData[7][j], maxStringLength);
			strcopy(sData[8][j], maxStringLength, GetTrueName(iBot != 0 ? iBot : client));

			//IntToString(999,	sData[1][j], maxStringLength);
			//IntToString(9999,	sData[2][j], maxStringLength);
			//IntToString(9999,	sData[3][j], maxStringLength);
			//IntToString(9999,	sData[4][j], maxStringLength);
			//IntToString(9999,	sData[5][j], maxStringLength);
			//IntToString(9999,	sData[6][j], maxStringLength);
			//
			//strcopy(sData[7][j], maxStringLength, "这是:一个测试信息输出.");
			aClients.Push(iBot != 0 ? iBot : client);
			ranking_count += 1;
		}
	}

	int quantity = sizeof(g_sTitle) - iValue[2];//数组减1和最后的名字减1
	for (int k = iValue[1]; k <= quantity; k++)
		temp[k - iValue[1]] = strlen(sData[k][1]);
		
	for (int j = 1; j < ranking_count; j++)
		for (int y = 0; y < sizeof(g_sTitle) - iValue[0]; y++)
			if(strlen(sData[y + iValue[1]][j]) > temp[y])
				temp[y] = strlen(sData[y + iValue[1]][j]);

	int iSlotList = RoundToCeil((ranking_count - 1) / 2.0);

	int maxlength, minlength;
	maxlength = 128 - ((strlen(g_sTitle[sizeof(g_sTitle) - 1])) + (ranking_count - 1) * 2);//计算剩余可用字符数(因为后面还有换行符所以这里需要*2)
	minlength = RoundToFloor(float(maxlength) / float(iSlotList));

	if (minlength > 32)//设置最大字符数(不建议大于32).
		minlength = 32;

	if (minlength < 14)//设置最小字符数(不建议小于14).
		minlength = 14;
	
	for (int j = 1; j < ranking_count; j++)
		strcopy(sData[sizeof(g_sTitle) - 1][j], maxStringLength, GetDealName(sData[sizeof(g_sTitle) - 1][j], minlength));
	
	//这里必须重新循环,不然数字不能对齐.
	for (int y = 1; y < ranking_count; y++)
	{
		for (int h = iValue[1]; h <= quantity; h++)
		{
			iMax[h - iValue[1]] = temp[h - iValue[1]] - strlen(sData[h][y]);
			
			if(iMax[h - iValue[1]] > 0)
				Format(sData[h][y], maxStringLength, "%s%s", GetAddSpacesMax(iMax[h - iValue[1]], " "),  sData[h][y]);//这里不能使用FormatEx
		}
	}

	char g_sInfo[sizeof(g_sTitle)][256];
	for (int y = 0; y < sizeof(g_sTitle); y++)
		ImplodeStrings(sData[y], assister_count + 1, "\n", g_sInfo[y], sizeof(g_sInfo[]));//打包字符串.
	
	float g_fPos = 0.03;
	float fValue = 0.035 * (assister_count + 1) + 0.0035 * (assister_count + 1);//根据最大行数计算所需的宽度.

	if(g_iShowServerName > 0)
		g_fPos += 0.03;
	if(g_iTypeRanking & Number_1)
	{
		HUDSetLayout(HUD_LEFT_BOT,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[0]);
		HUDPlace(HUD_LEFT_BOT, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
		iHudCoord += 1;
	}
	if(g_iTypeRanking & Number_2)
	{
		if(g_bLibraries && GetPlayerDifficultyStatus(aClients))
		{
			HUDSetLayout(HUD_MID_BOX,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[1]);
			HUDPlace(HUD_MID_BOX, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			iHudCoord += 1;
		}
	}
	if((g_iTypeRanking & Number_4))
	{
		if(g_iDispRanking == 0 || !(g_iDispRanking & Number_1))
		{
			HUDSetLayout(HUD_MID_BOT,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[2]);
			HUDPlace(HUD_MID_BOT, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			iHudCoord += 1;
		}
		else
		{
			if(GetPlayerStatus(g_sInfo[2], ranking_count))
			{
				HUDSetLayout(HUD_MID_BOT,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[2]);
				HUDPlace(HUD_MID_BOT, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
				iHudCoord += 1;
			}
		}
	}
	if((g_iTypeRanking & Number_8))
	{
		if(g_iDispRanking == 0 || !(g_iDispRanking & Number_2))
		{
			HUDSetLayout(HUD_RIGHT_TOP,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[3]);
			HUDPlace(HUD_RIGHT_TOP, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			iHudCoord += 1;
		}
		else
		{
			if(GetPlayerStatus(g_sInfo[3], ranking_count))
			{
				HUDSetLayout(HUD_RIGHT_TOP,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[3]);
				HUDPlace(HUD_RIGHT_TOP, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
				iHudCoord += 1;
			}
		}
	}
	if ((g_iTypeRanking & Number_16))
	{
		if(g_iDispRanking == 0 || !(g_iDispRanking & Number_4))
		{
			HUDSetLayout(HUD_SCORE_3,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[4]);
			HUDPlace(HUD_SCORE_3, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			iHudCoord += 1;
		}
		else
		{
			if(GetPlayerStatus(g_sInfo[4], ranking_count))
			{
				HUDSetLayout(HUD_SCORE_3,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[4]);
				HUDPlace(HUD_SCORE_3, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
				iHudCoord += 1;
			}
		}
	}
	if((g_iTypeRanking & Number_32))
	{
		if(g_iDispRanking == 0 || !(g_iDispRanking & Number_8))
		{
			HUDSetLayout(HUD_TICKER,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[5]);
			HUDPlace(HUD_TICKER, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			iHudCoord += 1;
		}
		else
		{
			if(GetPlayerStatus(g_sInfo[5], ranking_count))
			{
				HUDSetLayout(HUD_TICKER,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[5]);
				HUDPlace(HUD_TICKER, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
				iHudCoord += 1;
			}
		}
	}
	if((g_iTypeRanking & Number_64))
	{
		if(g_iDispRanking == 0 || !(g_iDispRanking & Number_16))
		{
			HUDSetLayout(HUD_FAR_RIGHT,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[6]);
			HUDPlace(HUD_FAR_RIGHT, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			iHudCoord += 1;
		}
		else
		{
			if(GetPlayerStatus(g_sInfo[6], ranking_count))
			{
				HUDSetLayout(HUD_FAR_RIGHT,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[6]);
				HUDPlace(HUD_FAR_RIGHT, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
				iHudCoord += 1;
			}
		}
	}
	if((g_iTypeRanking & Number_128))
	{
		if(g_iDispRanking == 0 || !(g_iDispRanking & Number_32))
		{
			HUDSetLayout(HUD_SCORE_2,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[7]);
			HUDPlace(HUD_SCORE_2, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			iHudCoord += 1;
		}
		else
		{
			if(GetPlayerStatus(g_sInfo[7], ranking_count))
			{
				HUDSetLayout(HUD_SCORE_2,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[7]);
				HUDPlace(HUD_SCORE_2, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
				iHudCoord += 1;
			}
		}
	}
	if(g_iTypeRanking & Number_256)
	{
		if(ranking_count <= iSlotList)
		{
			HUDSetLayout(HUD_RIGHT_BOT,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, g_sInfo[8]);
			HUDPlace(HUD_RIGHT_BOT, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
		}
		else//两个插槽拼接显示,但是有个奇怪的问题,必须15名生还者都显示的时候才能正常对齐,不然少一个生还者坐标就向下移动一点,很奇怪的问题(已解决:是因为字符串数组数量的问题,改成动态数组后成功解决了,更正:是因为字符数量超过上限导致的).
		{
			char[][] sValue = new char[ranking_count][maxStringLength];//创建动态大小的字符串.
			ExplodeString(g_sInfo[8], "\n", sValue, ranking_count, maxStringLength);//拆分字符串.
			char[][] sInfo1 = new char[ranking_count][maxStringLength];//创建动态大小的字符串数组.
			char[][] sInfo2 = new char[ranking_count][maxStringLength];//创建动态大小的字符串数组.

			for (int i = 0; i < ranking_count; i++)
			{
				if(i <= iSlotList)
				{
					strcopy(sInfo1[i], maxStringLength, sValue[i]);
					strcopy(sInfo2[i], maxStringLength, " ");
				}
				else
				{
					strcopy(sInfo1[i], maxStringLength, " ");
					strcopy(sInfo2[i], maxStringLength, sValue[i]);
				}
			}
			char sName[2][256];
			ImplodeStrings(sInfo1, ranking_count, "\n", sName[0], sizeof(sName[]));//打包字符串.
			ImplodeStrings(sInfo2, ranking_count, "\n", sName[1], sizeof(sName[]));//打包字符串.
			
			HUDSetLayout(HUD_RIGHT_BOT,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, sName[0]);
			HUDPlace(HUD_RIGHT_BOT, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			HUDSetLayout(HUD_SCORE_TITLE,HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEAM_SURVIVORS|HUD_FLAG_TEXT, sName[1]);
			HUDPlace(HUD_SCORE_TITLE, g_fCoord[iHudCoord],g_fPos,1.0,fValue);
			
			//PrintToChatAll("\x04[提示]\x05%d|%d|%d.", strlen(sName[0]), strlen(sName[1]), strlen("\n"));
		}	
			
		iHudCoord += 1;
	}
	delete aClients;
}
bool GetPlayerDifficultyStatus(ArrayList aClients)
{
	int temp;
	ArrayList aValue = new ArrayList();

	for (int i = 0; i < aClients.Length; i++)
	{
		int client = aClients.Get(i);
		if (IsClientInGame(client))
		{
			int iBot = iGetBotOfIdlePlayer(client);
			int value = GetCustomizeDifficulty(iBot != 0 ? iBot : client);
			if(value == GetGameDifficultyIndex())
				value = -1;
			aValue.Push(value);
		}
	}

	if(aValue.Length > 0)
	{
		temp = aValue.Get(0);

		for (int i = 1; i < aValue.Length; i++)
			if(aValue.Get(i) != temp)
				return true;
	}
	
	delete aValue;
	return false;
}
bool GetPlayerStatus(char[] sValue, int ranking_count)
{
	int maxStringLength = 256;
	char[][] sInfo = new char[ranking_count][maxStringLength];//更改为动态大小的数组.
	ExplodeString(sValue, "\n", sInfo, ranking_count, maxStringLength);//拆分字符串.
	for (int i = 1; i < ranking_count; i++)
		if (StringToInt(sInfo[i]) > 0)
			return true;
	return false;
}
int ClientValue2DSortDesc(int[] elem1, int[] elem2, const int[][] array, Handle hndl)
{
	if (elem1[2] > elem2[2])
		return -1;
	else if (elem2[2] > elem1[2])
		return 1;
	return 0;
}
/*
//获取玩家状态.
int GetSurvivorsStatus()
{
	for (int i = 1; i <= MaxClients; i++)
		if (IsClientInGame(i) && GetClientTeam(i) == 2 && !IsPlayerState(i))
			return 1;//有倒地的和挂边的生还者置顶显示.
	return 2;//以击杀数量排序.
}
*/
void IsPlayersNumber()
{
	int iNumber[8];
	char sLine[128], sInfo[sizeof(iNumber)][32];
	
	if(g_iPlayersNumber & Number_1)
	{
		iNumber[0] = GetConnectionNumber();
		if(iNumber[0] > 0)
			FormatEx(sInfo[0], sizeof(sInfo[]), "连接:%d ", iNumber[0]);
	}
	if(g_iPlayersNumber & Number_2)
	{
		iNumber[1] = GetPlayersStateNumber(1, true);
		if(iNumber[1] > 0)
			FormatEx(sInfo[1], sizeof(sInfo[]), "闲置:%d ", iNumber[1]);
	}
	if(g_iPlayersNumber & Number_4)
	{
		iNumber[2] = GetPlayersStateNumber(1, false);
		if(iNumber[2] > 0)
			FormatEx(sInfo[2], sizeof(sInfo[]), "旁观:%d ", iNumber[2]);
	}
	if(g_iPlayersNumber & Number_8)
	{
		iNumber[3] = GetInfectedNumber("infected");
		if(iNumber[3] > 0)
			FormatEx(sInfo[3], sizeof(sInfo[]), "丧尸:%d ", iNumber[3]);
	}
	if(g_iPlayersNumber & Number_16)
	{
		iNumber[4] = GetInfectedNumber("witch");
		if(iNumber[4] > 0)
			FormatEx(sInfo[4], sizeof(sInfo[]), "女巫:%d ", iNumber[4]);
	}
	if(g_iPlayersNumber & Number_32)
	{
		iNumber[5] = GetPlayersMaxNumber(3, false);
		if(iNumber[5] > 0)
			FormatEx(sInfo[5], sizeof(sInfo[]), "特感:%d ", iNumber[5]);
	}
	if(g_iPlayersNumber & Number_64)
	{
		iNumber[6] = GetPlayersMaxNumber(2, false);
		if(iNumber[6] > 0)
			FormatEx(sInfo[6], sizeof(sInfo[]), "生还:%d ", iNumber[6]);
	}
	
	ImplodeStrings(sInfo, sizeof(sInfo), "", sLine, sizeof(sLine));//打包字符串.
	HUDSetLayout(HUD_SCORE_4, HUD_FLAG_ALIGN_LEFT|HUD_FLAG_NOBG|HUD_FLAG_TEXT, sLine);
	HUDPlace(HUD_SCORE_4,0.00,0.00,1.0,0.03);
}

//显示当前和总人数.
void IsShowServersNumber()
{
	static char g_sTitleName[][] = {"➣路程:", "➣坦克:", "➣女巫:"};

	int iDistance[3], iValue[sizeof(iDistance)], iNumber[2];
	char sData[256], sLine[2][128], sDistance[sizeof(iDistance)][64], sNumber[sizeof(iNumber)][64];
	if(g_iShowServerNumber & Number_1)
	{
		iNumber[0] = g_iPlayerNum;
		int iMaxPlayers = GetMaxPlayers();
		if(iNumber[0] > 0)
			FormatEx(sNumber[0], sizeof(sNumber[]), "➣人数:%d/%d\n", iNumber[0], iNumber[0] > iMaxPlayers ? iNumber[0] : iMaxPlayers);
	}
	if(g_iShowServerNumber & Number_2)
	{
		iNumber[1] = g_iCurrentChapter;
		if(iNumber[1] > 0)
			FormatEx(sNumber[1], sizeof(sNumber[]), "➣地图:%d/%d\n", iNumber[1], g_iMaxChapters);
	}

	if(g_iShowServerNumber & Number_4)
		iDistance[0] = GetGameDistance();

	if(g_iShowServerNumber & Number_8)
		iDistance[1] = GetTankDistance();

	if(g_iShowServerNumber & Number_16)
		iDistance[2] = GetWitchDistance();

	int iTemp = GetMaxiMum(iDistance, sizeof(iDistance));//获取数组中的最大值.
	
	for (int i = 0; i < sizeof(iDistance); i++)
		if (iDistance[i] > 0)
			iValue[i] = GetCharacterSize(iTemp) - GetCharacterSize(iDistance[i]);
		
	for (int i = 0; i < sizeof(iDistance); i++)
		if (iDistance[i] > 0)
			FormatEx(sDistance[i], sizeof(sDistance[]), "%s%s%d％\n", g_sTitleName[i], GetAddSpacesMax(iValue[i], "0"), iDistance[i]);
		
	ImplodeStrings(sNumber, sizeof(sNumber), "", sLine[0], sizeof(sLine[]));//打包字符串.
	ImplodeStrings(sDistance, sizeof(sDistance), "", sLine[1], sizeof(sLine[]));//打包字符串.
	ImplodeStrings(sLine, sizeof(sLine), "", sData, sizeof(sData));//打包字符串.

	HUDSetLayout(HUD_SCORE_1, HUD_FLAG_BLINK|HUD_FLAG_ALIGN_RIGHT|HUD_FLAG_NOBG|HUD_FLAG_TEXT, sData);
	HUDPlace(HUD_SCORE_1,0.00,g_iShowServerName <= -1?0.035:0.06,1.0,0.22);
}

int GetMaxiMum(int[] iArray, int iNumber)
{
	int iMax;
	for (int i = 0; i < iNumber; i++)
		if (iArray[i] > 0)
			if (iArray[i] > iMax)
				iMax = iArray[i];
	
	return iMax;
}

//获取游戏路程.
int GetGameDistance()
{
	static int client;
	static float highestFlow;
	highestFlow = (client = L4D_GetHighestFlowSurvivor()) != -1 ? L4D2Direct_GetFlowDistance(client) : L4D2_GetFurthestSurvivorFlow();
	if (highestFlow)
		highestFlow = highestFlow / g_fMapMaxFlow * 100;
	return RoundToCeil(highestFlow) < 0 ? 0 : RoundToCeil(highestFlow);
}
//获取坦克路程.
int GetTankDistance()
{
	int flow, roundNumber;
	roundNumber = GetGameRulesNumber();

	if (L4D2Direct_GetVSTankToSpawnThisRound(roundNumber)) 
	{
		flow = RoundToCeil(L4D2Direct_GetVSTankFlowPercent(roundNumber) * 100.0);
		if (flow > 0) 
			flow -= RoundToFloor(g_fVsBossBuff / g_fMapMaxFlow * 100);
	}
	return flow < 0 ? 0 : flow;
}
//获取女巫路程.
int GetWitchDistance()
{
	int flow, roundNumber;
	roundNumber = GetGameRulesNumber();

	if (L4D2Direct_GetVSWitchToSpawnThisRound(roundNumber)) 
	{
		flow = RoundToCeil(L4D2Direct_GetVSWitchFlowPercent(roundNumber) * 100.0);
		if (flow > 0) 
			flow -= RoundToFloor(g_fVsBossBuff / g_fMapMaxFlow * 100);
	}
	return flow < 0 ? 0 : flow;
}
int GetGameRulesNumber()
{
	return GameRules_GetProp("m_bInSecondHalfOfRound");
}
int GetInfectedNumber(char[] g_sInfected)
{
	int inf = -1, count;
	while ((inf = FindEntityByClassname(inf, g_sInfected)) != INVALID_ENT_REFERENCE)
		count += 1;
	return count;
}

//显示服务器时间.
void IsShowServerTime()
{
	char sDate[3][128], sInfo[256], sTime[256];
	if(g_iShowServerTime & Number_1)
		FormatTime(sDate[0], sizeof(sDate[]), "%Y-%m-%d");
	if(g_iShowServerTime & Number_4)
		FormatTime(sDate[1], sizeof(sDate[]), "%H:%M:%S");
	if(g_iShowServerTime & Number_8)
		FormatEx(sDate[2], sizeof(sDate[]), "星期%s", IsWeekName());
	ImplodeStrings(sDate, sizeof(sDate), " ", sInfo, sizeof(sInfo));//打包字符串.
	FormatEx(sTime, sizeof(sTime), "%s%s", sInfo, GetAddSpacesMax(3, " "));
	HUDSetLayout(HUD_MID_TOP, HUD_FLAG_ALIGN_RIGHT|HUD_FLAG_NOBG|HUD_FLAG_TEXT, sTime);
	HUDPlace(HUD_MID_TOP,0.00,0.00,1.0,0.03);
}

//填入对应数量的内容.
char[] GetAddSpacesMax(int Value, char[] sContent)
{
	char g_sBlank[64];
	
	if(Value > 0)
	{
		char g_sFill[32][64];
		if(Value > sizeof(g_sFill))
			Value = sizeof(g_sFill);
		for (int i = 0; i < Value; i++)
			strcopy(g_sFill[i], sizeof(g_sFill[]), sContent);
		ImplodeStrings(g_sFill, sizeof(g_sFill), "", g_sBlank, sizeof(g_sBlank));//打包字符串.
	}
	return g_sBlank;
}

//返回对应的内容.
char[] GetTrueName(int client)
{
	char sName[64];
	int Bot = IsClientIdle(client);
	
	if(Bot != 0)
		FormatEx(sName, sizeof(sName), "闲置:%N", Bot);
	else
		GetClientName(client, sName, sizeof(sName));

	ReplaceString(sName, sizeof(sName), "\n", "");//把换行符替换为空.
	ReplaceString(sName, sizeof(sName), "\r", "");//把换行符替换为空.

	return sName;
}
//返回处理的内容.
char[] GetDealName(char[] buffer, int minlength)
{
	char sName[64];//定义最大显示字符数量(不建议设置太大或太小,建议最小14最大32即可).
	Format(sName, sizeof(sName), "%s", IsSplitStrings(buffer, minlength));
	//PrintToChatAll("测试信息:当前字符数(%d)", minlength);
	return sName;//新增防止玩家名字被截断时出现乱码.
}
//防止玩家名字被截断时出现乱码(部分符号可能会有点小问题)(此代码嫖至QAQ大佬！！！！！)
char[] IsSplitStrings(char[] src, int maxsize)
{
	ArrayList hData = new ArrayList(ByteCountToCells(32));
	int lengthsize;
	char sInfo[32];

	for(int i=0;i<strlen(src);)
	{
		if(0x00<=src[i] && src[i]<=0x7f)
		{
			sInfo[0] = '\0';
			lengthsize += strcopy(sInfo, sizeof(sInfo), GetSplitString(src,i,1));
			if(lengthsize > maxsize)
				break;
			hData.PushString(sInfo);
			i+=1;
		}
		else if(0xC0<=src[i] && src[i]<=0xDf)
		{
			sInfo[0] = '\0';
			lengthsize += strcopy(sInfo, sizeof(sInfo), GetSplitString(src,i,2));
			if(lengthsize > maxsize)
				break;
			hData.PushString(sInfo);
			i+=2;
		}
		else if(0xE0<=src[i] && src[i]<=0xEf)
		{
			sInfo[0] = '\0';
			lengthsize += strcopy(sInfo, sizeof(sInfo), GetSplitString(src,i,3));
			if(lengthsize > maxsize)
				break;
			hData.PushString(sInfo);
			i+=3;
		}
		else if(0xF0<=src[i] && src[i]<=0xF7)
		{
			sInfo[0] = '\0';
			lengthsize += strcopy(sInfo, sizeof(sInfo), GetSplitString(src,i,4));
			if(lengthsize > maxsize)
				break;
			hData.PushString(sInfo);
			i+=4;
		}
	}
	
	char sString[256];

	if(hData.Length > 0)
	{
		char[][] sTemp = new char[hData.Length][32];
		for(int i = 0; i < hData.Length; i++)
			hData.GetString(i, sTemp[i], 32);

		ImplodeStrings(sTemp, hData.Length, "", sString, sizeof(sString));//打包字符串.
	}

	delete hData;
	return sString;
}

char[] GetSplitString(char[] src, int srcindex, int length)
{
	char temp[10];
	for(int i = 0; i < length; i++)
	{
		temp[i] = src[i + srcindex];
		if(i + srcindex >= strlen(src))
			break;
	}
	return temp;
	//PrintToChatAll("\x04[提示]\x05当前内容:(%s).", temp);
}

//返回当前星期几.
char[] IsWeekName()
{
	char g_sWeek[8];
	FormatTime(g_sWeek, sizeof(g_sWeek), "%u");
	return g_sWeekName[StringToInt(g_sWeek) - 1];
}
//返回玩家状态.
char[] GetSurvivorStatus(int client)
{
	char g_sStatus[8];
	if(IsReviveCount(client))//判断是否黑白.
	{
		if (!IsPlayerAlive(client))
			strcopy(g_sStatus, sizeof(g_sStatus), "死亡");
		else
			strcopy(g_sStatus, sizeof(g_sStatus), GetSurvivorHP(client) < g_iSurvivorHealth ? "濒死" : g_iMaxReviveCount <= 0 ? "正常" : "黑白");
	}
	else
		if (!IsPlayerAlive(client))
			strcopy(g_sStatus, sizeof(g_sStatus), "死亡");
		else if (IsPlayerFallen(client))
			strcopy(g_sStatus, sizeof(g_sStatus), "倒地");
		else if (IsPlayerFalling(client))
			strcopy(g_sStatus, sizeof(g_sStatus), "挂边");
		else
			strcopy(g_sStatus, sizeof(g_sStatus), GetSurvivorHP(client) < g_iSurvivorHealth ? g_iMaxReviveCount <= 0 ? "濒死" : "瘸腿" : "正常");
	return g_sStatus;
}
bool IsReviveCount(int client)
{
	return GetEntProp(client, Prop_Send, "m_currentReviveCount") >= g_iMaxReviveCount;
}
//返回生还者难度.
char[] GetSurvivorDifficulty(int client)
{
	char sDifficulty[8];
	if(g_bLibraries)
	{
		int value = GetCustomizeDifficulty(client);
		strcopy(sDifficulty, sizeof(sDifficulty), g_sDifficultyName[value == -1 ? GetGameDifficultyIndex() : value]);
	}
	strcopy(sDifficulty, sizeof(sDifficulty), sDifficulty[0] != '\0' ? sDifficulty : GetGameDifficultyName());
	return sDifficulty;
}
stock int GetGameDifficultyIndex()
{
	char sDifficulty[32];
	GetConVarString(FindConVar("z_Difficulty"), sDifficulty, sizeof(sDifficulty));

	for (int i = 0; i < sizeof(g_sDifficultyCode); i++)
		if(strcmp(g_sDifficultyCode[i], sDifficulty, false) == 0)
			return i;
	return -1;
}
stock char[] GetGameDifficultyName()
{
	char sDifficulty[32];
	GetConVarString(FindConVar("z_Difficulty"), sDifficulty, sizeof(sDifficulty));
	return sDifficulty;
}
//返回服务器名字.
char[] GetHostName()
{
	char g_sHostName[256];
	g_hHostName.GetString(g_sHostName, sizeof(g_sHostName));
	return g_sHostName;
}

//返回字符串实际大小.
int GetCharacterSize(int g_iSize)
{
	char sChapter[64];
	IntToString(g_iSize, sChapter, sizeof(sChapter));//格式化int类型为char类型.
	return strlen(sChapter);
}

//幸存者总血量.
int GetSurvivorHP(int client)
{
	int HP = GetClientHealth(client) + GetPlayerTempHealth(client);
	return IsPlayerAlive(client) ? HP > 999 ? 999 : HP : 0;//如果幸存者血量大于999就显示为999
}

//幸存者虚血量.
int GetPlayerTempHealth(int client)
{
	static Handle painPillsDecayCvar;
	painPillsDecayCvar = FindConVar("pain_pills_decay_rate");
	if (painPillsDecayCvar == null)
		return -1;

	int tempHealth = RoundToCeil(GetEntPropFloat(client, Prop_Send, "m_healthBuffer") - ((GetGameTime() - GetEntPropFloat(client, Prop_Send, "m_healthBufferTime")) * GetConVarFloat(painPillsDecayCvar))) - 1;
	return tempHealth < 0 ? 0 : tempHealth;
}

//获取正在连接的玩家数量.
int GetConnectionNumber()
{
	int count = 0;
	for (int i = 1; i <= MaxClients; i++)
		if (IsClientConnected(i) && !IsClientInGame(i) && !IsFakeClient(i))
			count += 1;
	
	return count;
}

//获取闲置或旁观者数量.
int GetPlayersStateNumber(int iTeam, bool bClientTeam)
{
	int count = 0;
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientInGame(i) && !IsFakeClient(i) && GetClientTeam(i) == iTeam)
		{
			if (bClientTeam)
			{
				if (iGetBotOfIdlePlayer(i))
					count += 1;
			}
			else
			{
				if (!iGetBotOfIdlePlayer(i))
					count += 1;
			}
		}
	}
	return count;
}

//获取特感或幸存者数量.
int GetPlayersMaxNumber(int iTeam, bool bSurvive)
{
	int count = 0;
	for (int i = 1; i <= MaxClients; i++)
		if (IsClientInGame(i) && GetClientTeam(i) == iTeam)
			if (bSurvive)
			{
				if(IsPlayerAlive(i))
					count += 1;
			}
			else
				count += 1;
	
	return count;
}

//返回最大人数.
int GetMaxPlayers()
{
	static Handle hMaxPlayers;
	hMaxPlayers = FindConVar("sv_maxplayers");
	if (hMaxPlayers == null)
		return GetDefaultNumber();
		
	int iMaxPlayers = GetConVarInt(hMaxPlayers);
	if(iMaxPlayers <= -1)
		return GetDefaultNumber();
	
	return iMaxPlayers;
}

int GetDefaultNumber()
{
	for (int i = 0; i < sizeof(g_sModeVersus); i++)
		if(strcmp(GetGameMode(), g_sModeVersus[i]) == 0)
			return 8;
	for (int i = 0; i < sizeof(g_sModeSingle); i++)
		if(strcmp(GetGameMode(), g_sModeSingle[i]) == 0)
			return 1;
	return 4;
}

char[] GetGameMode()
{
	char g_sMode[32];
	GetConVarString(FindConVar("mp_gamemode"), g_sMode, sizeof(g_sMode));
	return g_sMode;
}

//返回闲置玩家对应的电脑.
int iGetBotOfIdlePlayer(int client)
{
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientInGame(i) && IsFakeClient(i) && GetClientTeam(i) == 2 && IsClientIdle(i) == client)
			return i;
	}
	return 0;
}

//返回电脑幸存者对应的玩家.
int IsClientIdle(int client)
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}

//正常状态.
stock bool IsPlayerState(int client)
{
	return !GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}

//挂边状态.
bool IsPlayerFalling(int client)
{
	return GetEntProp(client, Prop_Send, "m_isIncapacitated") && GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}

//倒地状态.
bool IsPlayerFallen(int client)
{
	return GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}

bool bCheckClientAccess(int client)
{
	if(GetUserFlagBits(client) & ADMFLAG_ROOT)
		return true;
	return false;
}

//判断玩家有效.
bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}