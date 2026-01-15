
#pragma semicolon 1
#include <sourcemod>
#include <sdktools>
#include <sdkhooks>

#define PLUGIN_VERSION "1.0.5+"
#define CVAR_FLAGS FCVAR_NOTIFY

#define IsValidClient(%1)		(1 <= %1 <= MaxClients && IsClientInGame(%1))


ConVar plugin_enable;
ConVar sound_enable;
ConVar pic_enable;

new Handle:Time = INVALID_HANDLE;
new Handle:sound_1 = INVALID_HANDLE;
new Handle:sound_2 = INVALID_HANDLE;
new Handle:sound_3 = INVALID_HANDLE;
new Handle:hit1 = INVALID_HANDLE;
new Handle:hit2 = INVALID_HANDLE;
new Handle:hit3 = INVALID_HANDLE;
new Handle:g_blast = INVALID_HANDLE;

enum {
	kill_1,
	hit_armor,
	kill
};

new Handle:g_taskCountdown[33] = INVALID_HANDLE,Handle:g_taskClean[33] = INVALID_HANDLE;
new g_killCount[33] = 0;
new bool:g_bShowAuthor[33] = false;
new bool:IsVictimDeadPlayer[MAXPLAYERS+1] = { false, ... };

public Plugin:myinfo = 
{
	name = "击中反馈",
	author = "TsukasaSato",
	description = "自定义击中和击杀的图标、声音、时长",
	version = "PLUGIN_VERSION"
}

public OnPluginStart()
{
	decl String:Game_Name[64];
	GetGameFolderName(Game_Name, sizeof(Game_Name));
	if(!StrEqual(Game_Name, "left4dead2", false))
	{
		SetFailState("本插件仅支持L4D2!");
	}

	CreateConVar("l4d2_hitsound", PLUGIN_VERSION, "Plugin version", 0);
	Time = CreateConVar("sm_hitsound_showtime", "0.3", "图标存在的时长(默认为0.3)");
	sound_1 = CreateConVar("sm_hitsound_mp3_headshot", "hitsound/headshot.mp3", "爆头音效的地址");	
	sound_2 = CreateConVar("sm_hitsound_mp3_hit", "hitsound/hit.mp3", "击中音效的地址");
	sound_3 = CreateConVar("sm_hitsound_mp3_kill", "hitsound/kill.mp3", "击杀音效的地址");
	hit1 = CreateConVar("sm_hitsound_pic_headshot", "overlays/head", "爆头图标的地址");
	hit2 = CreateConVar("sm_hitsound_pic_hit", "overlays/body", "击中图标的地址");
	hit3 = CreateConVar("sm_hitsound_pic_kill", "overlays/head", "击杀图标的地址");
	
	sound_enable = CreateConVar("sm_hitsound_sound_enable", "1", "是否开启音效(0-关, 1-开)", CVAR_FLAGS);
	pic_enable = CreateConVar("sm_hitsound_pic_enable", "1", "是否开启及击杀图标(0-关, 1-开)", CVAR_FLAGS);
	g_blast = CreateConVar("sm_blast_damage_enable", "0", "是否开启爆炸反馈提示(0-关, 1-开 建议关闭)", CVAR_FLAGS);
	
	plugin_enable 		= CreateConVar("sm_hitsound_enable","1","是否开启本插件(0-关, 1-开)", CVAR_FLAGS);
	AutoExecConfig(true, "l4d2_hitsound");//是否生成cfg注释即不生成
	if (GetConVarInt(plugin_enable) == 1)
	{
		HookEvent("infected_hurt",			Event_InfectedHurt, EventHookMode_Pre); //感染受伤
		HookEvent("infected_death",			Event_InfectedDeath); //感染死亡
		HookEvent("player_death",			Event_PlayerDeath); // 玩家死亡
		HookEvent("player_hurt",				Event_PlayerHurt, EventHookMode_Pre); //玩家受伤
		HookEvent("tank_spawn", Event_TankSpawn);
		HookEvent("player_spawn", Event_Spawn);
		HookEvent("round_start", Event_round_start,EventHookMode_Post);
		HookEvent("player_incapacitated", PlayerIncap);
	}
}

public void Event_Spawn(Event event, const char[] name, bool dontBroadcast)
{
	new Client = GetClientOfUserId(GetEventInt(event, "userid"));
	IsVictimDeadPlayer[Client] = false;
}


public Action:PlayerIncap(Handle:event, const String:name[], bool:dontBroadcast)
{
	new victim = GetClientOfUserId(GetEventInt(event, "userid"));
	if(IsValidClient(victim) && GetClientTeam(victim) == 3 && GetEntProp(victim, Prop_Send, "m_zombieClass") == 8)
	IsVictimDeadPlayer[victim] = true;
}

public Action:Event_TankSpawn(Handle:event, const String:name[], bool:dontBroadcast)
{
	new tank = GetClientOfUserId(GetEventInt(event, "userid"));
	IsVictimDeadPlayer[tank] = false;
}

public Action:Event_PlayerDeath(Handle:event, const String:name[], bool:dontBroadcast)
{
	new victim = GetClientOfUserId(GetEventInt(event, "userid"));
	new attacker = GetClientOfUserId(GetEventInt(event, "attacker"));
	new bool:heatshout = false;
	heatshout = GetEventBool(event, "headshot");
	new IsHeatshout = 0;
	new damagetype = GetClientOfUserId(GetEventInt(event, "type"));

	if(damagetype & DMG_DIRECT)
		return Plugin_Changed;
		
	if(GetConVarInt(g_blast) == 0 && damagetype & DMG_BLAST)
		return Plugin_Changed;

	if (heatshout) IsHeatshout = 1;
	
	if(IsValidClient(victim))
	{
		if(GetClientTeam(victim) == 3)
		{
			if(IsValidClient(attacker))
			{
				if(GetClientTeam(attacker) == 2)	
				{
					if(!IsFakeClient(attacker))
					{
						if(IsHeatshout)
						{
							if(GetConVarInt(pic_enable) == 1)
							{
								ShowKillMessage(attacker,kill_1);
							}
							new String:sound1[64]; 
							GetConVarString(sound_1, sound1, sizeof(sound1));
							if (GetConVarInt(sound_enable) == 1)
							{
								PrecacheSound(sound1, true);
								EmitSoundToClient(attacker, sound1, SOUND_FROM_PLAYER, SNDCHAN_STATIC, SNDLEVEL_NORMAL, SND_NOFLAGS, SNDVOL_NORMAL, SNDPITCH_NORMAL, -1, NULL_VECTOR, NULL_VECTOR, true, 0.0);
							}
							if(g_taskClean[attacker] != INVALID_HANDLE)
							{
								KillTimer(g_taskClean[attacker]);
								g_taskClean[attacker] = INVALID_HANDLE;
							}
							new Float:showtime = GetConVarFloat(Time);
							g_taskClean[attacker] = CreateTimer(showtime,task_Clean,attacker);
							}else{
							if (GetConVarInt(pic_enable) == 1)
							{
								ShowKillMessage(attacker,kill);
							}
							if(g_taskClean[attacker] != INVALID_HANDLE)
							{
								KillTimer(g_taskClean[attacker]);
								g_taskClean[attacker] = INVALID_HANDLE;
							}
							new Float:showtime = GetConVarFloat(Time);
							g_taskClean[attacker] = CreateTimer(showtime,task_Clean,attacker);
							new String:sound3[64];
							GetConVarString(sound_3, sound3, sizeof(sound3));
							if (GetConVarInt(sound_enable) == 1)
							{
								PrecacheSound(sound3, true);
								EmitSoundToClient(attacker,sound3, SOUND_FROM_PLAYER, SNDCHAN_STATIC, SNDLEVEL_NORMAL, SND_NOFLAGS, SNDVOL_NORMAL, SNDPITCH_NORMAL, -1, NULL_VECTOR, NULL_VECTOR, true, 0.0);
							}				
						}
					}
				}
			}
		}
	}
	return Plugin_Continue;
}

public Action:Event_PlayerHurt(Handle:event, const String:name[], bool:dontBroadcast)
{
	new victim = GetClientOfUserId(GetEventInt(event, "userid"));
	new attacker = GetClientOfUserId(GetEventInt(event, "attacker"));
	new dmg = GetEventInt(event, "dmg_health");
	new eventhealth = GetEventInt(event, "health");
	new damagetype = GetClientOfUserId(GetEventInt(event, "type"));
	decl String:WeaponName[64];
	GetEventString(event, "weapon", WeaponName, sizeof(WeaponName));
//火inferno
//火entityflame
	new bool:Isinferno = false;

	if(damagetype & DMG_DIRECT)
        return Plugin_Changed;

	if(GetConVarInt(g_blast) == 0 && damagetype & DMG_BLAST)
        return Plugin_Changed;
		
	if(strcmp(WeaponName, "entityflame", false) == 0)
	{
		Isinferno = true;
	}else{
	if(strcmp(WeaponName, "inferno", false) == 0)
		Isinferno = true;
	}
	if(IsValidClient(victim))
	{
		if(IsValidClient(attacker))
		{
			if(!IsFakeClient(attacker))
			{
				if(GetClientTeam(victim) == 3)
				{
					new Float:AddDamage = 0.0;
					if(RoundToNearest(eventhealth - dmg - AddDamage) <= 0)
					{
						IsVictimDeadPlayer[victim] = true;
					}
					if(IsVictimDeadPlayer[victim] == false)
					{
					if (GetConVarInt(pic_enable) == 1)
					{
						ShowKillMessage(attacker,hit_armor);
					}
					new String:sound2[64];
					GetConVarString(sound_2, sound2, sizeof(sound2));				
					if (GetConVarInt(sound_enable) == 1 && Isinferno == false)
					{
						//PrintToChatAll("获取到的武器是%s", WeaponName);
						PrecacheSound(sound2, true);
						EmitSoundToClient(attacker, sound2, SOUND_FROM_PLAYER, SNDCHAN_STATIC, SNDLEVEL_NORMAL, SND_NOFLAGS, SNDVOL_NORMAL, SNDPITCH_NORMAL, -1, NULL_VECTOR, NULL_VECTOR, true, 0.0);
					}
					if(g_taskClean[attacker] != INVALID_HANDLE)
					{
						KillTimer(g_taskClean[attacker]);
						g_taskClean[attacker] = INVALID_HANDLE;
					}
					new Float:showtime = GetConVarFloat(Time);
					g_taskClean[attacker] = CreateTimer(showtime,task_Clean,attacker);
					}
				}
			}
		}
	}
	return Plugin_Changed;
}

public Action:Event_InfectedDeath(Handle:event, const String:name[], bool:dontBroadcast)
{
	new victim = GetEventInt(event, "infected_id");
	decl String:sname[32];
	GetEdictClassname(victim, sname, sizeof(sname));
	new attacker = GetClientOfUserId(GetEventInt(event, "attacker"));
	new bool:heatshout = false;
	heatshout = GetEventBool(event, "headshot");
	new bool:damagetype = GetEventBool(event, "blast");
	new IsHeatshout = 0;
	new WeaponID = GetEventInt(event, "weapon_id");
	if (heatshout) IsHeatshout = 1;

	if(WeaponID == 0)
    return Plugin_Changed;

	if(GetConVarInt(g_blast) == 0 && damagetype)
    return Plugin_Changed;

	if(IsValidClient(attacker))
	{
	if (IsHeatshout)
	{
		if(GetClientTeam(attacker) == 2)	
		{
			if(!IsFakeClient(attacker))
			{
				if (GetConVarInt(pic_enable) == 1)
				{
				ShowKillMessage(attacker,kill_1);
				}
				new String:sound1[64];
				GetConVarString(sound_1, sound1, sizeof(sound1));
				if (GetConVarInt(sound_enable) == 1)
				{
				PrecacheSound(sound1, true);
				EmitSoundToClient(attacker, sound1, SOUND_FROM_PLAYER, SNDCHAN_STATIC, SNDLEVEL_NORMAL, SND_NOFLAGS, SNDVOL_NORMAL, SNDPITCH_NORMAL, -1, NULL_VECTOR, NULL_VECTOR, true, 0.0);
				}
				if(g_taskClean[attacker] != INVALID_HANDLE)
				{
				KillTimer(g_taskClean[attacker]);
				g_taskClean[attacker] = INVALID_HANDLE;
				}
				new Float:showtime = GetConVarFloat(Time);
				g_taskClean[attacker] = CreateTimer(showtime,task_Clean,attacker);
			}
		}
	}
	else 
	{
	if(GetClientTeam(attacker) == 2)	
		{
			if(!IsFakeClient(attacker))
			{
			if (GetConVarInt(pic_enable) == 1)
			{
				ShowKillMessage(attacker,kill);
			}
			new String:sound3[64];
			GetConVarString(sound_3, sound3, sizeof(sound3));
			if (GetConVarInt(sound_enable) == 1)
			{
				//PrintToChatAll("获取到的id是%i", WeaponID);
				PrecacheSound(sound3, true);
				EmitSoundToClient(attacker, sound3, SOUND_FROM_PLAYER, SNDCHAN_STATIC, SNDLEVEL_NORMAL, SND_NOFLAGS, SNDVOL_NORMAL, SNDPITCH_NORMAL, -1, NULL_VECTOR, NULL_VECTOR, true, 0.0);
			}
			if(g_taskClean[attacker] != INVALID_HANDLE)
			{
				KillTimer(g_taskClean[attacker]);
				g_taskClean[attacker] = INVALID_HANDLE;
			}
			new Float:showtime = GetConVarFloat(Time);
			g_taskClean[attacker] = CreateTimer(showtime,task_Clean,attacker);
				}
			}
		}
	}
	return Plugin_Continue;
}
/*
public OnMapStart()
{
	char sounda[64];
	GetConVarString(sound_1, sounda, sizeof(sounda));
	char soundb[64];
	GetConVarString(sound_2, soundb, sizeof(soundb));
	char soundc[64];
	GetConVarString(sound_3, soundc, sizeof(soundc));
	if (!IsSoundPrecached(sounda)) PrecacheSound(sounda, true);
	if (!IsSoundPrecached(soundb)) PrecacheSound(soundb, true);
	if (!IsSoundPrecached(soundc)) PrecacheSound(soundc, true);
}
*/

public Action:Event_InfectedHurt(Handle:event, String:event_name[], bool:dontBroadcast)
{
	new victim = GetEventInt(event, "entityid");
	decl String:sname[32];
	new attacker = GetClientOfUserId(GetEventInt(event, "attacker"));
	new dmg = GetEventInt(event, "amount");
	new eventhealth = GetEntProp(victim, Prop_Data, "m_iHealth");
	new bool:IsVictimDead = false;
	new damagetype = GetClientOfUserId(GetEventInt(event, "type"));

	if(damagetype & DMG_DIRECT)
        return Plugin_Changed;

	if(GetConVarInt(g_blast) == 0 && damagetype & DMG_BLAST)
        return Plugin_Changed;

	if(IsValidClient(attacker))
	{
	if(!IsFakeClient(attacker))
		{
	if((eventhealth - dmg) <= 0)
			{
				IsVictimDead = true;
			}
	if(!IsVictimDead)
	{
		if (StrEqual(sname, "witch"))
		{
			if (GetConVarInt(pic_enable) == 1)
			{
			ShowKillMessage(attacker,hit_armor);
			}
			new String:sound2[64];
			GetConVarString(sound_2, sound2, sizeof(sound2));
			if (GetConVarInt(sound_enable) == 1)
			{
				PrecacheSound(sound2, true);
				EmitSoundToClient(attacker, sound2, SOUND_FROM_PLAYER, SNDCHAN_STATIC, SNDLEVEL_NORMAL, SND_NOFLAGS, SNDVOL_NORMAL, SNDPITCH_NORMAL, -1, NULL_VECTOR, NULL_VECTOR, true, 0.0);
			}
			if(g_taskClean[attacker] != INVALID_HANDLE)
			{
				KillTimer(g_taskClean[attacker]);
				g_taskClean[attacker] = INVALID_HANDLE;
			}
			new Float:showtime = GetConVarFloat(Time);
			g_taskClean[attacker] = CreateTimer(showtime,task_Clean,attacker);
			}else{
			if (GetConVarInt(pic_enable) == 1)
			{
				ShowKillMessage(attacker,hit_armor);
			}
			new String:sound2[64];
			GetConVarString(sound_2, sound2, sizeof(sound2));
			if (GetConVarInt(sound_enable) == 1)
			{
				PrecacheSound(sound2, true);
				EmitSoundToClient(attacker, sound2, SOUND_FROM_PLAYER, SNDCHAN_STATIC, SNDLEVEL_NORMAL, SND_NOFLAGS, SNDVOL_NORMAL, SNDPITCH_NORMAL, -1, NULL_VECTOR, NULL_VECTOR, true, 0.0);
			}
			if(g_taskClean[attacker] != INVALID_HANDLE)
			{
				KillTimer(g_taskClean[attacker]);
				g_taskClean[attacker] = INVALID_HANDLE;
			}
			new Float:showtime = GetConVarFloat(Time);
			g_taskClean[attacker] = CreateTimer(showtime,task_Clean,attacker);
				}
			}
		}
	}
	return Plugin_Changed;
}

public Event_round_start(Handle:event,const String:name[],bool:dontBroadcast)
{
	for(new client=1;client <= MaxClients;client++)
	{
		g_killCount[client] = 0;
		if(g_taskCountdown[client] != INVALID_HANDLE)
		{
			KillTimer(g_taskCountdown[client]);
			g_taskCountdown[client] = INVALID_HANDLE;
		}
		g_bShowAuthor[client] = GetRandomInt(1,3)==1 ? true : false;
	}
}

public Action:task_Countdown(Handle:Timer, any:client)
{
	g_killCount[client] --;
	if(!IsPlayerAlive(client) || g_killCount[client]==0)
	{
		KillTimer(Timer);
		g_taskCountdown[client] = INVALID_HANDLE;
	}
}

public Action:task_Clean(Handle:Timer, any:client)
{
	KillTimer(Timer);
	g_taskClean[client] = INVALID_HANDLE;
	int iFlags = GetCommandFlags("r_screenoverlay") & (~FCVAR_CHEAT);
	SetCommandFlags("r_screenoverlay", iFlags);
	ClientCommand(client, "r_screenoverlay \"\"");
}

public ShowKillMessage(client,type)
{
	char overlays_file[64];
	char pic1[64];
	char pic2[64];
	char pic3[64];
	GetConVarString(hit1, pic1, sizeof(pic1));
	GetConVarString(hit2, pic2, sizeof(pic2));
	GetConVarString(hit3, pic3, sizeof(pic3));
	Format(overlays_file,sizeof(overlays_file),"%s.vtf",pic1);
	PrecacheDecal(overlays_file,true);
	Format(overlays_file,sizeof(overlays_file),"%s.vtf",pic2);
	PrecacheDecal(overlays_file,true);
	Format(overlays_file,sizeof(overlays_file),"%s.vtf",pic3);
	PrecacheDecal(overlays_file,true);
	Format(overlays_file,sizeof(overlays_file),"%s.vmt",pic1);
	PrecacheDecal(overlays_file,true);
	Format(overlays_file,sizeof(overlays_file),"%s.vmt",pic2);
	PrecacheDecal(overlays_file,true);
	Format(overlays_file,sizeof(overlays_file),"%s.vmt",pic3);
	PrecacheDecal(overlays_file,true);
	
	int iFlags = GetCommandFlags("r_screenoverlay") & (~FCVAR_CHEAT);
	SetCommandFlags("r_screenoverlay", iFlags);
	switch(type)
	{
		case (kill_1):ClientCommand(client, "r_screenoverlay \"%s\"",pic1);
		case (kill):ClientCommand(client, "r_screenoverlay \"%s\"",pic3);
		case (hit_armor):ClientCommand(client, "r_screenoverlay \"%s\"",pic2);	
	}
	if(g_bShowAuthor[client])
	{
		g_bShowAuthor[client] = false;
		SendTopLeftText(client,225,225,64,192,1,2," ");
	}
}

public OnClientDisconnect_Post(client)
{
	if(g_taskCountdown[client] != INVALID_HANDLE)
	{
		KillTimer(g_taskCountdown[client]);
		g_taskCountdown[client] = INVALID_HANDLE;
	}
	
	if(g_taskClean[client] != INVALID_HANDLE)
	{
		KillTimer(g_taskClean[client]);
		g_taskClean[client] = INVALID_HANDLE;
	}
}

public SendTopLeftText(client,r, g, b, a, level, time, const String:message[])
{
	new Handle:kv = CreateKeyValues("Stuff", "title", message);
	if(kv == INVALID_HANDLE)
		return;
	
	KvSetColor(kv, "color", r, g, b, a);
	KvSetNum(kv, "level", level);
	KvSetNum(kv, "time", time);
	CreateDialog(client, kv, DialogType_Msg);		
	CloseHandle(kv);
	
}
