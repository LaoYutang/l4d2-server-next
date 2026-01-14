#pragma semicolon 1
#pragma newdecls required
#include <sourcemod>
#include <sdktools>

ConVar g_hChangeNameTries;

char g_sDefName[MAXPLAYERS + 1][MAX_NAME_LENGTH];
int g_iChangeNameTries[MAXPLAYERS + 1];

public Plugin myinfo =  
{
	name = "NameChange Blocker",
	author = "蛋疼哥,sorallll",
	description = "",
	version = "1.0",
	url = ""
}

public void OnPluginStart()
{
	g_hChangeNameTries = CreateConVar("change_name_tries", "3.0", "改名次数达到多少后自动踢出", _, true, 0.0);
	AutoExecConfig(true, "l4d2_NameChange_Blocker");//生成指定文件名的CFG.
	HookEvent("player_changename", Event_PlayerChangename);
	HookUserMessage(GetUserMessageId("SayText2"), SayText2, true);
}

public void OnClientPutInServer(int client)
{
	if(IsFakeClient(client))
		return;

	g_iChangeNameTries[client] = 0;
	FormatEx(g_sDefName[client], sizeof(g_sDefName[]), "%N", client);
}

public void Event_PlayerChangename(Event event, const char[] name, bool dontBroadcast)
{
	int userid = event.GetInt("userid");
	int client = GetClientOfUserId(userid);
	if(IsFakeClient(client))
		return;

	if(g_iChangeNameTries[client] >= g_hChangeNameTries.IntValue)
	{
		KickClient(client, "[提示] 本服禁止频繁更改游戏名字");
		g_iChangeNameTries[client] = 0;
		return;
	}

	char sNewname[MAX_NAME_LENGTH];
	event.GetString("newname", sNewname, sizeof(sNewname));
	if(strcmp(sNewname, g_sDefName[client]) != 0)
		RequestFrame(ResetNameFunc, GetClientUserId(client));

	g_iChangeNameTries[client]++;
}

void ResetNameFunc(any client)
{	
	if((client = GetClientOfUserId(client)) && IsClientInGame(client))
		SetClientInfo(client, "name", g_sDefName[client]);
}

public Action SayText2(UserMsg msg_id, BfRead msg, const int[] players, int playersNum, bool reliable, bool init)
{
	static char buffer[128];
	buffer[0] = 0;

	msg.ReadString(buffer, sizeof(buffer));
	msg.ReadString(buffer, sizeof(buffer));
	if(strcmp(buffer, "#Cstrike_Name_Change") == 0)
		return Plugin_Handled;

	return Plugin_Continue;
}