/*
 *	v1.0.4
 *
 *	1:修复 player_death 事件里偶尔出现玩家不在游戏中的报错.
 *
 *	v1.1.4
 *
 *	1:修复可选插件未安装时插件无效的问题.
 *
 */
#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#undef REQUIRE_PLUGIN	//标记为可选开始.
#include <l4d2_GetWitchNumber>//女巫自定义编号插件.
#define REQUIRE_PLUGIN	//标记为可选结束.

#define CVAR_FLAGS		FCVAR_NOTIFY
#define PLUGIN_VERSION 	"1.1.4"

//这里设置击杀奖励的血量(根据g_sZombieName数组顺序设置).
int g_iKillDefault[] = {1, 1, 1, 1, 1, 2, 5, 10};
//这里设置爆头奖励的血量(根据g_sZombieName数组顺序设置).
int g_iHeadDefault[] = {2, 2, 2, 2, 2, 5, 15, 35};

bool g_bWitchNumber;

public APLRes AskPluginLoad2(Handle myself, bool late, char[] error, int err_max)
{
	if(GetEngineVersion() != Engine_Left4Dead2 )
	{
		strcopy(error, err_max, "Plugin only supports Left 4 Dead 2");
		return APLRes_SilentFailure;
	}
	return APLRes_Success;
}

public void OnAllPluginsLoaded()
{
	g_bWitchNumber = LibraryExists("l4d2_GetWitchNumber");
}

public void OnLibraryAdded(const char[] sName)
{
	if(StrEqual(sName, "l4d2_GetWitchNumber"))
		g_bWitchNumber = true;
}

public void OnLibraryRemoved(const char[] sName)
{
	if(StrEqual(sName, "l4d2_GetWitchNumber"))
		g_bWitchNumber = false;
}

char g_sZombieClass[][] = 
{
	"Smoker",
	"Boomer",
	"Hunter",
	"Spitter",
	"Jockey",
	"Charger",
	"witch",
	"Tank"
};

char g_sZombieName[][] = 
{
	"舌头",
	"胖子",
	"猎人",
	"口水",
	"猴子",
	"牛牛",
	"女巫",
	"坦克"
};

int    g_iKill[sizeof(g_iKillDefault)], g_iHead[sizeof(g_iHeadDefault)], g_iOneshotWitch, g_iLimitHealth, g_iReviveSuccess, g_iSurvivorRescued, g_iHealSuccess, g_iDefibrillator;
ConVar g_hKill[sizeof(g_iKillDefault)], g_hHead[sizeof(g_iHeadDefault)], g_hOneshotWitch, g_hLimitHealth, g_hReviveSuccess, g_hSurvivorRescued, g_hHealSuccess, g_hDefibrillator;

public Plugin myinfo =
{
	name = "l4d2_health_rewards",
	author = "豆瓣酱な", 
	description = "击杀特感和女巫奖励血量.",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart()
{
	
	HookEvent("witch_killed", Event_Witchkilled);//女巫死亡.
	HookEvent("witch_harasser_set", Event_WitchHarasserSet);//惊扰女巫
	HookEvent("player_death", Event_PlayerDeath);//玩家死亡.
	
	HookEvent("defibrillator_used", Event_DefibrillatorUsed);//幸存者使用电击器救活队友.
	HookEvent("revive_success", Event_ReviveSuccess);//救起幸存者.
	HookEvent("survivor_rescued", Event_SurvivorRescued);//幸存者在营救门复活.
	HookEvent("heal_success", Event_HealSuccess);//幸存者治疗.
	HookEvent("adrenaline_used", Event_AdrenalineUsed, EventHookMode_Pre);//使用肾上腺素.
	
	char bar[2][128], buffers[2][128],value[2][128];
	for (int i = 0; i < sizeof(g_iKillDefault); i++)
	{
		FormatEx(buffers[0], sizeof(buffers[]), "l4d2_health_Kill_%s", g_sZombieClass[i]);
		FormatEx(value  [0], sizeof(value  []), "%d", g_iKillDefault[i]);
		FormatEx(bar    [0], sizeof(bar    []), "击杀%s的幸存者奖励多少血. 0=禁用(设置小于0等于启用加血但是不显示提示).", g_sZombieName[i]);
		g_hKill[i] = CreateConVar(buffers[0], value[0], bar[0], CVAR_FLAGS);
	}

	for (int i = 0; i < sizeof(g_iHeadDefault); i++)
	{
		FormatEx(buffers[1], sizeof(buffers[]), "l4d2_health_Head_%s", g_sZombieClass[i]);
		FormatEx(value  [1], sizeof(value  []), "%d", g_iHeadDefault[i]);
		FormatEx(bar    [1], sizeof(bar    []), "爆头%s的幸存者奖励多少血. 0=禁用(设置小于0等于启用加血但是不显示提示).", g_sZombieName[i]);
		g_hHead[i] = CreateConVar(buffers[1], value[1], bar[1], CVAR_FLAGS);
	}
	
	g_hOneshotWitch	= CreateConVar("l4d2_health_oneshot_witch", "20", "秒杀女巫的幸存者奖励多少血. 0=禁用(设置小于0等于启用加血但是不显示提示).", FCVAR_NOTIFY);
	g_hLimitHealth	= CreateConVar("l4d2_survivor_health_Limit", "100", "设置幸存者获得血量奖励的最高上限.", FCVAR_NOTIFY);
	
	g_hReviveSuccess	= CreateConVar("l4d2_health_reviveSuccess", "2", "救起倒地的幸存者奖励多少血. 0=禁用(设置小于0等于启用加血但是不显示提示).", FCVAR_NOTIFY);
	g_hSurvivorRescued	= CreateConVar("l4d2_health_survivorRescued", "3", "营救队友的幸存者奖励多少血. 0=禁用(设置小于0等于启用加血但是不显示提示).", FCVAR_NOTIFY);
	g_hHealSuccess		= CreateConVar("l4d2_health_healSuccess", "15", "治愈队友的幸存者奖励多少血. 0=禁用(设置小于0等于启用加血但是不显示提示).", FCVAR_NOTIFY);
	g_hDefibrillator	= CreateConVar("l4d2_health_defibrillator", "20", "电击器复活队友的幸存者奖励多少血. 0=禁用(设置小于0等于启用加血但是不显示提示).", FCVAR_NOTIFY);
	
	for (int i = 0; i < sizeof(g_iKillDefault); i++)
		g_hKill[i].AddChangeHook(ConVarChangedHealth);

	for (int i = 0; i < sizeof(g_iHeadDefault); i++)
		g_hHead[i].AddChangeHook(ConVarChangedHealth);

	g_hOneshotWitch.AddChangeHook(ConVarChangedHealth);
	g_hLimitHealth.AddChangeHook(ConVarChangedHealth);

	g_hReviveSuccess.AddChangeHook(ConVarChangedHealth);
	g_hSurvivorRescued.AddChangeHook(ConVarChangedHealth);
	g_hHealSuccess.AddChangeHook(ConVarChangedHealth);
	g_hDefibrillator.AddChangeHook(ConVarChangedHealth);

	AutoExecConfig(true, "l4d2_health_rewards");//生成指定文件名的CFG.
}

//地图开始.
public void OnMapStart()
{
	GetConVarChange();
}

public void ConVarChangedHealth(ConVar convar, const char[] oldValue, const char[] newValue)
{
	GetConVarChange();
}

void GetConVarChange()
{
	for (int i = 0; i < sizeof(g_iKillDefault); i++)
		g_iKill[i] = g_hKill[i].IntValue;
	for (int i = 0; i < sizeof(g_iHeadDefault); i++)
		g_iHead[i] = g_hHead[i].IntValue;

	g_iOneshotWitch = g_hOneshotWitch.IntValue;
	g_iLimitHealth = g_hLimitHealth.IntValue;

	g_iReviveSuccess = g_hReviveSuccess.IntValue;
	g_iSurvivorRescued = g_hSurvivorRescued.IntValue;
	g_iHealSuccess = g_hHealSuccess.IntValue;
	g_iDefibrillator = g_hDefibrillator.IntValue;
}
//使用肾上腺素.
public void Event_AdrenalineUsed(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));

	if(IsValidClient(client) && GetClientTeam(client) == 2)
	{
		int iHealth = GetClientHealth(client);
		int tHealth = GetPlayerTempHealth(client);
		//重新设置一次血量,以避免一些问题.
		SetEntPropFloat(client, Prop_Send, "m_healthBuffer", float(tHealth) < 0.0 ? 0.0 : float(tHealth));
		SetEntPropFloat(client, Prop_Send, "m_healthBufferTime", GetGameTime());
		SetEntityHealth(client, iHealth < 1 ? 1 : iHealth);
	}
}
//电击器救活队友.
public void Event_DefibrillatorUsed(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int subject = GetClientOfUserId(event.GetInt("subject"));

	if(g_iDefibrillator != 0 && IsValidClient(client) && GetClientTeam(client) == 2)
	{
		if(IsValidClient(subject) && GetClientTeam(subject) == 2)
		{
			if(client != subject)
			{
				int iReward = g_iDefibrillator;

				if(SetSurvivorHealth(client, GetRewardHealth(iReward), g_iLimitHealth))
				{
					if(iReward > 0)
						PrintToChatAll("\x04[提示]\x03%s\x05救活了\x03%s\x04,\x05奖励\x03%d\x05点血量.", GetTrueName(client), GetTrueName(subject), GetRewardHealth(iReward));
				}
				else
				{
					if(iReward > 0)
						PrintToChatAll("\x04[提示]\x03%s\x05救活了\x03%s\x04,\x05血量已达\x03%d\x05上限.", GetTrueName(client), GetTrueName(subject), g_iLimitHealth);//聊天窗提示.
				}
			}
		}
	}
}

public void Event_ReviveSuccess(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int subject = GetClientOfUserId(event.GetInt("subject"));

	if(g_iReviveSuccess != 0 && IsValidClient(client) && GetClientTeam(client) == 2)
	{
		if(IsValidClient(subject) && GetClientTeam(subject) == 2)
		{
			if(client != subject)
			{
				int iReward = g_iReviveSuccess;
				if(SetSurvivorHealth(client, GetRewardHealth(iReward), g_iLimitHealth))
				{
					if(iReward > 0)
						PrintToChatAll("\x04[提示]\x03%s\x05救起了\x03%s\x04,\x05奖励\x03%d\x05点血量.", GetTrueName(client), GetTrueName(subject), GetRewardHealth(iReward));
				}
				else
				{
					if(iReward > 0)
						PrintToChatAll("\x04[提示]\x03%s\x05救起了\x03%s\x04,\x05血量已达\x03%d\x05上限.", GetTrueName(client), GetTrueName(subject), g_iLimitHealth);//聊天窗提示.
				}
			}
		}
	}
}
//幸存者治疗.
public void Event_HealSuccess(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int subject = GetClientOfUserId(event.GetInt("subject"));

	if(g_iHealSuccess != 0 && IsValidClient(client) && GetClientTeam(client) == 2)
	{
		if(IsValidClient(subject) && GetClientTeam(subject) == 2)
		{
			if(client != subject)
			{
				int iReward = g_iHealSuccess;

				if(SetSurvivorHealth(client, GetRewardHealth(iReward), g_iLimitHealth))
				{
					if(iReward > 0)
						PrintToChatAll("\x04[提示]\x03%s\x05治疗了\x03%s\x04,\x05奖励\x03%d\x05点血量.", GetTrueName(client), GetTrueName(subject), GetRewardHealth(iReward));
				}
				else
				{
					if(iReward > 0)
						PrintToChatAll("\x04[提示]\x03%s\x05治疗了\x03%s\x04,\x05血量已达\x03%d\x05上限.", GetTrueName(client), GetTrueName(subject), g_iLimitHealth);//聊天窗提示.
				}
			}
		}
	}
}
//幸存者在营救门复活.
public void Event_SurvivorRescued(Event event, const char[] name, bool dontBroadcast)
{
	int rescuer = GetClientOfUserId(event.GetInt("rescuer"));
	int client = GetClientOfUserId(event.GetInt("victim"));

	if(g_iSurvivorRescued != 0 && IsValidClient(client) && GetClientTeam(client) == 2)
	{
		if(IsValidClient(rescuer) && GetClientTeam(rescuer) == 2)
		{
			if(client != rescuer)
			{
				int iReward = g_iSurvivorRescued;

				if(SetSurvivorHealth(rescuer, GetRewardHealth(iReward), g_iLimitHealth))
				{
					if(iReward > 0)
						PrintToChatAll("\x04[提示]\x03%s\x05营救了\x03%s\x04,\x05奖励\x03%d\x05点血量.", GetTrueName(rescuer), GetTrueName(client), GetRewardHealth(iReward));
				}
				else
				{
					if(iReward > 0)
						PrintToChatAll("\x04[提示]\x03%s\x05营救了\x03%s\x04,\x05血量已达\x03%d\x05上限.", GetTrueName(rescuer), GetTrueName(client), g_iLimitHealth);//聊天窗提示.
				}
			}
		}
	}
}
//惊扰女巫提示.
public void Event_WitchHarasserSet(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int iWitchid = event.GetInt("witchid" );
	
	if(IsValidClient(client) && GetClientTeam(client) == 2)
		PrintToChatAll("\x04[提示]\x03%s\x05惊扰了\x03%s.", GetTrueName(client), GetWitchName(iWitchid));//聊天窗提示.
}

public void Event_Witchkilled(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));

	if(IsValidClient(client) && GetClientTeam(client) == 2)
	{
		char sType[32];
		int iReward;
		int iWitchid = event.GetInt("witchid");
		int iType = GetWitchkilled(iWitchid, sType, sizeof(sType));
		
		switch (iType)
		{
			case 0: iReward = g_iKill[6];
			case 1: iReward = g_iHead[6];
			case 2: iReward = g_iOneshotWitch;
			default: iReward = g_iKill[6];
		}
		if(SetSurvivorHealth(client, GetRewardHealth(iReward), g_iLimitHealth))
		{
			if(iReward > 0)
				PrintToChatAll("\x04[提示]\x03%s\x05%s了\x03%s\x04,\x05奖励\x03%d\x05点血量.", GetTrueName(client), sType, GetWitchName(iWitchid), GetRewardHealth(iReward));
		}
		else
		{
			if(iReward > 0)
				PrintToChatAll("\x04[提示]\x03%s\x05%s了\x03%s\x04,\x05血量已达\x03%d\x05上限.", GetTrueName(client), sType, GetWitchName(iWitchid), g_iLimitHealth);//聊天窗提示.
		}
	}
}

public void Event_PlayerDeath(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	int attacker = GetClientOfUserId(event.GetInt("attacker"));
	int headshot = event.GetBool("headshot");
	
	if(IsValidClient(attacker) && GetClientTeam(attacker) == 2)
	{
		if(IsValidClient(client) && GetClientTeam(client) == 3)
		{
			char sType[32];
			strcopy(sType, sizeof(sType), headshot == 0 ? "击杀" : "爆头");
			int iHLZClass = GetEntProp(client, Prop_Send, "m_zombieClass") - 1;
			int iReward = headshot == 0 ? g_iKill[iHLZClass] :  g_iHead[iHLZClass];

			int iBot = IsClientIdle(attacker);

			if(IsPlayerAlive(attacker) || IsPlayerState(attacker))
			{
				if(SetSurvivorHealth(attacker, GetRewardHealth(iReward), g_iLimitHealth))
				{
					if(iReward > 0)
						if(iBot != 0)
						{
							if(IsValidClient(iBot))
								PrintToChat(iBot, "\x04[提示]\x05%s了\x03%s\x04,\x05奖励\x03%d\x05点血量.", sType, GetPlayerName(client, iHLZClass), GetRewardHealth(iReward));
						}
						else
							PrintToChat(attacker, "\x04[提示]\x05%s了\x03%s\x04,\x05奖励\x03%d\x05点血量.", sType, GetPlayerName(client, iHLZClass), GetRewardHealth(iReward));
				}
				else
				{
					if(iReward > 0)
						if(iBot != 0)
						{
							if(IsValidClient(iBot))
								PrintToChat(iBot, "\x04[提示]\x05%s了\x03%s\x04,\x05血量已达\x03%d\x05上限.", sType, GetPlayerName(client, iHLZClass), g_iLimitHealth);//聊天窗提示.
						}
						else
							PrintToChat(attacker, "\x04[提示]\x05%s了\x03%s\x04,\x05血量已达\x03%d\x05上限.", sType, GetPlayerName(client, iHLZClass), g_iLimitHealth);//聊天窗提示.
				}	
			}
			else
			{
				if(iReward > 0)
					if(iBot != 0)
					{
						if(IsValidClient(iBot))
							PrintToChat(iBot, "\x04[提示]\x05%s了\x03%s\x04.", sType, GetPlayerName(client, iHLZClass));//聊天窗提示.
					}
					else
						PrintToChat(attacker, "\x04[提示]\x05%s了\x03%s\x04.", sType, GetPlayerName(client, iHLZClass));//聊天窗提示.
					
			}
		}
	}
}

char[] GetPlayerName(int client, int iHLZClass)
{
	char sName[32];
	GetClientName(client, sName, sizeof(sName));

	if(!IsFakeClient(client))
	{
		Format(sName, sizeof(sName), "%s\x04%s", g_sZombieName[iHLZClass], sName);
	}
	else
	{
		SplitString(sName, g_sZombieClass[iHLZClass], sName, sizeof(sName));
		Format(sName, sizeof(sName), "%s%s", g_sZombieName[iHLZClass], sName);
	}
	return sName;
}
bool SetSurvivorHealth(int attacker, int iReward, int iMaxHealth)
{
	int iHealth = GetClientHealth(attacker);
	int tHealth = GetPlayerTempHealth(attacker);

	if (tHealth == -1)
		tHealth = 0;
	
	if (iHealth + tHealth + iReward > iMaxHealth)
	{
		float overhealth, fakehealth;
		overhealth = float(iHealth + tHealth + iReward - iMaxHealth);
		if (tHealth < overhealth)
			fakehealth = 0.0;
		else
			fakehealth = float(tHealth) - overhealth;
		
		SetEntPropFloat(attacker, Prop_Send, "m_healthBufferTime", GetGameTime());
		SetEntPropFloat(attacker, Prop_Send, "m_healthBuffer", fakehealth);
	}
		
	if ((iHealth + iReward) < iMaxHealth)
	{
		SetEntProp(attacker, Prop_Send, "m_iHealth", iHealth + iReward);
		return true;
	}
	else
	{
		SetEntProp(attacker, Prop_Send, "m_iHealth", iHealth > iMaxHealth ? iHealth : iMaxHealth);
	}
	return false;
}
//获取奖励的血量.
int GetRewardHealth(int iReward)
{
	return iReward = iReward < 0 ? iReward * -1 : iReward;
}
bool IsValidClient(int client)
{
	return (client > 0 && client <= MaxClients && IsClientInGame(client));
}
//正常状态.
bool IsPlayerState(int client)
{
	return !GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}
//获取自定义的女巫名字.
char[] GetWitchName(int iWitchid)
{
	char sName[32];
	if(g_bWitchNumber == true)
		FormatEx(sName, sizeof(sName), "女巫%s", GetWitchIndex(iWitchid));
	else
		strcopy(sName, sizeof(sName), "女巫");
	return sName;
}
//获取自定义的女巫编号.
char[] GetWitchIndex(int iWitchid)
{
	char sName[32];
	if(g_bWitchNumber == true)
	{
		int iIndex = GetWitchNumber(iWitchid);
		if(iIndex != 0)
			FormatEx(sName, sizeof(sName), "(%d)", iIndex);
	}
	return sName;
}
char[] GetTrueName(int client)
{
	char sName[32];
	int Bot = IsClientIdle(client);
	
	if(Bot != 0)
		FormatEx(sName, sizeof(sName), "闲置:%N", Bot);
	else
		GetClientName(client, sName, sizeof(sName));
	return sName;
}
int IsClientIdle(int client) 
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}
//获取虚血值.
int GetPlayerTempHealth(int client)
{
    static Handle painPillsDecayCvar = null;
    if (painPillsDecayCvar == null)
    {
        painPillsDecayCvar = FindConVar("pain_pills_decay_rate");
        if (painPillsDecayCvar == null)
            return -1;
    }

    int tempHealth = RoundToCeil(GetEntPropFloat(client, Prop_Send, "m_healthBuffer") - ((GetGameTime() - GetEntPropFloat(client, Prop_Send, "m_healthBufferTime")) * GetConVarFloat(painPillsDecayCvar))) - 1;
    return tempHealth < 0 ? 0 : tempHealth;
}