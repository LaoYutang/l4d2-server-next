#pragma semicolon 1
#pragma newdecls required
#include <sourcemod>
#include <sdkhooks>
#include <sdktools>

ConVar g_hMeleeTankDamage, g_hMeleeWitchDamage;

float g_fMeleeTankDamage, g_fMeleeWitchDamage;

bool g_bLateLoad;

public Plugin myinfo =
{
	name = "L4D2 AntiMelee",
	description = "Nerfes melee damage against tanks by a set amount of %",
	author = "Visor",
	version = "1.0",
	url = "https://github.com/Attano/L4D2-Competitive-Framework"
};

public APLRes AskPluginLoad2(Handle myself, bool late, char[] error, int err_max)
{
	g_bLateLoad = late;
	return APLRes_Success;
}

public void OnPluginStart()
{
	if(g_bLateLoad)
	{
		for(int i = 1; i <= MaxClients; i++)
		{
			if(IsClientInGame(i))
				SDKHook(i, SDKHook_OnTakeDamage, TankOnTakeDamage);
		}
		
		int entity = MaxClients + 1;
		while((entity = FindEntityByClassname(entity, "witch")) != INVALID_ENT_REFERENCE)
			SDKHook(entity, SDKHook_OnTakeDamage, WitchOnTakeDamage);
	}

	g_hMeleeTankDamage	= CreateConVar("l4d2_melee_tank_damage", "200.0", "设置近战对坦克的伤害. -1.0=禁用,0.0=免疫近战伤害,0.01=1%,类推,1.0=100%(1刀死),大于1.0=实际伤害.", FCVAR_NOTIFY);
	g_hMeleeWitchDamage	= CreateConVar("l4d2_melee_witch_damage", "-1.0", "设置近战对女巫的伤害. -1.0=禁用,0.0=免疫近战伤害,0.01=1%,类推,1.0=100%(1刀死),大于1.0=实际伤害.", FCVAR_NOTIFY);

	g_hMeleeTankDamage.AddChangeHook(ConVarChanged);
	g_hMeleeWitchDamage.AddChangeHook(ConVarChanged);
	
	AutoExecConfig(true, "l4d2_antimelee");//生成指定文件名的CFG.
}

public void OnConfigsExecuted()
{
	GetCvars();
}

public void ConVarChanged(Handle convar, const char[] oldValue, const char[] newValue)
{
	GetCvars();
}

void GetCvars()
{
	g_fMeleeTankDamage = g_hMeleeTankDamage.FloatValue;
	g_fMeleeWitchDamage = g_hMeleeWitchDamage.FloatValue;
}

public void OnClientPutInServer(int client)
{
	SDKHook(client, SDKHook_OnTakeDamage, TankOnTakeDamage);
}

public void OnEntityCreated (int entity, const char[] classname)
{
	if(entity <= MaxClients || !IsValidEntity(entity))
		return;
		
	if(classname[0] != 'w')
		return;
		
	if(strcmp(classname, "witch") == 0)
		SDKHook(entity, SDKHook_OnTakeDamage, WitchOnTakeDamage);
}

public Action TankOnTakeDamage(int victim, int &attacker, int &inflictor, float &damage, int &damagetype, int &weapon, float damageForce[3], float damagePosition[3])
{
	if(g_fMeleeTankDamage < 0.0 || damage == 0.0 || !IsSurvivor(attacker) || !IsTank(victim) || !IsMelee(inflictor))
		return Plugin_Continue;
	
	if(g_fMeleeTankDamage == 0.0)
	{
		PrintHintText(attacker, "坦克免疫近战伤害.");
		return Plugin_Handled;
	}
	else
	{
		damage = g_fMeleeTankDamage > 1.0 ? g_fMeleeTankDamage : g_fMeleeTankDamage * GetEntProp(victim, Prop_Data, "m_iMaxHealth");
		PrintHintText(attacker, "你的近战对坦克造成了%d点伤害.", RoundFloat(damage));
	}
	return Plugin_Changed;
}

public Action WitchOnTakeDamage(int victim, int &attacker, int &inflictor, float &damage, int &damagetype, int &weapon, float damageForce[3], float damagePosition[3])
{
	if(g_fMeleeWitchDamage < 0.0 || damage == 0.0 || !IsSurvivor(attacker) || !IsWitch(victim) || !IsMelee(inflictor))
		return Plugin_Continue;
	
	if(g_fMeleeWitchDamage == 0.0)
	{
		PrintHintText(attacker, "女巫免疫近战伤害.");
		return Plugin_Handled;
	}
	else
	{
		damage = g_fMeleeWitchDamage > 1.0 ? g_fMeleeWitchDamage : g_fMeleeWitchDamage * GetEntProp(victim, Prop_Data, "m_iMaxHealth");
		PrintHintText(attacker, "你的近战对女巫造成了%d点伤害.", RoundFloat(damage));
	}
	return Plugin_Changed;
}

bool IsSurvivor(int attacker)
{
	return attacker > 0 && attacker <= MaxClients && IsClientInGame(attacker) && GetClientTeam(attacker) == 2;
}

bool IsMelee(int inflictor)
{
	if(inflictor > MaxClients)
	{
		static char classname[13];
		GetEdictClassname(inflictor, classname, sizeof(classname));
		return strcmp(classname, "weapon_melee") == 0;
	}
	return false;
}

bool IsTank(int victim)
{
	return victim > 0 && victim <= MaxClients && IsClientInGame(victim) && GetClientTeam(victim) == 3 && GetEntProp(victim, Prop_Send, "m_zombieClass") == 8;
}

bool IsWitch(int victim)
{
	if(victim > MaxClients)
	{
		static char classname[6];
		GetEdictClassname(victim, classname, sizeof(classname));
		return strcmp(classname, "witch") == 0;
	}
	return false;
}
