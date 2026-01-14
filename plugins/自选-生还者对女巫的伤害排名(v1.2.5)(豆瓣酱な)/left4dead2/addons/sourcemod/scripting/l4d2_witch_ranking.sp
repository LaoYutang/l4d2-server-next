
/*	修改GetDamagePercentage的百分比计算，尝试解决百分比溢出.	*/

/* 已知问题:如果有插件中途给女巫增加血会导致统计严重不准(例如某些通过给女巫加血或减血达到减伤或增伤目的的插件).*/
#pragma semicolon 1
#pragma dynamic 231072	//增加堆栈空间(不知道为什么生还者数量到达26个的时候会出问题,加上这个好像就好了).
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#include <sdkhooks>
#include <dhooks>
#undef REQUIRE_PLUGIN	//标记为可选开始.
#include <l4d2_GetWitchNumber>//自定义女巫编号插件.
#define REQUIRE_PLUGIN	//标记为可选结束.

#define MAX_SIZE		128	//定义字符串大小.
#define MAX_ARRAY	 	2048
#define PLUGIN_VERSION	"1.2.5"

bool g_bLateLoad, g_bWitchNumber;

int    g_iWitchRanking;
ConVar g_hWitchRanking;

float g_fWitchHP[MAX_ARRAY+1], g_fWitchHurt[MAX_ARRAY+1], g_fSurvivorWitchHurt[MAXPLAYERS+1][MAX_ARRAY+1];

public APLRes AskPluginLoad2(Handle myself, bool late, char[] error, int err_max)
{
	if(GetEngineVersion() != Engine_Left4Dead2 )
	{
		strcopy(error, err_max, "Plugin only supports Left 4 Dead 2");
		return APLRes_SilentFailure;
	}
	g_bLateLoad = late;
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

public Plugin myinfo =  
{
	name = "l4d2_witch_ranking",
	author = "豆瓣酱な",  
	description = "生还者对女巫的伤害排名.",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart()
{
	HookEvent("witch_spawn", Event_WitchSpawn);//女巫出现
	HookEvent("witch_killed", Event_Witchkilled);//女巫死亡.

	g_hWitchRanking = CreateConVar("l4d2_witch_Ranking", "5", "设置生还者对女巫的伤害的最大排名. 0=禁用.", FCVAR_NOTIFY);
	g_hWitchRanking.AddChangeHook(IsConVarChanged);
	AutoExecConfig(true, "l4d2_witch_ranking");//生成指定文件名的CFG.

	if(g_bLateLoad)//如果插件延迟加载.
	{
		int entity = -1;
		while ((entity = FindEntityByClassname(entity, "witch")) != INVALID_ENT_REFERENCE)
		{
			g_fWitchHP[entity] = GetWitchHealth(entity);//记录女巫的血量.
			SDKHook(entity, SDKHook_OnTakeDamageAlive, OnTakeDamageAlive);//勾住女巫.
		}
	}
}

//地图开始.
public void OnMapStart()
{
	IsGetCvars();
}

public void IsConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	IsGetCvars();
}

void IsGetCvars()
{
	g_iWitchRanking = g_hWitchRanking.IntValue;

	if (g_iWitchRanking > MaxClients)
		g_iWitchRanking = MaxClients;
}

//女巫出现.
public void Event_WitchSpawn(Event event, const char[] name, bool dontBroadcast)
{
	int iWitchid = event.GetInt("witchid");
	if (IsValidEdict(iWitchid))
	{
		//这里使用下一帧.
		DataPack hPack = new DataPack();
		hPack.WriteCell(iWitchid);
		RequestFrame(IsWitchFrameHealth, hPack);
		IsResetVariable(iWitchid);//女巫出现时重置整型变量.
	}
}

void IsWitchFrameHealth(DataPack hPack)
{
	hPack.Reset();
	int iWitchid = hPack.ReadCell();
	
	if (IsValidEdict(iWitchid))
		g_fWitchHP[iWitchid] = GetWitchHealth(iWitchid);//记录女巫出现时的血量.
	delete hPack;
}

float GetWitchHealth(int iWitchid)
{
	return float(GetEntProp(iWitchid, Prop_Data, "m_iHealth"));
}

public void OnEntityCreated (int entity, const char[] classname)
{
	if(entity <= MaxClients || !IsValidEntity(entity))
		return;
		
	if(classname[0] != 'w')
		return;
		
	if(strcmp(classname, "witch") == 0)
		SDKHook(entity, SDKHook_OnTakeDamageAlive, OnTakeDamageAlive);
}
//实体删除.
public void OnEntityDestroyed(int iEntity)
{
	if(IsValidEdict(iEntity))
	{
		static char sEntity[64];
		GetEntityClassname(iEntity, sEntity, sizeof(sEntity));
		if(strcmp(sEntity,"witch") == 0)
		{
			if (g_iWitchRanking > 0)
				IsWitchDamageSort(iEntity, GetWitchIndex(iEntity), "消失");
			IsResetVariable(iEntity);//女巫被删除时重置整型变量.
		}
	}
}
//女巫受伤.
public Action OnTakeDamageAlive(int client, int &attacker, int &inflictor, float &damage, int &damagetype, int &weapon, float damageForce[3], float damagePosition[3])
{
	if(IsValidClient(attacker) && GetClientTeam(attacker) == 2)
	{
		if (IsValidEdict(client))
		{
			int iBot = IsClientIdle(attacker);
			g_fWitchHurt[client] = GetWitchHealth(client);//记录女巫剩余的血量.
			g_fSurvivorWitchHurt[!iBot ? attacker : iBot][client] += damage > g_fWitchHurt[client] ? g_fWitchHurt[client] < 0.0 ? 0.0 : g_fWitchHurt[client] : damage;
		}
	}
	return Plugin_Changed;
}
//女巫死亡.
public void Event_Witchkilled(Event event, const char[] name, bool dontBroadcast)
{
	int iWitchid = event.GetInt("witchid");

	if (IsValidEdict(iWitchid))
	{
		if (g_iWitchRanking > 0)
			IsWitchDamageSort(iWitchid, GetWitchIndex(iWitchid), "死亡");
		IsResetVariable(iWitchid);//女巫被死亡时重置整型变量.
	}
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
//女巫伤害排行榜.
void IsWitchDamageSort(int witchId, char[] sIndex, char[] sWhys)
{
	int assister_count;
	float[][] assisters = new float[MaxClients][2];//动态数组.
	
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientInGame(i) && GetClientTeam(i) == 2)
		{
			int iBot = IsClientIdle(i);
			if (g_fSurvivorWitchHurt[!iBot ? i : iBot][witchId] > 0.0)
			{
				assisters[assister_count][0] = !iBot ? float(i) : float(iBot);
				assisters[assister_count][1] = g_fSurvivorWitchHurt[!iBot ? i : iBot][witchId];
				assister_count++;
			}
		}
	}
	if (assister_count > 0)
	{
		SortCustom2D(assisters, assister_count, ClientValue2DSortDesc);
		int iCount, iTemp[3], iMax[3];
		float fTotalDamage = GetTotalDamage(witchId);
		float fTotalHealth = GetTotalHealth(g_fWitchHP[witchId], fTotalDamage);
		char sInfo[128], sTemp[2][64];
		char[][][] sData = new char[assister_count][4][MAX_SIZE];//动态数组.
		FormatEx(sTemp[0], sizeof(sTemp[]), "\x05总血量\x04:\x03%d", RoundToFloor(g_fWitchHP[witchId]));
		if(RoundToFloor(fTotalDamage) > RoundToFloor(g_fWitchHP[witchId]))
			FormatEx(sTemp[1], sizeof(sTemp[]), "\x04+\x03%d", RoundToFloor(fTotalDamage - g_fWitchHP[witchId]));
		ImplodeStrings(sTemp, sizeof(sTemp), "", sInfo, sizeof(sInfo));//打包字符串.

		PrintToChatAll("\x04女巫%s\x03%s\x04,%s\x05HP\x04.\n\x05显示伤害排名\x04:\x03(\x05总伤害\x04:\x05%d\x03)", sIndex, sWhys, sInfo, RoundToFloor(fTotalDamage));
		for (int x = 0; x < g_iWitchRanking; x++)
		{
			int attacker = RoundToFloor(assisters[x][0]);
			float damage   = assisters[x][1];
			
			if (IsValidClient(attacker))
			{
				FormatEx(sData[x][0], MAX_SIZE, "%d", x + 1);
				FormatEx(sData[x][1], MAX_SIZE, "%.1f", GetDamagePercentage(damage, fTotalHealth));
				FormatEx(sData[x][2], MAX_SIZE, "%d", RoundToFloor(damage));
				FormatEx(sData[x][3], MAX_SIZE, "%N", attacker);
				iCount += 1;
			}
		}
		//后面执行对齐后显示.
		for (int y = 0; y < 3; y++)
			iTemp[y] = strlen(sData[0][y]);

		for (int x = 0; x < iCount; x++)
		{ 
			for (int y = 0; y < 3; y++)
				if(strlen(sData[x][y]) > iTemp[y])
					iTemp[y] = strlen(sData[x][y]);

			for (int y = 0; y < 3; y++)
				iMax[y] = iTemp[y] - strlen(sData[x][y]);

			PrintToChatAll("%s\x04%s%s\x05:\x03[%s\x04%s%%%s\x03]\x03(\x04%s%s%s\x03)\x05%s", 
			IsWritesData(iMax[0], " "), sData[x][0], IsWritesData(iMax[0], " "), IsWritesData(iMax[1], " "), sData[x][1], 
			IsWritesData(iMax[1], " "), IsWritesData(iMax[2], " "), sData[x][2], IsWritesData(iMax[2], " "), sData[x][3]);
		}
	}
}

float GetTotalHealth(float fWitchHealth, float fTotalDamage)
{
	return fTotalDamage > fWitchHealth ? fTotalDamage : fWitchHealth;
}
//生还者对女巫的总伤害.
float GetTotalDamage(int witchId)
{
	float fTotalDamage;
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientInGame(i) && GetClientTeam(i) == 2)
		{
			int iBot = IsClientIdle(i);
			if (g_fSurvivorWitchHurt[!iBot ? i : iBot][witchId] > 0)
				fTotalDamage += g_fSurvivorWitchHurt[!iBot ? i : iBot][witchId];
		}
	}
	return fTotalDamage;
}
//填入对应数量的内容.
stock char[] IsWritesData(int iNumber, char[] sValue)
{
	char sInfo[128];
	if(iNumber > 0)
	{
		int iLength = strlen(sValue) + 1;
		char[][] sData = new char[iNumber][iLength];//动态数组.
		for (int i = 0; i < iNumber; i++)
			strcopy(sData[i], iLength, sValue);
		ImplodeStrings(sData, iNumber, "", sInfo, sizeof(sInfo));//打包字符串.
	}
	return sInfo;
}

//计算百分比.
float GetDamagePercentage(float fDamage, float fTotalHealth)
{
	return fDamage / fTotalHealth * 100.0;
}

int ClientValue2DSortDesc(any[] elem1, any[] elem2, const any[][] array, Handle hndl)
{
	if (elem1[1] > elem2[1])
		return -1;
	else if (elem2[1] > elem1[1])
		return 1;
		
	return 0;
}

//女巫死亡后重置整型变量.
void IsResetVariable(int Witchid)
{
	for(int i = 1; i <= MaxClients; i++)
		g_fSurvivorWitchHurt[i][Witchid] = 0.0;
	g_fWitchHP[Witchid] = 0.0;
}
//判断玩家有效性.
stock bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}

int IsClientIdle(int client)
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}