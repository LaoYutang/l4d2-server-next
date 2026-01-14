#pragma semicolon 1
#pragma newdecls required
#include <sourcemod>

#define CVAR_FLAGS 		FCVAR_NOTIFY
#define PLUGIN_VERSION "1.0.0"
#define DEBUG	0	//0=禁用调试信息,1=显示调试信息.

enum L4D2WeaponType 
{
	L4D2WeaponType_Unknown = 0,
	L4D2WeaponType_Pistol,
	L4D2WeaponType_Magnum,
	L4D2WeaponType_Rifle,
	L4D2WeaponType_RifleAk47,
	L4D2WeaponType_RifleDesert,
	L4D2WeaponType_RifleM60,
	L4D2WeaponType_RifleSg552,
	L4D2WeaponType_HuntingRifle,
	L4D2WeaponType_SniperAwp,
	L4D2WeaponType_SniperMilitary,
	L4D2WeaponType_SniperScout,
	L4D2WeaponType_SMG,
	L4D2WeaponType_SMGSilenced,
	L4D2WeaponType_SMGMp5,
	L4D2WeaponType_Autoshotgun,
	L4D2WeaponType_AutoshotgunSpas,
	L4D2WeaponType_Pumpshotgun,
	L4D2WeaponType_PumpshotgunChrome,
	L4D2WeaponType_Molotov,
	L4D2WeaponType_Pipebomb,
	L4D2WeaponType_FirstAid,
	L4D2WeaponType_Pills,
	L4D2WeaponType_Gascan,
	L4D2WeaponType_Oxygentank,
	L4D2WeaponType_Propanetank,
	L4D2WeaponType_Vomitjar,
	L4D2WeaponType_Adrenaline,
	L4D2WeaponType_Chainsaw,
	L4D2WeaponType_Defibrilator,
	L4D2WeaponType_GrenadeLauncher,
	L4D2WeaponType_Melee,
	L4D2WeaponType_UpgradeFire,
	L4D2WeaponType_UpgradeExplosive,
	L4D2WeaponType_BoomerClaw,
	L4D2WeaponType_ChargerClaw,
	L4D2WeaponType_HunterClaw,
	L4D2WeaponType_JockeyClaw,
	L4D2WeaponType_SmokerClaw,
	L4D2WeaponType_SpitterClaw,
	L4D2WeaponType_TankClaw,
	L4D2WeaponType_Gnome
}

//投掷武器.
char g_sWeaponThrow[][][] = 
{
	{"燃烧瓶", "molotov"},
	{"土质炸弹", "pipe_bomb"},
	{"胆汁瓶", "vomitjar"}
};

float  g_fWeaponThrow[sizeof(g_sWeaponThrow)][MAXPLAYERS+1];
float  g_fThrowSpeed[sizeof(g_sWeaponThrow)] = {1.0, ...};
ConVar g_hThrowSpeed[sizeof(g_sWeaponThrow)] = {null, ...};

public Plugin myinfo =  
{
	name = "l4d2_speed_throw",
	author = "豆瓣酱な",  
	description = "设置生还者投掷物的使用速度",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart()
{
	char g_Temp[2][128];
	CreateConVar("l4d2_speed_throw_Version", PLUGIN_VERSION, "设置投掷物的使用速度插件的版本.", CVAR_FLAGS|FCVAR_SPONLY|FCVAR_REPLICATED);
	
	for (int i = 0; i < sizeof(g_sWeaponThrow); i++)
	{
		Format(g_Temp[0], sizeof(g_Temp[]), "l4d2_speed_throw_%s", g_sWeaponThrow[i][1]);
		Format(g_Temp[1], sizeof(g_Temp[]), "设置投掷%s的速度(默认/最低:1.0).", g_sWeaponThrow[i][0]);
		g_hThrowSpeed[i] = CreateConVar(g_Temp[0],"1.0", g_Temp[1], CVAR_FLAGS);
	}
	for (int i = 0; i < sizeof(g_sWeaponThrow); i++)
		g_hThrowSpeed[i].AddChangeHook(SpeedConVarChanged);
	AutoExecConfig(true, "l4d2_speed_throw");
}

public void OnConfigsExecuted()
{	
	ConVarPlayerSpeed();
}

public void SpeedConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	ConVarPlayerSpeed();
}

void ConVarPlayerSpeed()
{
	for (int i = 0; i < sizeof(g_sWeaponThrow); i++)
		g_fThrowSpeed[i] = g_hThrowSpeed[i].FloatValue;
}

//玩家连接成功.
public void OnClientPostAdminCheck(int client)
{
	for (int i = 0; i < sizeof(g_sWeaponThrow); i++)
		g_fWeaponThrow[i][client] = g_fThrowSpeed[i];
}

//快速扔出投掷武器.
public void WH_OnStartThrow(int client, int weapon, L4D2WeaponType weapontype, float &speedmodifier) 
{
	char sTemp[32];
	GetEntityClassname(weapon, sTemp, sizeof(sTemp));
	SplitStringRight(sTemp, "weapon_", sTemp, sizeof(sTemp));
	int iWeaponType = GetWeaponCode(sTemp, g_sWeaponThrow, sizeof(g_sWeaponThrow));
	
	if(iWeaponType != -1)
	{
		speedmodifier = IsSpeedModifier(speedmodifier, g_fWeaponThrow[iWeaponType][client]);
		#if DEBUG
		PrintToChatAll("玩家1:%N,武器:%s,投掷速度:%f(返回值:%d)", client, sTemp, g_fWeaponThrow[iWeaponType][client], iWeaponType);
		#endif
	}
}

//快速扔出投掷武器.
public void WH_OnReadyingThrow(int client, int weapon, L4D2WeaponType weapontype, float &speedmodifier)
{
	char sTemp[32];
	GetEntityClassname(weapon, sTemp, sizeof(sTemp));
	SplitStringRight(sTemp, "weapon_", sTemp, sizeof(sTemp));
	int iWeaponType = GetWeaponCode(sTemp, g_sWeaponThrow, sizeof(g_sWeaponThrow));
	
	if(iWeaponType != -1)
	{
		speedmodifier = IsSpeedModifier(speedmodifier, g_fWeaponThrow[iWeaponType][client]);
		#if DEBUG
		PrintToChatAll("玩家2:%N,武器:%s,投掷速度:%f(返回值:%d)", client, sTemp, g_fWeaponThrow[iWeaponType][client], iWeaponType);
		#endif
	}
}

float IsSpeedModifier(float speedmodifier, float playerspeed) 
{
	if (playerspeed > 1.0)
		speedmodifier = speedmodifier * playerspeed;// multiply current modifier to not overwrite any existing modifiers already

	return speedmodifier;
}

int GetWeaponCode(char[] sCode, char[][][] sName, int iName)
{
	for (int i = 0; i < iName; i++)
		if (strcmp(sCode, sName[i][1]) == 0)
			return i;
	return -1;
}

//这里取右(SplitString 是取左).
stock bool SplitStringRight(const char[] source, const char[] split, char[] part, int partLen)
{
	int index = StrContains(source, split);
	if (index == -1)
		return false;
	
	index += strlen(split);
	if (index == strlen(source) - 1)
		return false;
	
	strcopy(part, partLen, source[index]);
	return true;
}