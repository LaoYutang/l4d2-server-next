#pragma semicolon 1
#pragma newdecls required
#include <sourcemod>
#include <sdktools>
#define PLUGIN_VERSION 	"1.1.1"
#define NAME_RoundRespawn "CTerrorPlayer::RoundRespawn"
#define SIG_RoundRespawn_LINUX "@_ZN13CTerrorPlayer12RoundRespawnEv"
#define SIG_RoundRespawn_WINDOWS "\\x2A\\x2A\\x2A\\x2A\\x2A\\x2A\\x2A\\x2A\\xE8\\x2A\\x2A\\x2A\\x2A\\x84\\x2A\\x75\\x2A\\x8B\\x2A\\xE8\\x2A\\x2A\\x2A\\x2A\\xC6\\x86"

Handle hRoundRespawn;
Address g_pStatsCondition;
bool SurvivorsHealth, SurvivorsDefaults = false;

int    g_iEnabled, g_iDefaults;
ConVar g_hEnabled, g_hDefaults;

public Plugin myinfo =
{
	name = "l4d2_survivor_transition",
	author = "豆瓣酱な", 
	description = "生还者过关时自动满血或复活.",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart() 
{
	IsLoadGameCFG();
	
	RegConsoleCmd("sm_hp", Command_ResetSurvivor, "生还者过关时自动满血或复活.");
	
	g_hEnabled	= CreateConVar("l4d2_survivor_transition_Enabled",	"1", "启用生还者过关时自动满血或复活? 0=禁用, 1=启用.", FCVAR_NOTIFY);
	g_hDefaults	= CreateConVar("l4d2_survivor_transition_defaults", "1", "设置开启或关闭生还者过关满血或复活. 0=关闭, 1=开启.", FCVAR_NOTIFY);
	
	g_hEnabled.AddChangeHook(l4d2survivorsConVarChanged);
	g_hDefaults.AddChangeHook(l4d2survivorsConVarChanged);
	
	HookEvent("map_transition", Event_ResetSurvivors, EventHookMode_Pre);
	
	AutoExecConfig(true, "l4d2_survivor_transition");//生成指定文件名的CFG.
}

public void OnPluginEnd()
{
	vStatsConditionPatch(false);
}

public void OnMapStart()
{
	l4d2_cvar_survivors();
}

public void l4d2survivorsConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	l4d2_cvar_survivors();
}

void l4d2_cvar_survivors()
{
	g_iEnabled	= g_hEnabled.IntValue;
	g_iDefaults	= g_hDefaults.IntValue;
}

public Action Command_ResetSurvivor(int client, int args)
{
	if(bCheckClientAccess(client))
	{
		if (SurvivorsHealth)
		{
			SurvivorsHealth = false;
			SurvivorsDefaults = true;
			PrintToChat(client, "\x04[提示]\x03已关闭\x05生还者过关时自动满血.");
		}
		else
		{
			SurvivorsHealth = true;
			SurvivorsDefaults = true;
			PrintToChat(client, "\x04[提示]\x03已开启\x05生还者过关时自动满血.");
		}
	}
	else
		PrintToChat(client, "\x04[提示]\x05你无权使用此指令.");
	return Plugin_Handled;
}
	
bool bCheckClientAccess(int client)
{
	if(GetUserFlagBits(client) & ADMFLAG_ROOT)
		return true;
	return false;
}

public void OnConfigsExecuted()
{
	IsLoadGameCFG();
	
	if(!SurvivorsDefaults)
	{
		switch (g_iDefaults)
		{
			case 0:
				SurvivorsHealth = false;
			case 1:
				SurvivorsHealth = true;
		}
	}
}

/// 初始化
public void IsLoadGameCFG()
{
	char sPath[PLATFORM_MAX_PATH];
	BuildPath(Path_SM, sPath, sizeof(sPath), "gamedata/l4d2_survivor_transition.txt");
	
	//判断是否有文件
	if (FileExists(sPath))
	{
		GameData hGameData = new GameData("l4d2_survivor_transition");
		if(hGameData == null) 
			SetFailState("Failed to load gamedata/l4d2_survivor_transition.txt");
			
		StartPrepSDKCall(SDKCall_Player);
		if(PrepSDKCall_SetFromConf(hGameData, SDKConf_Signature, "CTerrorPlayer::RoundRespawn") == false)
			SetFailState("Failed to find signature: CTerrorPlayer::RoundRespawn");
		else
		{
			hRoundRespawn = EndPrepSDKCall();
			if(hRoundRespawn == null)
				SetFailState("Failed to create SDKCall: CTerrorPlayer::RoundRespawn");
		}
		
		vRegisterStatsConditionPatch(hGameData);
		
		delete hGameData;
	}
	else
	{
		//创建偏移文件.
		PrintToServer("[提示] 未发现%s文件,创建中...", sPath);
		File hFile = OpenFile(sPath, "w", false);
	
		hFile.WriteLine("\"Games\"");
		hFile.WriteLine("{");

		hFile.WriteLine("	\"left4dead2\"");
		hFile.WriteLine("	{");
		
		hFile.WriteLine("		\"Addresses\"");
		hFile.WriteLine("		{");
		
		hFile.WriteLine("			\"CTerrorPlayer::RoundRespawn\"");
		hFile.WriteLine("			{");
		hFile.WriteLine("				\"linux\"");
		hFile.WriteLine("				{");
		hFile.WriteLine("					\"signature\"	\"CTerrorPlayer::RoundRespawn\"");
		hFile.WriteLine("				}");
		hFile.WriteLine("				\"windows\"");
		hFile.WriteLine("				{");
		hFile.WriteLine("					\"signature\"	\"CTerrorPlayer::RoundRespawn\"");
		hFile.WriteLine("				}");
		hFile.WriteLine("			}");
		
		hFile.WriteLine("		}");
		
		hFile.WriteLine("		\"Offsets\"");
		hFile.WriteLine("		{");
		
		hFile.WriteLine("			\"RoundRespawn_Offset\"");
		hFile.WriteLine("			{");
		hFile.WriteLine("				\"linux\"	\"25\"");
		hFile.WriteLine("				\"windows\"	\"15\"");
		hFile.WriteLine("			}");
		
		hFile.WriteLine("			\"RoundRespawn_Byte\"");
		hFile.WriteLine("			{");
		hFile.WriteLine("				\"linux\"	\"117\"");
		hFile.WriteLine("				\"windows\"	\"117\"");
		hFile.WriteLine("			}");
		
		hFile.WriteLine("		}");
		
		hFile.WriteLine("		\"Signatures\"");
		hFile.WriteLine("		{");
		
		hFile.WriteLine("			\"%s\"", NAME_RoundRespawn);
		hFile.WriteLine("			{");
		hFile.WriteLine("				\"library\"	\"server\"");
		hFile.WriteLine("				\"linux\"	\"%s\"", SIG_RoundRespawn_LINUX);
		hFile.WriteLine("				\"windows\"	\"%s\"", SIG_RoundRespawn_WINDOWS);
		hFile.WriteLine("			}");
		
		hFile.WriteLine("		}");
			
		hFile.WriteLine("	}");
		hFile.WriteLine("}");
		
		FlushFile(hFile);
		delete hFile;
	}
}

void vRegisterStatsConditionPatch(GameData hGameData = null)
{
	int iOffset = hGameData.GetOffset("RoundRespawn_Offset");
	if(iOffset == -1)
		SetFailState("Failed to find offset: RoundRespawn_Offset");

	int iByteMatch = hGameData.GetOffset("RoundRespawn_Byte");
	if(iByteMatch == -1)
		SetFailState("Failed to find byte: RoundRespawn_Byte");

	g_pStatsCondition = hGameData.GetAddress("CTerrorPlayer::RoundRespawn");
	if(!g_pStatsCondition)
		SetFailState("Failed to find address: CTerrorPlayer::RoundRespawn");
	
	g_pStatsCondition += view_as<Address>(iOffset);
	
	int iByteOrigin = LoadFromAddress(g_pStatsCondition, NumberType_Int8);
	if(iByteOrigin != iByteMatch)
		SetFailState("Failed to load 'CTerrorPlayer::RoundRespawn', byte mis-match @ %d (0x%02X != 0x%02X)", iOffset, iByteOrigin, iByteMatch);
}

public void Event_ResetSurvivors(Event event, const char[] name, bool dontBroadcast)
{
	if (g_iEnabled != 0 && SurvivorsHealth)
		ResetSurvivorsiMaxHealth();
}

char[] GetGameMode()
{
	char g_sMode[32];
	GetConVarString(FindConVar("mp_gamemode"), g_sMode, sizeof(g_sMode));
	return g_sMode;
}

void ResetSurvivorsiMaxHealth()
{
	for (int i = 1; i <= MaxClients; i++)
	{
		if(IsClientInGame(i) && GetClientTeam(i) == 2)
		{
			if (!IsAlive(i))//死亡状态.
				vRoundRespawn(i);//复活玩家.
			else
				SetSurvivoHealth(i);//加满血量.
		}
	}
}

//设置生还者血量.
void SetSurvivoHealth(int client)
{
	if (!IsPlayerState(client))//正常状态.
	{
		CheatCommand(client, "give", "health");//加满血量.
		SetSurvivoTempHealth(client);
	}
	else
	{
		int iHealth = GetClientHealth(client);//获取玩家实血.
		int tHealth = GetPlayerTempHealth(client);
		int iMaxHP  = GetEntProp(client, Prop_Data, "m_iMaxHealth");
		int iTempHealth = iMaxHP - iHealth;
		int iPlayerHP = tHealth + iHealth;
		
		if (strcmp(GetGameMode(), "mutation3", false) == 0)//虚血模式
		{
			SetEntPropFloat(client, Prop_Send, "m_healthBufferTime", GetGameTime());
			SetEntPropFloat(client, Prop_Send, "m_healthBuffer", iPlayerHP < iMaxHP ? float(iTempHealth + 1) : float(tHealth + 1));
		}
		else
		{
			SetEntPropFloat(client, Prop_Send, "m_healthBuffer", 0.0);
			SetEntityHealth(client, iHealth < iMaxHP ? iMaxHP : iHealth);//设置玩家实血.
			SetEntPropFloat(client, Prop_Send, "m_healthBufferTime", GetGameTime());
		}
		SetEntProp(client, Prop_Send, "m_isGoingToDie", 0);//不知道干嘛的.
		SetEntProp(client, Prop_Send, "m_bIsOnThirdStrike", 0);//取消黑白状态.
		SetEntProp(client, Prop_Send, "m_currentReviveCount", 0);//重置倒地次数.
		StopSound(client, SNDCHAN_STATIC, "player/heartbeatloop.wav");//去除黑白心跳声.
	}
}

void vRoundRespawn(int client)
{
	vStatsConditionPatch(true);
	SDKCall(hRoundRespawn, client);
	vStatsConditionPatch(false);
	TeleportClient(client);//复活后随机传送到其他幸存者身边.
	SetSurvivoTempHealth(client);
}

void vStatsConditionPatch(bool bPatch)
{
	static bool bPatched;
	if(!bPatched && bPatch)
	{
		bPatched = true;
		StoreToAddress(g_pStatsCondition, 0x79, NumberType_Int8);
	}
	else if(bPatched && !bPatch)
	{
		bPatched = false;
		StoreToAddress(g_pStatsCondition, 0x75, NumberType_Int8);
	}
}

void SetSurvivoTempHealth(int client)
{
	int iMaxHP  = GetEntProp(client, Prop_Data, "m_iMaxHealth");
	if (strcmp(GetGameMode(), "mutation3", false) == 0)//虚血模式
		SetEntityHealth(client, 1);//设置玩家实血.
	SetEntPropFloat(client, Prop_Send, "m_healthBufferTime", GetGameTime());
	SetEntPropFloat(client, Prop_Send, "m_healthBuffer", strcmp(GetGameMode(), "mutation3", false) == 0 ? float(iMaxHP) : 0.0);
}

void CheatCommand(int client, char[] strCommand, char[] strParam1)
{
	int flags = GetCommandFlags(strCommand);
	SetCommandFlags(strCommand, flags & ~FCVAR_CHEAT);
	FakeClientCommand(client, "%s %s", strCommand, strParam1);
	SetCommandFlags(strCommand, flags);
}

//死亡状态.
bool IsAlive(int client)
{
	return !GetEntProp(client, Prop_Send, "m_lifeState");
}

//正常状态.
bool IsPlayerState(int client)
{
	return !GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}

//随机传送幸存者到其他幸存者身边.
void TeleportClient(int client)
{
	int iTarget = GetTeleportTarget(client);
	
	if(iTarget != -1)
	{
		//传送时强制蹲下防止卡住.
		ForceCrouch(client);
		
		float vPos[3];
		GetClientAbsOrigin(iTarget, vPos);
		TeleportEntity(client, vPos, NULL_VECTOR, NULL_VECTOR);
	}
}

int GetTeleportTarget(int client)
{
	int iNormal, iIncap, iHanging;
	int[] iNormalSurvivors = new int[MaxClients];
	int[] iIncapSurvivors = new int[MaxClients];
	int[] iHangingSurvivors = new int[MaxClients];
	
	for(int i = 1; i <= MaxClients; i++)
	{
		if(i != client && IsClientInGame(i) && GetClientTeam(i) == 2 && IsAlive(i))
		{
			if(GetEntProp(i, Prop_Send, "m_isIncapacitated") > 0)
			{
				if(GetEntProp(i, Prop_Send, "m_isHangingFromLedge") > 0)
					iHangingSurvivors[iHanging++] = i;
				else
					iIncapSurvivors[iIncap++] = i;
			}
			else
				iNormalSurvivors[iNormal++] = i;
		}
	}
	return (iNormal == 0) ? (iIncap == 0 ? (iHanging == 0 ? -1 : iHangingSurvivors[GetRandomInt(0, iHanging - 1)]) : iIncapSurvivors[GetRandomInt(0, iIncap - 1)]) :iNormalSurvivors[GetRandomInt(0, iNormal - 1)];
}

void ForceCrouch(int client)
{
	SetEntProp(client, Prop_Send, "m_bDucked", 1);
	SetEntProp(client, Prop_Send, "m_fFlags", GetEntProp(client, Prop_Send, "m_fFlags") | FL_DUCKING);
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
