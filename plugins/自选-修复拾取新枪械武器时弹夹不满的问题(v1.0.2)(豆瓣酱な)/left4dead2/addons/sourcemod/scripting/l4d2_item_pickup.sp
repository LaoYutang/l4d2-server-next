/* 
 *	过渡武器判断嫖其他作者的插件,作者的插件地址↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓
 *	https://forums.alliedmods.net/showthread.php?p=2755917 
 *
 *	v1.0.0
 *
 *	1:初始版本发布.
 *
 *	v1.0.1
 *
 *	1:笑死,测试提示忘记关.
 *
 *	v1.0.2
 *
 *	1:数组里的枪械武器索引被删除时设置值为-1更改为删除此条索引(其实两个方法都行,无影响).
 *
 */

//每行代码结束需填写“;”
#pragma semicolon 1
//强制新语法
#pragma newdecls required
//加载需要的头文件
#include <sourcemod>
#include <left4dhooks>

#define DEBUG	0	//0=禁用调试信息,1=显示调试信息.
#define PLUGIN_VERSION	"1.0.2"

char g_sWeaponName[][][] = 
{
	{"小手枪", "weapon_pistol"},
	{"马格南", "weapon_pistol_magnum"},
	{"UZI微冲", "weapon_smg"},
	{"MP5微冲", "weapon_smg_mp5"},
	{"MAC微冲", "weapon_smg_silenced"},
	{"M16步枪", "weapon_rifle"},
	{"三连发步枪", "weapon_rifle_desert"},
	{"AK47步枪", "weapon_rifle_ak47"},
	{"SG552步枪", "weapon_rifle_sg552"},
	{"连发木狙", "weapon_hunting_rifle"},
	{"军用狙击枪", "weapon_sniper_military"},
	{"单发鸟狙", "weapon_sniper_scout"},
	{"AWP大狙", "weapon_sniper_awp"},
	{"M60机关枪", "weapon_rifle_m60"},
	{"榴弹发射器", "weapon_grenade_launcher"},
	{"单发木喷", "weapon_pumpshotgun"},
	{"单发铁喷", "weapon_shotgun_chrome"},
	{"一代连喷", "weapon_autoshotgun"},
	{"二代连喷", "weapon_shotgun_spas"}
};

int m_nSkin;
bool g_bLateLoad;
ArrayList g_hWeaponIndex;

public APLRes AskPluginLoad2(Handle myself, bool late, char[] error, int err_max)
{
	g_bLateLoad = late;
	return APLRes_Success;
}
public Plugin myinfo =  
{
	name = "l4d2_item_pickup",
	author = "豆瓣酱な",  
	description = "修复拾取新枪械武器时弹夹不满的问题",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart()
{
	g_hWeaponIndex = new ArrayList();//动态大小的数组.
	m_nSkin = FindSendPropInfo("CBaseAnimating", "m_nSkin");
	
	HookEvent("item_pickup", Event_ItemPickup);
	HookEvent("round_start",  Event_RoundStart, EventHookMode_Pre);//回合开始.
	HookEvent("map_transition", Event_MapTransition, EventHookMode_PostNoCopy);

	if(g_bLateLoad)//如果插件延迟加载.
	{
		int entity = MaxClients + 1;
		while ((entity = FindEntityByClassname(entity, "weapon_*")) != INVALID_ENT_REFERENCE)
		{
			char classname[64];
			GetEntityClassname(entity, classname, sizeof(classname));
			if (GetFirearmsWeaponName(classname))
				g_hWeaponIndex.Push(EntIndexToEntRef(entity));//把武器索引写入数组末尾.
		}
	}
}
//回合开始.
public void Event_RoundStart(Event event, const char[] sName, bool bDontBroadcast)
{
	g_hWeaponIndex.Clear();//清除数组内容.
}
public void Event_MapTransition(Event event, const char[] name, bool dontbroadcast)
{
	int entity = MaxClients + 1;
	while ((entity = FindEntityByClassname(entity, "weapon_*")) && IsValidEntity(entity))
		SetEntData(entity, m_nSkin, (entity << 16) | GetEntData(entity, m_nSkin));
}
//创建实体时.
public void OnEntityCreated(int entity, const char[] name)
{
	if (StrContains(name, "weapon_") == -1 || !IsValidEntity(entity) )
		return;
	
	SDKHook(entity, SDKHook_SpawnPost, OnEntitySpawnedSH);
}
public void OnEntitySpawnedSH(int entity)
{	
	RequestFrame(WeaponNextFrame, EntIndexToEntRef(entity));
}
//下一帧函数回调.
void WeaponNextFrame(int entity)
{
	if ((entity = EntRefToEntIndex(entity)) == -1 || !IsValidEntity(entity) || entity <= MaxClients)
		return;
		
	int skin = GetEntData(entity, m_nSkin);
	
	if (skin >> 16 == 0)
		return;
	
	char classname[64];
	GetEntityClassname(entity, classname, sizeof(classname));
	if (GetFirearmsWeaponName(classname))
	{
		g_hWeaponIndex.Push(EntIndexToEntRef(entity));//把武器索引写入数组末尾.
		#if DEBUG
		PrintToChatAll("\x04[提示]\x05创建了%s过渡武器%d.", classname, entity);
		#endif
	}
	SetEntData(entity, m_nSkin, skin & 0xFFFF);
}
//玩家拾取物品时.
public void Event_ItemPickup(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));

	if (IsValidClient(client) && GetClientTeam(client) == 2)
	{
		int iWeapon = GetEntPropEnt(client, Prop_Send, "m_hActiveWeapon");
		if (IsValidEntity(iWeapon))
			RequestFrame(WeaponIndexFrame, EntIndexToEntRef(iWeapon));//这里必须使用下一帧,因为要获取是否为过渡物品.
	}
}
//下一帧回调.
void WeaponIndexFrame(int iWeapon)
{
	if ((iWeapon = EntRefToEntIndex(iWeapon)) == -1 || !IsValidEntity(iWeapon) || iWeapon <= MaxClients)
		return;
	
	char classname[64];
	GetEntityClassname(iWeapon, classname, sizeof(classname));
	if (GetFirearmsWeaponName(classname))
	{
		int index = GetWeaponIndex(iWeapon);
				
		if(index != -1)
		{
			#if DEBUG
			PrintToChatAll("\x04[提示]\x05拾取了%s旧的武器%d.", classname, iWeapon);
			#endif
		}
		else
		{
			g_hWeaponIndex.Push(EntIndexToEntRef(iWeapon));//把武器索引写入数组末尾.
			SetEntProp(iWeapon, Prop_Send, "m_iClip1", L4D2_GetIntWeaponAttribute(classname, L4D2IWA_ClipSize));
			#if DEBUG
			PrintToChatAll("\x04[提示]\x05拾取了%s新的武器%d.", classname, iWeapon);
			#endif
		}
	}
}
//实体删除时.
public void OnEntityDestroyed(int entity)
{
	if(IsValidEntity(entity))
	{
		char classname[64];
		GetEntityClassname(entity, classname, sizeof(classname));

		if(GetFirearmsWeaponName(classname))
		{
			int index = GetWeaponIndex(entity);
			
			if(index != -1)
			{
				g_hWeaponIndex.Erase(index);//删除此条数据.
				//g_hWeaponIndex.Set(index, -1);
				#if DEBUG
				PrintToChatAll("\x04[提示]\x05删除了%s旧的武器%d", classname, entity);
				#endif
			}	
			else
			{
				#if DEBUG
				PrintToChatAll("\x04[提示]\x05删除了%s新的武器%d", classname, entity);
				#endif
			}
		}
	}
}
//获取索引对应的数组位置.
stock int GetWeaponIndex(int entity)
{
	for (int i = 0; i < g_hWeaponIndex.Length; i++)
		if(EntRefToEntIndex(g_hWeaponIndex.Get(i)) == entity)
			return i;
	return -1;
}
//判断当前武器索引在数组里.
stock bool GetFirearmsWeaponName(char[] classname)
{
	for (int i = 0; i < sizeof(g_sWeaponName); i++)
		if (strcmp(classname, g_sWeaponName[i][1]) == 0)
			return true;

	return false;
}
//判断玩家有效.
stock bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}