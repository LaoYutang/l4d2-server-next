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

float  g_fWeaponDeploy[MAXPLAYERS+1] = {1.0, ...};
float  g_fDeploySpeed = 1.0;
ConVar g_hDeploySpeed = null;

public Plugin myinfo =  
{
	name = "l4d2_speed_deploy",
	author = "豆瓣酱な",  
	description = "设置生还者其它加速",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart()
{
	CreateConVar("l4d2_speed_deploy_Version", PLUGIN_VERSION, "设置生还者其它加速插件的版本.", CVAR_FLAGS|FCVAR_SPONLY|FCVAR_REPLICATED);
	g_hDeploySpeed = CreateConVar("l4d2_speed_deploy",	"1.0",	"设置生还者其它加速,(默认/最低:1.0).", CVAR_FLAGS);
	g_hDeploySpeed.AddChangeHook(SpeedConVarChanged);
	AutoExecConfig(true, "l4d2_speed_deploy");
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
	g_fDeploySpeed = g_hDeploySpeed.FloatValue;
}

//玩家连接成功.
public void OnClientPostAdminCheck(int client)
{
	g_fWeaponDeploy[client] = g_fDeploySpeed;
}

//切换物品快速准备.
public void WH_OnDeployModifier(int client, int weapon, L4D2WeaponType weapontype, float &speedmodifier) 
{
	#if DEBUG
	char sTemp[32];
	GetEntityClassname(weapon, sTemp, sizeof(sTemp));
	PrintToChatAll("玩家:%N,名称:%s,准备速度:%f", client, sTemp, g_fWeaponDeploy[client]);
	#endif
	speedmodifier = IsSpeedModifier(speedmodifier, g_fWeaponDeploy[client]);
	
}

float IsSpeedModifier(float speedmodifier, float playerspeed) 
{
	if (playerspeed > 1.0)
		speedmodifier = speedmodifier * playerspeed;// multiply current modifier to not overwrite any existing modifiers already

	return speedmodifier;
}