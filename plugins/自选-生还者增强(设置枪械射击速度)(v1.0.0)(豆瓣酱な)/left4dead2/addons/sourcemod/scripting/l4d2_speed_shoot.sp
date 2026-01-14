#pragma semicolon 1
#pragma newdecls required
#include <sourcemod>

#define CVAR_FLAGS 		FCVAR_NOTIFY
#define PLUGIN_VERSION "1.0.0"
#define DEBUG	0	//0=禁用调试信息,1=显示调试信息.

enum L4D2WeaponType {
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

//枪械.
char g_sWeaponShoot[][][] = 
{
	{"小手枪", "pistol"},
	{"马格南", "pistol_magnum"},
	{"微型冲锋枪", "smg"},
	{"MP5冲锋枪", "smg_mp5"},
	{"消音冲锋枪", "smg_silenced"},
	{"单发木喷", "pumpshotgun"},
	{"铁喷木喷", "shotgun_chrome"},
	{"M16步枪", "rifle"},
	{"三连发步枪", "rifle_desert"},
	{"AK47步枪", "rifle_ak47"},
	{"SG552步枪", "rifle_sg552"},
	{"一代连喷", "autoshotgun"},
	{"二代连喷", "shotgun_spas"},
	{"连发木狙", "hunting_rifle"},
	{"军用狙击枪", "sniper_military"},
	{"单发鸟狙", "sniper_scout"},
	{"AWP大狙", "sniper_awp"},
	{"M60机关枪", "rifle_m60"},
	{"榴弹发射器", "grenade_launcher"}
};

float  g_fWeaponShoot[sizeof(g_sWeaponShoot)][MAXPLAYERS+1];
float  g_fShootSpeed[sizeof(g_sWeaponShoot)] = {1.0, ...};//设置默认值(创建CFG也使用这个).
ConVar g_hShootSpeed[sizeof(g_sWeaponShoot)] = {null, ...};

public Plugin myinfo =  
{
	name = "l4d2_speed_shoot",
	author = "豆瓣酱な",  
	description = "设置生还者的枪械射击速度",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart()
{
	char sTemp[3][128];
	CreateConVar("l4d2_speed_shoot_Version", PLUGIN_VERSION, "设置枪械射击速度插件的版本.", CVAR_FLAGS|FCVAR_SPONLY|FCVAR_REPLICATED);
	
	for (int i = 0; i < sizeof(g_sWeaponShoot); i++)
	{
		Format(sTemp[0], sizeof(sTemp[]), "l4d2_speed_shoot_%s", g_sWeaponShoot[i][1]);
		Format(sTemp[1], sizeof(sTemp[]), "设置枪械%s的射击速度(默认/最低:1.0).", g_sWeaponShoot[i][0]);
		FloatToString(g_fShootSpeed[i], sTemp[2], sizeof(sTemp[]));
		g_hShootSpeed[i] = CreateConVar(sTemp[0], sTemp[2], sTemp[1], CVAR_FLAGS);
	}
	for (int i = 0; i < sizeof(g_sWeaponShoot); i++)
		g_hShootSpeed[i].AddChangeHook(SpeedConVarChanged);
	AutoExecConfig(true, "l4d2_speed_shoot");
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
	for (int i = 0; i < sizeof(g_sWeaponShoot); i++)
		g_fShootSpeed[i] = g_hShootSpeed[i].FloatValue;
}

//玩家连接成功.
public void OnClientPostAdminCheck(int client)
{
	for (int i = 0; i < sizeof(g_sWeaponShoot); i++)
		g_fWeaponShoot[i][client] = g_fShootSpeed[i];
}

//快速射击.
public void WH_OnGetRateOfFire(int client, int weapon, L4D2WeaponType weapontype, float &speedmodifier) 
{
	char sTemp[32];
	GetEntityClassname(weapon, sTemp, sizeof(sTemp));
	SplitStringRight(sTemp, "weapon_", sTemp, sizeof(sTemp));
	int iWeaponType = GetWeaponCode(sTemp, g_sWeaponShoot, sizeof(g_sWeaponShoot));
	
	if(iWeaponType != -1)
	{
		speedmodifier = IsSpeedModifier(speedmodifier, g_fWeaponShoot[iWeaponType][client]);
		#if DEBUG
		PrintToChatAll("玩家:%N,武器:%s,射击速度:%f(返回值:%d)", client, sTemp, g_fWeaponShoot[iWeaponType][client], iWeaponType);
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