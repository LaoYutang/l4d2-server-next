#pragma semicolon 1
#pragma newdecls required
#include <sourcemod>

#define CVAR_FLAGS 		FCVAR_NOTIFY
#define PLUGIN_VERSION "1.0.1"
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

//近战.
char g_sWeaponMelee[][][] = 
{
	//{"电锯", "chainsaw"},
	//{"盾牌", "riotshield"},
	{"斧头", "fireaxe"},
	{"平底锅", "frying_pan"},
	{"砍刀", "machete"},
	{"棒球棒", "baseball_bat"},
	{"撬棍", "crowbar"},
	{"球拍", "cricket_bat"},
	{"警棍", "tonfa"},
	{"武士刀", "katana"},
	{"电吉他", "electric_guitar"},
	{"小刀", "knife"},
	{"高尔夫球棍", "golfclub"},
	{"铁铲", "shovel"},
	{"草叉", "pitchfork"}
};

float  g_fWeaponMelee[sizeof(g_sWeaponMelee)][MAXPLAYERS+1];
float  g_fMeleeSpeed[sizeof(g_sWeaponMelee)] = {1.0, ...};
ConVar g_hMeleeSpeed[sizeof(g_sWeaponMelee)] = {null, ...};

public Plugin myinfo =  
{
	name = "l4d2_speed_melee",
	author = "豆瓣酱な",  
	description = "设置生还者的近战攻击速度",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart()
{
	char g_Temp[2][128];
	CreateConVar("l4d2_speed_melee_Version", PLUGIN_VERSION, "设置近战攻击速度插件的版本.", CVAR_FLAGS|FCVAR_SPONLY|FCVAR_REPLICATED);
	
	for (int i = 0; i < sizeof(g_sWeaponMelee); i++)
	{
		Format(g_Temp[0], sizeof(g_Temp[]), "l4d2_speed_melee_%s", g_sWeaponMelee[i][1]);
		Format(g_Temp[1], sizeof(g_Temp[]), "设置近战%s攻击速度(默认/最低:1.0,建议最大:3.5).", g_sWeaponMelee[i][0]);
		g_hMeleeSpeed[i] = CreateConVar(g_Temp[0],"1.0", g_Temp[1], CVAR_FLAGS);
	}
	for (int i = 0; i < sizeof(g_sWeaponMelee); i++)
		g_hMeleeSpeed[i].AddChangeHook(SpeedConVarChanged);
	AutoExecConfig(true, "l4d2_speed_melee");
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
	for (int i = 0; i < sizeof(g_sWeaponMelee); i++)
		g_fMeleeSpeed[i] = g_hMeleeSpeed[i].FloatValue;
}

//玩家连接成功.
public void OnClientPostAdminCheck(int client)
{
	for (int i = 0; i < sizeof(g_sWeaponMelee); i++)
		g_fWeaponMelee[i][client] = g_fMeleeSpeed[i];
}

//近战攻速.
public void WH_OnMeleeSwing(int client, int weapon, float &speedmodifier)
{
	char sTemp[32];
	GetEntityClassname(weapon, sTemp, sizeof(sTemp));
	if (strcmp(sTemp, "weapon_melee") == 0 && HasEntProp(weapon, Prop_Data, "m_strMapSetScriptName"))
	{
		GetEntPropString(weapon, Prop_Data, "m_strMapSetScriptName", sTemp, sizeof(sTemp));
		int iWeaponType = GetWeaponCode(sTemp, g_sWeaponMelee, sizeof(g_sWeaponMelee));
		if(iWeaponType != -1)
		{
			speedmodifier = IsSpeedModifier(speedmodifier, g_fWeaponMelee[iWeaponType][client]);
			#if DEBUG
			PrintToChatAll("玩家:%N,武器:%s,近战速度:%f(返回值:%d)", client, sTemp, g_fWeaponMelee[iWeaponType][client], iWeaponType);
			#endif
		}
	}
}

int GetWeaponCode(char[] sCode, char[][][] sName, int iName)
{
	for (int i = 0; i < iName; i++)
		if (strcmp(sCode, sName[i][1]) == 0)
			return i;
	return -1;
}

float IsSpeedModifier(float speedmodifier, float playerspeed) 
{
	if (playerspeed > 1.0)
		speedmodifier = speedmodifier * playerspeed;// multiply current modifier to not overwrite any existing modifiers already

	return speedmodifier;
}