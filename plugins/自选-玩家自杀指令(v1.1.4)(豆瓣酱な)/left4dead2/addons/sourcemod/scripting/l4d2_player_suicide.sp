/*
 * v1.1.3
 *	
 *	1:掉落钞票插件的函数库名称更改了.
 *
 * v1.1.4
 *	
 *	1:修复依赖插件被卸载时此插件会被暂停的问题.
 *
 */

#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#include <sdktools>
#undef REQUIRE_PLUGIN	//标记为可选开始.
#include <l4d2_DropMoney>//生还者丢钱插件的API函数.
#define REQUIRE_PLUGIN	//标记为可选结束.

#define PLUGIN_VERSION "1.1.4"

bool g_bPointsFall;

int    g_iSurvivor, g_iInfected, g_iShowTips;
ConVar g_hSurvivor, g_hInfected, g_hShowTips;

public Plugin myinfo =  
{
	name = "l4d2_player_suicide",
	author = "豆瓣酱な",  
	description = "玩家自杀指令",
	version = PLUGIN_VERSION,
	url = "N/A"
};
//所有插件加载成功后.
public void OnAllPluginsLoaded()
{
	g_bPointsFall = LibraryExists("l4d2_DropMoney");
}
//库被加载时.
public void OnLibraryAdded(const char[] name) 
{
	if (strcmp(name, "l4d2_DropMoney") == 0)
		g_bPointsFall = true;
}
//库被卸载时.
public void OnLibraryRemoved(const char[] name) 
{
	if (strcmp(name, "l4d2_DropMoney") == 0)
		g_bPointsFall = false;
}
//插件开始时.
public void OnPluginStart()
{
	RegConsoleCmd("sm_zs", Command_Suicide, "玩家自杀指令.");
	RegConsoleCmd("sm_kill", Command_Suicide, "玩家自杀指令.");
	
	g_hSurvivor	= CreateConVar("l4d2_survivor_suicide",		"1", "启用生还者自杀功能. 0=禁用, 1=只限倒地或挂边, 2=无条件使用.");
	g_hInfected	= CreateConVar("l4d2_infected_suicide",		"1", "启用感染者自杀功能. 0=禁用, 1=只限非灵魂状态, 2=无条件使用.");
	g_hShowTips	= CreateConVar("l4d2_command_hint",			"7", "设置开局提示自杀指令的延迟显示时间/秒. 0=禁用.");
	g_hSurvivor.AddChangeHook(IsSuicideConVarChanged);
	g_hInfected.AddChangeHook(IsSuicideConVarChanged);
	g_hShowTips.AddChangeHook(IsSuicideConVarChanged);
	AutoExecConfig(true, "l4d2_player_suicide");
}
public void OnConfigsExecuted()
{	
	IsConVarSuicide();
}
public void IsSuicideConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	IsConVarSuicide();
}
void IsConVarSuicide()
{
	g_iSurvivor = g_hSurvivor.IntValue;
	g_iInfected = g_hInfected.IntValue;
	g_iShowTips = g_hShowTips.IntValue;
}
//聊天窗中文指令.
public Action OnClientSayCommand(int client, const char[] commnad, const char[] args)
{
	if(strlen(args) <= 1 || strncmp(commnad, "say", 3, false) != 0)
		return Plugin_Continue;

	if (StrEqual(args, "自杀", false))
		RequestFrame(IsFrameSuicide, GetClientUserId(client));
	return Plugin_Continue;
}
//玩家连接成功.
public void OnClientPostAdminCheck(int client)
{
	if (!IsFakeClient(client) && g_iShowTips > 0)
		CreateTimer(float(g_iShowTips), IsShowTipsTimer, GetClientUserId(client), TIMER_FLAG_NO_MAPCHANGE);
}
//计时器回调.
public Action IsShowTipsTimer(Handle timer, any client)
{
	if ((client = GetClientOfUserId(client)))
	{
		if (IsClientInGame(client))
		{
			switch (GetClientTeam(client))
			{
				case 1:
				{
					int iBot = IsGetBotOfIdlePlayer(client);
					if (iBot != 0)
					{
						switch (g_iSurvivor)
						{
							case 1:
								PrintToChat(client, "\x04[提示]\x05倒地或挂边输入指令\x03!zs\x05或\x03!kill\x05或\x04(\x03自杀\x04)\x05可自杀.");//聊天窗提示.
							case 2:
								PrintToChat(client, "\x04[提示]\x05输入指令\x03!zs\x05或\x03!kill\x05或\x03自杀\x05可自杀.");//聊天窗提示.
							default:
								PrintToChat(client, "\x04[提示]\x05输入指令\x03!zs\x05或\x03!kill\x05或\x03自杀\x05可自杀.");//聊天窗提示.
						}
					}
					else
						PrintToChat(client, "\x04[提示]\x05输入指令\x03!zs\x05或\x03!kill\x05或\x03自杀\x05可自杀.");//聊天窗提示.
				}
				case 2,4:
				{
					switch (g_iSurvivor)
					{
						case 1:
							PrintToChat(client, "\x04[提示]\x05倒地或挂边输入指令\x03!zs\x05或\x03!kill\x05或\x04(\x03自杀\x04)\x05可自杀.");//聊天窗提示.
						case 2:
							PrintToChat(client, "\x04[提示]\x05输入指令\x03!zs\x05或\x03!kill\x05或\x03自杀\x05可自杀.");//聊天窗提示.
						default:
							PrintToChat(client, "\x04[提示]\x05输入指令\x03!zs\x05或\x03!kill\x05或\x03自杀\x05可自杀.");//聊天窗提示.
					}
				}
				case 3:
				{
					switch (g_iInfected)
					{
						case 1:
							PrintToChat(client, "\x04[提示]\x05非灵魂状态输入指令\x03!zs\x05或\x03!kill\x05或\x04(\x03自杀\x04)\x05可自杀.");//聊天窗提示.
						case 2:
							PrintToChat(client, "\x04[提示]\x05输入指令\x03!zs\x05或\x03!kill\x05或\x03自杀\x05可自杀.");//聊天窗提示.
						default:
							PrintToChat(client, "\x04[提示]\x05输入指令\x03!zs\x05或\x03!kill\x05或\x03自杀\x05可自杀.");//聊天窗提示.
					}
				}
			}
		}
	}
	return Plugin_Stop;
}

public Action Command_Suicide(int client, int args)
{
	RequestFrame(IsFrameSuicide, GetClientUserId(client));
	return Plugin_Handled;
}

void IsFrameSuicide(int client)
{
	if ((client = GetClientOfUserId(client)))
	{
		if(IsClientInGame(client) && !IsFakeClient(client))
		{
			switch (GetClientTeam(client))
			{
				case 1:
				{
					int iBot = IsGetBotOfIdlePlayer(client);
					if (iBot != 0)
						if(g_iSurvivor > 0)
							IsRegSuicide(client);
						else
							PrintToChat(client, "\x04[提示]\x05生还者自杀指令未启用.");
					else
						PrintToChat(client, "\x04[提示]\x05旁观者无权使用该指令.");
				}
				case 2:
				{
					if(g_iSurvivor > 0)
						IsRegSuicide(client);
					else
						PrintToChat(client, "\x04[提示]\x05生还者自杀指令未启用.");
				}
				case 3:
				{
					if(g_iInfected > 0)
						IsRegSuicide(client);
					else
						PrintToChat(client, "\x04[提示]\x05感染者自杀指令未启用.");
				}
			}
		}
	}
}

void IsRegSuicide(int client)
{
	if(IsValidClient(client) && !IsFakeClient(client))
	{
		switch (GetClientTeam(client))
		{
			case 1:
			{
				int iBot = IsGetBotOfIdlePlayer(client);
				if (iBot != 0)
					IsSurvivorSuicide(iBot, client, GetTrueName(client), "生还者");
				else
					PrintToChat(client, "\x04[提示]\x05旁观者无权使用该指令.");
			}
			case 2:
				IsSurvivorSuicide(client, client, GetTrueName(client), "生还者");
			case 3:
				IsInfectedSuicide(client, client, GetTrueName(client), "感染者");
			case 4:
				IsSurvivorSuicide(client, client, GetTrueName(client), "生还者");
		}
	}
}
//生还者.
void IsSurvivorSuicide(int client, int victim, char[] sName, char[] sTeam)
{
	if (IsPlayerAlive(client))
	{
		switch (g_iSurvivor)
		{
			case 1:
			{
				if (!IsPlayerState(client))
				{
					if(g_bPointsFall)
						SuicideFallPoints(client);//幸存者自杀代码.
					else
						ForcePlayerSuicide(client);//幸存者自杀代码.
					PrintToChatAll("\x04[提示]\x05(\x04%s\x05)\x03%s\x05突然失去了梦想.", sTeam, sName);
				}
				else
					PrintToChat(victim, "\x04[提示]\x05该指令只限倒地或挂边的%s使用.", sTeam);
			}
			case 2:
			{
				if(g_bPointsFall)
					SuicideFallPoints(client);//幸存者自杀代码.
				else
					ForcePlayerSuicide(client);//幸存者自杀代码.
				PrintToChatAll("\x04[提示]\x05(\x04%s\x05)\x03%s\x05突然失去了梦想.", sTeam, sName);
			}
		}
	}
	else
		PrintToChat(victim, "\x04[提示]\x05你当前已是死亡状态.");
}
//感染者.
void IsInfectedSuicide(int client, int victim, char[] sName, char[] sTeam)
{
	if (IsPlayerAlive(client))
	{
		switch (g_iInfected)
		{
			case 1:
			{
				if (!GetEntProp(client, Prop_Send, "m_isGhost"))
				{
					ForcePlayerSuicide(client);//幸存者自杀代码.
					PrintToChatAll("\x04[提示]\x05(\x04%s\x05)\x03%s\x05突然失去了梦想.", sTeam, sName);
				}
				else
					PrintToChat(victim, "\x04[提示]\x05灵魂状态禁止使用该指令.");
			}
			case 2:
			{
				ForcePlayerSuicide(client);//幸存者自杀代码.
				PrintToChatAll("\x04[提示]\x05(\x04%s\x05)\x03%s\x05突然失去了梦想.", sTeam, sName);
			}
		}
	}
	else
		PrintToChat(victim, "\x04[提示]\x05你当前已是死亡状态.");
}
//玩家有效.
stock bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}
//正常状态.
stock bool IsPlayerState(int client)
{
	return !GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}
//返回对应的内容.
char[] GetTrueName(int client)
{
	char sName[32];
	GetClientName(client, sName, sizeof(sName));
	return sName;
}
//返回闲置玩家对应的电脑.
int IsGetBotOfIdlePlayer(int client)
{
	for (int i = 1; i <= MaxClients; i++)
		if (IsClientInGame(i) && IsFakeClient(i) && GetClientTeam(i) == 2 && IsClientIdle(i) == client)
			return i;

	return 0;
}
//返回电脑幸存者对应的玩家.
int IsClientIdle(int client)
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}