#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>

#define PLUGIN_VERSION	"1.0.1"

char sRunTime[32];

ConVar Kickafk, Switch, Kickplayer, Kickadmin;
int g_Kickafk, g_Switch;
float g_kickplayer, g_kickadmin;

bool l4d2_timer_kick;
bool l4d2_kickafk = false;

int KickLookOnPlayer[MAXPLAYERS+1];
Handle kickPlayerTimer[MAXPLAYERS+1];

public Plugin myinfo = 
{
	name 			= "l4d2_kick_afk_player",
	author 			= "豆瓣酱な",
	description 	= "防止玩家或管理员长时间挂机",
	version 		= PLUGIN_VERSION,
	url 			= "N/A"
}

public void OnPluginStart()
{
	RegConsoleCmd("sm_timer", l4d2_timer_kick_switch, "管理员开启或关闭普通玩家闲置超时踢出.");
	
	Kickafk		= CreateConVar("l4d2_afk_kick", "1", "启用普通玩家或管理员闲置超时踢出? (总开关,禁用后指令开关也不可用) 0=禁用, 1=启用(管理员免疫), 2=启用(包括管理员).", FCVAR_NOTIFY);
	Switch		= CreateConVar("l4d2_afk_kick_switch", "1", "设置默认开启或关闭自动踢出玩家或管理员? (输入指令 !timer 关闭或开启) 0=关闭, 1=开启.", FCVAR_NOTIFY);
	Kickplayer	= CreateConVar("l4d2_afk_player_kick_time","300","设置普通玩家闲置超时踢出的时间/秒(不建议低于300秒).", FCVAR_NOTIFY);
	Kickadmin	= CreateConVar("l4d2_afk_player_kick_time_admin","500","设置管理员闲置超时踢出的时间/秒(不建议低于300秒).", FCVAR_NOTIFY);
	
	Kickafk.AddChangeHook(CVARChanged);
	Switch.AddChangeHook(CVARChanged);
	Kickplayer.AddChangeHook(CVARChanged);
	Kickadmin.AddChangeHook(CVARChanged);
	
	HookEvent("player_team", Event_playerteam);//玩家转换队伍.
	HookEvent("player_disconnect", Event_Player_disconnect_kickafk);//玩家离开.
	HookEvent("round_end", Event_Round_End_kickafk);//回合结束.
	
	AutoExecConfig(true, "l4d2_kick_afk_player");//生成指定文件名的CFG.
}

public void OnMapStart()
{
	l4d2_GetKickafkCvars();
}

public void CVARChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	l4d2_GetKickafkCvars();
}

void l4d2_GetKickafkCvars()
{
	g_Kickafk = Kickafk.IntValue;
	g_Switch = Switch.IntValue;
	g_kickplayer = Kickplayer.FloatValue;
	g_kickadmin = Kickadmin.FloatValue;
}

public void OnConfigsExecuted()
{
	if(!l4d2_kickafk)
	{
		switch(g_Switch)
		{
			case 0:
				l4d2_timer_kick = false;
			case 1:
				l4d2_timer_kick = true;
		}
	}
}

public Action l4d2_timer_kick_switch(int client, int args)
{
	if(bCheckClientAccess(client))
	{
		switch(g_Kickafk)
		{
			case 0:
			{
				PrintToChat(client, "\x04[提示]\x05普通玩家闲置超时踢出已禁用,请在CFG中设为1启用.");
			}
			case 1,2:
			{
				if(l4d2_timer_kick)
				{
					OnMapEnd();
					l4d2_kickafk = true;
					l4d2_timer_kick = false;
					
					if(g_Kickafk == 1)
						PrintToChatAll("\x04[提示]\x03已关闭\x05普通玩家闲置超时踢出.");
					else if(g_Kickafk == 2)
						PrintToChatAll("\x04[提示]\x03已关闭\x05普通玩家和管理员闲置超时踢出.");
				}
				else
				{
					forkickafkplayer();
					l4d2_kickafk = true;
					l4d2_timer_kick = true;
					
					if(g_Kickafk == 1)
						PrintToChatAll("\x04[提示]\x03已开启\x05普通玩家闲置超时踢出.");
					else if(g_Kickafk == 2)
						PrintToChatAll("\x04[提示]\x03已开启\x05普通玩家和管理员闲置超时踢出.");
				}
			}
		}
	}
	else
		PrintToChat(client, "\x04[提示]\x05你无权使用此指令.");
	return Plugin_Handled;
}

//玩家离开
public void Event_Player_disconnect_kickafk(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	
	if(client && !IsFakeClient(client))
	{
		delete kickPlayerTimer[client];
	}
}

public void Event_Round_End_kickafk(Event event, const char[] name, bool dontBroadcast)
{
	OnMapEnd();
}

public void OnMapEnd()
{
	for (int i = 1; i <= MaxClients; i++)
	{
		delete kickPlayerTimer[i];
	}
}


void forkickafkplayer()
{
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientConnected(i) && IsClientInGame(i) && !IsFakeClient(i) && GetClientTeam(i) == 1)
		{
			kickafkplayer(i);
		}
	}
}

public void Event_playerteam(Event event, const char[] name, bool dontBroadcast) 
{
	int newteam = GetEventInt(event, "team");
	int client = GetClientOfUserId(event.GetInt("userid"));
	
	if (g_Kickafk != 0 && l4d2_timer_kick)
	{
		if (client && IsClientConnected(client) && IsClientInGame(client) && !IsFakeClient(client))
		{
			if (newteam == 1)
				kickafkplayer(client);
			else
				delete kickPlayerTimer[client];
		}
	}
}

void kickafkplayer(int client)
{
	KickLookOnPlayer[client] = 0;
	delete kickPlayerTimer[client];
	kickPlayerTimer[client] = CreateTimer(1.0, Timer_KickLookOnPlayer, GetClientUserId(client), TIMER_REPEAT);
}

public Action Timer_KickLookOnPlayer(Handle timer, any client)
{
	if ((client = GetClientOfUserId(client)) && l4d2_timer_kick)
	{
		if (GetClientTeam(client) != 1)
		{
			kickPlayerTimer[client] = null;
			KickLookOnPlayer[client] = 0;
			return Plugin_Stop;
		}
		switch (g_Kickafk)
		{
			case 1:
			{
				if(bCheckClientAccess(client))
				{
					KickLookOnPlayer[client] = 0;
					kickPlayerTimer[client] = null;
					return Plugin_Stop;
				}
				else
				{
					if (KickLookOnPlayer[client] >= g_kickplayer)
					{
						StandardizeTime(g_kickplayer, sRunTime);
						KickClient(client, "服务器自动踢出闲置超过 %s 的玩家", sRunTime);
						PrintToChatAll("\x04[提示]\x03%N\x05闲置超过\x03%s\x05而被服务器踢出.", client, sRunTime);//聊天窗提示
						kickPlayerTimer[client] = null;
						KickLookOnPlayer[client] = 0;
						return Plugin_Stop;
					}
					else
					{
						StandardizeTime(g_kickplayer - KickLookOnPlayer[client], sRunTime);
						PrintHintText(client, "你将会在 %s 后被踢出游戏.", sRunTime);//屏幕中下提示.
					}
					KickLookOnPlayer[client]++;
				}
			}
			case 2:
			{
				if(bCheckClientAccess(client))
				{
					if (KickLookOnPlayer[client] >= g_kickadmin)
					{
						StandardizeTime(g_kickadmin, sRunTime);
						KickClient(client, "服务器自动踢出闲置超过 %s 的管理员", sRunTime);
						PrintToChatAll("\x04[提示]\x03%N\x05闲置超过\x03%s\x05而被服务器踢出.", client, sRunTime);//聊天窗提示
						kickPlayerTimer[client] = null;
						KickLookOnPlayer[client] = 0;
						return Plugin_Stop;
					}
					else
					{
						StandardizeTime(g_kickadmin - KickLookOnPlayer[client], sRunTime);
						PrintHintText(client, "你将会在 %s 后被踢出游戏.", sRunTime);//屏幕中下提示.
					}
				}
				else
				{
					if (KickLookOnPlayer[client] >= g_kickplayer)
					{
						StandardizeTime(g_kickplayer, sRunTime);
						KickClient(client, "服务器自动踢出闲置超过 %s 的玩家", sRunTime);
						PrintToChatAll("\x04[提示]\x03%N\x05闲置超过\x03%s\x05而被服务器踢出.", client, sRunTime);//聊天窗提示
						kickPlayerTimer[client] = null;
						KickLookOnPlayer[client] = 0;
						return Plugin_Stop;
					}
					else
					{
						StandardizeTime(g_kickplayer - KickLookOnPlayer[client], sRunTime);
						PrintHintText(client, "你将会在 %s 后被踢出游戏.", sRunTime);//屏幕中下提示.
					}
				}
				KickLookOnPlayer[client]++;
			}
		}
	}
	return Plugin_Continue;
}

bool bCheckClientAccess(int client)
{
	if(GetUserFlagBits(client) & ADMFLAG_ROOT)
		return true;
	return false;
}

//https://forums.alliedmods.net/showthread.php?t=288686
void StandardizeTime(float time, char str[32])
{
	char sD[32], sH[32], sM[32], sS[32];
	float remainder = time;

	int D = RoundToFloor(remainder / 86400.0);
	remainder = remainder - float(D * 86400);
	int H = RoundToFloor(remainder / 3600.0);
	remainder = remainder - float(H * 3600);
	int M = RoundToFloor(remainder / 60.0);
	remainder = remainder - float(M * 60);
	int S = RoundToFloor(remainder);

	Format(sD, sizeof(sD), "%d天", D);
	Format(sH, sizeof(sH), "%d%s", H, !D && !M && !S ? "小时" : "时");
	Format(sM, sizeof(sM), "%d%s", M, !D && !H && !S ? "分钟" : "分");
	Format(sS, sizeof(sS), "%d秒", S);
	FormatEx(str, sizeof(str), "%s%s%s%s", !D ? "" : sD, !H ? "" : sH, !M ? "" : sM, !S ? "" : sS);
}