/*
 * v1.1.3
 *	
 *	1:设置火焰伤害还是得使用 SDKHooks_TakeDamage 的方法,不然统计类插件结果严重不准.
 *
 * v1.1.4
 *	
 *	1:设置伤害函数后面不能设置类型,否则会疯狂报错.
 *
 *
 */
#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#include <sdkhooks>
#include <sdktools>

#define PLUGIN_VERSION "1.1.4"

#define DMG_FIRE	2056
#define DMG_ONFIRE	268435464

bool g_bLateLoad;

int    g_iTankDamage;
ConVar g_hTankDamage;

public Plugin myinfo =  
{
	name = "重置火焰伤害",
	author = "豆瓣酱な",  
	description = "重置火焰对坦克的伤害",
	version = PLUGIN_VERSION,
	url = "N/A"
};

public APLRes AskPluginLoad2(Handle myself, bool late, char[] error, int err_max)
{
	g_bLateLoad = late;
	return APLRes_Success;
}

public void OnPluginStart()
{
	g_hTankDamage = CreateConVar("l4d2_tank_damage", "2", "设置坦克每次受到多少火焰伤害(约0.18秒/次). -1=禁用, 0=免疫火焰伤害.", FCVAR_NOTIFY);
	g_hTankDamage.AddChangeHook(ConVarChanged);
	AutoExecConfig(true, "l4d2_tank_damage");

	if(g_bLateLoad)//如果插件延迟加载.
		for(int i = 1; i <= MaxClients; i++)
			if(IsValidClient(i) && IsValidTank(i))
				OnClientPutInServer(i);
}

public void ConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	IsTankDamage();
}

public void OnMapStart()
{
	IsTankDamage();
}

void IsTankDamage()
{
	g_iTankDamage = g_hTankDamage.IntValue;
}

public void OnClientPutInServer(int client)
{
	SDKHook(client, SDKHook_OnTakeDamageAlive, OnTakeDamageAlive);//重新设置坦克受到的火焰伤害.
}

//重新设置坦克受到的火焰伤害.
public Action OnTakeDamageAlive(int client, int &attacker, int &inflictor, float &damage, int &damagetype, int &weapon, float damageForce[3], float damagePosition[3], int damagecustom)
{ 
	if(g_iTankDamage < 0)
		return Plugin_Continue;

	if(IsValidClient(client) && IsValidTank(client))
	{	
		if(damagetype == DMG_BURN  || damagetype == DMG_FIRE || damagetype == DMG_ONFIRE)
		{
			if(g_iTankDamage == 0)
			{
				if(GetEntityFlags(client) & FL_ONFIRE)//判断客户端是否着火.
					ExtinguishEntity(client);//灭火.
				return Plugin_Handled;
			}
			else
			{
				int iHealth = GetClientHealth(client);
				
				if (iHealth > g_iTankDamage)
				{
					SDKHooks_TakeDamage(client, inflictor, attacker, float(g_iTankDamage));//设置指定的伤害.
					//PrintToChat(attacker, "[提示]攻击者%N|%N,血量:%d,伤害:%f,类型:%d.", attacker, client, iHealth, damage, damagetype);
					return Plugin_Handled;//阻止原始伤害.
				}
			}
		}
	}
	return Plugin_Continue;
}

stock bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}

stock bool IsValidTank(int client)  
{
	return GetClientTeam(client) == 3 && IsPlayerAlive(client) && GetEntProp(client, Prop_Send, "m_zombieClass") == 8;
}