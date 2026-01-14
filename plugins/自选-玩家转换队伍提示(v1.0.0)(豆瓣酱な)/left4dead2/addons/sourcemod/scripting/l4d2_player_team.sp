#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>

#define PLUGIN_VERSION	"1.0.0"

public Plugin myinfo =
{
	name = "l4d2_player_team", 
	author = "豆瓣酱な", 
	description = "玩家转换队伍提示.", 
	version = PLUGIN_VERSION, 
	url = "N/A"
};

public void OnPluginStart()
{
	HookEvent("player_team", Event_PlayerTeam);//玩家转换队伍.
}
 
public void Event_PlayerTeam(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(event.GetInt("userid"));
	//int oldteam = event.GetInt("oldteam");
	int iTeam = event.GetInt("team");
	
	if(IsValidClient(client) && !IsFakeClient(client))
	{
		switch(iTeam)
		{
			case 1:
				PrintToChatAll("\x04[提示]\x03%N\x05加入了观察者.", client);
			case 2:
				PrintToChatAll("\x04[提示]\x03%N\x05加入了幸存者.", client);
			case 3:
				PrintToChatAll("\x04[提示]\x03%N\x05加入了感染者.", client);
		}
	}
}

bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}