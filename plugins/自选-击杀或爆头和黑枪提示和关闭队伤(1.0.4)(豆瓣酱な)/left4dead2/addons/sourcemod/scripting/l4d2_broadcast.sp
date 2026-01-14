/*
 *
 *	v1.0.0
 *
 *	1:初始版本(原作者的版本有些地方看着有点奇怪,我干脆重新一下算了).
 *
 *	v1.0.1
 *
 *	1:修复条件少写了个导致计时器句柄错误.
 *
 *	v1.0.2
 *
 *	1:修复间接伤害(例如间接火)没有正确判断的问题(因为间接伤害攻击者不是玩家,所以地图火也会被过滤伤害).
 *
 *	v1.0.3
 *
 *	1:修复玩家点火后立即闲置导致不算队伤的问题(闲置火引燃的其它火或爆炸物可能不算友伤).
 *
 *	v1.0.4
 *
 *	1:尝试修复计时器报错问题.
 *
 */
#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#include <sdkhooks>

//定义插件版本.
#define PLUGIN_VERSION	"1.0.4"
//榴弹发射器伤害(有燃烧火高爆弹药时).
#define	DMG_OTHER		(1 << 30)

#define	Number_1		(1 << 0)
#define Number_2		(1 << 1)
#define	Number_4		(1 << 2)
#define Number_8		(1 << 3)

int    g_iBroadcast, g_iPlayerHurt, g_iTakeDamage;
ConVar g_hBroadcast, g_hPlayerHurt, g_hTakeDamage;

bool g_bPlayerHurt[MAXPLAYERS+1][MAXPLAYERS+1];

int g_iKillCounts[MAXPLAYERS+1] = {0,...};
int g_iHeadCounts[MAXPLAYERS+1] = {0,...};

Handle g_hKillTimer[MAXPLAYERS+1];
Handle g_hHeadTimer[MAXPLAYERS+1];

char g_sHitName[][] = 
{
	"",
	"的\x05头部",
	"的\x05胸部",
	"的\x05腹部",
	"的\x05左手",
	"的\x05右手",
	"的\x05左脚",
	"的\x05右脚"
};
//定义插件信息.
public Plugin myinfo =  
{
	name = "l4d2_broadcast",
	author = "豆瓣酱な",
	description = "显示击杀和爆头提示",
	version = PLUGIN_VERSION,
	url = "N/A"
};
//插件开始时.
public void OnPluginStart() 
{
	HookEvent("player_hurt", Event_PlayerHurt);
	HookEvent("weapon_fire", Event_WeaponFire, EventHookMode_Pre);
	HookEvent("player_death", Event_PlayerDeath, EventHookMode_Pre);
	
	g_hBroadcast = CreateConVar("l4d2_broadcast_player_death", "3", "击杀或爆头提示? 0=禁用, 1=击杀时, 2=爆头时, 3=全部.", FCVAR_NOTIFY);
	g_hPlayerHurt = CreateConVar("l4d2_broadcast_player_hurt", "1", "幸存者黑枪提示? 0=禁用, 1=启用.", FCVAR_NOTIFY);
	g_hTakeDamage = CreateConVar("l4d2_broadcast_take_damage", "15", "幸存者友伤功能? 0=禁用, 1=队伤(玩家火和爆炸伤害也是队伤范围), 2=火伤(只限地图火或间接火), 4=爆炸类(只限非玩家引爆)(土制炸弹,煤气罐,氧气罐), 8=榴弹发射器(闲置榴弹), 15=全部.", FCVAR_NOTIFY);
	g_hBroadcast.AddChangeHook(ConVarChanged);
	g_hPlayerHurt.AddChangeHook(ConVarChanged);
	g_hTakeDamage.AddChangeHook(ConVarChanged);
	AutoExecConfig(true, "l4d2_broadcast");//生成指定文件名的CFG.
}
//
public void OnConfigsExecuted()
{
	GetCvars();
}
public void ConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	GetCvars();
}
void GetCvars()
{
	g_iBroadcast = g_hBroadcast.IntValue;
	g_iPlayerHurt = g_hPlayerHurt.IntValue;
	g_iTakeDamage = g_hTakeDamage.IntValue;
}
public void OnClientPutInServer(int client)
{
	SDKHook(client, SDKHook_OnTakeDamage, OnTakeDamage);
}
//SDK回调.
public Action OnTakeDamage(int client, int &attacker, int &inflictor, float &damage, int &damagetype)
{ 
	if(g_iTakeDamage > 0)
	{
		if(IsValidClient(client) && GetClientTeam(client) == 2)
		{
			if(IsValidClient(attacker) && g_iTakeDamage & Number_1)
			{
				switch(GetClientTeam(attacker))
				{
					case 1:
					{
						int iBot = iGetBotOfIdlePlayer(attacker);//返回闲置玩家对应的电脑.
						if(iBot != 0)//玩家是闲置状态,反之是旁观者.
							return Plugin_Handled;//阻止伤害.
					}
					case 2:
					{
						return Plugin_Handled;//阻止伤害.
					}
				}
			}
			if(damagetype & DMG_BURN || damagetype & DMG_PREVENT_PHYSICS_FORCE || damagetype & DMG_DIRECT)//火焰伤害(只限地图火和间接火).
				if(g_iTakeDamage & Number_2)
					return Plugin_Handled;
			if(damagetype & DMG_BLAST || damagetype & DMG_BLAST_SURFACE)//爆炸类(土制炸弹,煤气罐,氧气罐).
				if(g_iTakeDamage & Number_4)
					return Plugin_Handled;
			if(damagetype & DMG_BLAST || damagetype & DMG_PLASMA || damagetype & DMG_AIRBOAT || damagetype & DMG_OTHER)//榴弹发射器(最后一个类型是有特殊弹药时).
				if(g_iTakeDamage & Number_8)
					return Plugin_Handled;
		}
	}
	return Plugin_Continue;
}
//玩家开火.
public void Event_WeaponFire(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));

	if(IsValidClient(client) && GetClientTeam(client) == 2)
		for (int i = 1; i <= MaxClients; i++)
			g_bPlayerHurt[client][i] = true;
}
//玩家受伤.
public void Event_PlayerHurt(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int attacker = GetClientOfUserId(event.GetInt("attacker"));

	if(g_iPlayerHurt > 0)
	{
		if(IsValidClient(attacker) && GetClientTeam(attacker) == 2)
		{
			if(IsValidClient(client) && GetClientTeam(client) == 2)
			{
				if (client == attacker)
					PrintToChat(attacker, "\x04[提示]\x05请勿自残\x04!");
				else
				{
					if (g_bPlayerHurt[attacker][client] == true)
					{
						int iBot[2];
						iBot[0] = IsClientIdle(client);
						iBot[1] = IsClientIdle(attacker);
						g_bPlayerHurt[attacker][client] = false;
						int iHitGroup = GetEventInt(event, "hitgroup");
						PrintToChat(iBot[0] != 0 ? iBot[0] : client, "\x04[提示]\x03%s\x05攻击了你\x04%s\x04.", GetTrueName(attacker), g_sHitName[iHitGroup]);
						PrintToChat(iBot[1] != 0 ? iBot[1] : attacker, "\x04[提示]\x05你攻击了\x03%s\x04%s\x04.", GetTrueName(client), g_sHitName[iHitGroup]);
					}
				}
			}
		}
	}
}
//玩家死亡.
public void Event_PlayerDeath(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int attacker = GetClientOfUserId(event.GetInt("attacker"));
	bool headshot = GetEventBool(event, "headshot");
	
	if (g_iBroadcast > 0 && IsValidClient(attacker) && client != attacker)
		IsPrintkillinfo(attacker, headshot);
}
//击杀或爆头计数.
void IsPrintkillinfo(int attacker, bool headshot)
{
	int murder;
	if(g_iBroadcast & Number_1)
	{
		g_iKillCounts[attacker] += 1;
		murder = g_iKillCounts[attacker];
		IsDisplayInfo(attacker, headshot, murder);
		delete g_hKillTimer[attacker];
		g_hKillTimer[attacker] = CreateTimer(5.0, IsKillCountTimer, GetClientUserId(attacker));//创建计时器.
	}
	if(headshot == true && (g_iBroadcast & Number_2))
	{
		g_iHeadCounts[attacker] += 1;
		murder = g_iHeadCounts[attacker];
		IsDisplayInfo(attacker, headshot, murder);
		delete g_hHeadTimer[attacker];
		g_hHeadTimer[attacker] = CreateTimer(5.0, IsHeadCountTimer, GetClientUserId(attacker));//创建计时器.
	}
}
//显示击杀或爆头提示.
void IsDisplayInfo(int attacker, bool headshot, int murder)
{
	if(murder > 1)
		PrintCenterText(attacker, "%s! +%d", headshot ? "爆头" : "击杀", murder);
	else
		PrintCenterText(attacker, "%s!", headshot ? "爆头" : "击杀");
}
//计时器回调.
Action IsKillCountTimer(Handle timer, any client)
{
	if ((client = GetClientOfUserId(client)) && IsClientInGame(client))
	{
		g_iKillCounts[client] = 0;
		g_hKillTimer[client] = null;
		return Plugin_Stop;
	}
	return Plugin_Continue;
}
//计时器回调.
Action IsHeadCountTimer(Handle timer, any client)
{
	if ((client = GetClientOfUserId(client)) && IsClientInGame(client))
	{
		g_iHeadCounts[client] = 0;
		g_hHeadTimer[client] = null;
		return Plugin_Stop;
	}
	return Plugin_Continue;
}
//玩家离开时.
public void OnClientDisconnect(int client)
{
	//if(!IsFakeClient(client))
	{
		g_iKillCounts[client] = 0;
		g_iHeadCounts[client] = 0;
		delete g_hKillTimer[client];
		delete g_hHeadTimer[client];
	}
}
//玩家有效.
stock bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}
stock char[] GetTrueName(int client)
{
	char g_sName[32];
	int Bot = IsClientIdle(client);
	
	if(Bot != 0)
		Format(g_sName, sizeof(g_sName), "闲置:%N", Bot);
	else
		GetClientName(client, g_sName, sizeof(g_sName));
	return g_sName;
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
stock int IsClientIdle(int client) 
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}