#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>

public Plugin myinfo = 
{
	name = "Car Alarm Inform",
	author = "Eyal282 ( FuckTheSchool )",
	description = "Tells the players whomst'dve the fucc started the mofo car alarm",
	version = "2.0",
	url = "<- URL ->"
}

bool g_bAlarmWentOff;

public void OnPluginStart()
{
	HookEvent("create_panic_event", Event_CreatePanicEvent, EventHookMode_Post);
	HookEvent("triggered_car_alarm", Event_TriggeredCarAlarm, EventHookMode_Pre);
}

public void Event_TriggeredCarAlarm(Event event, const char[] name, bool dontBroadcast)
{
	g_bAlarmWentOff = true;
}

public void Event_CreatePanicEvent(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	
	if(IsValidClient(client) && GetClientTeam(client) == 2)
		RequestFrame(IsCheckAlarm, GetClientUserId(client));
}

void IsCheckAlarm(int client)
{
	if(!(client = GetClientOfUserId(client)) || !g_bAlarmWentOff)
		return;

	g_bAlarmWentOff = false;
	
	PrintToChatAll("\x04[提示]\x03%s\x05触发了汽车警报.", GetTrueName(client));//聊天窗提示.
}

bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}

char[] GetTrueName(int client)
{
	char g_sName[32];
	int iBot = IsClientIdle(client);
	
	if(iBot != 0)
		FormatEx(g_sName, sizeof(g_sName), "闲置:%N", iBot);
	else
		GetClientName(client, g_sName, sizeof(g_sName));
	return g_sName;
}

int IsClientIdle(int client)
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}