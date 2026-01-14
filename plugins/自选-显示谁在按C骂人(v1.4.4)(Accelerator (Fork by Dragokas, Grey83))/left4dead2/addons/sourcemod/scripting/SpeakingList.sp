#pragma semicolon 1
#pragma newdecls required
#include <basecomm>
#include <sdktools_voice>

public Plugin myinfo = {
	name = "[ANY] Speaking List",
	author = "Accelerator (Fork by Dragokas, Grey83)",
	description = "Voice Announce. Print To Center Message who's Speaking",
	version = "1.4.4",
	url = "https://forums.alliedmods.net/showthread.php?t=339934"
}

/*
	ChangeLog:
	
	 * 1.4.1 (26-Jan-2020) (Dragokas)
	  - Client in game check fixed
	  - Code is simplified
	  - New syntax
	  
	 * 1.4.2 (23-Dec-2020) (Dragokas)
	  - Updated to use with SM 1.11
	  - Timer is increased 0.7 => 1.0
	  
	 * 1.4.4 (10-Oct-2022) (Grey83)
	  - Optimization: timer moved from OnPluginStart to OnMapStart.
	  - Optimization: max. buffer checks and caching.
*/

bool
	g_bSpeaking[MAXPLAYERS + 1];

char
	g_sSpeaking[PLATFORM_MAX_PATH];

public void OnMapStart() {
    for (int i = 1; i <= MaxClients; i++)
		g_bSpeaking[i] = false;

    CreateTimer(1.0, tmrUpdateList, _, TIMER_REPEAT|TIMER_FLAG_NO_MAPCHANGE);
}

public void OnClientSpeaking(int client) {
	g_bSpeaking[client] = true;
}

/*
public void OnClientSpeakingEnd(int client) {
	g_bSpeaking[client] = false;
}
*/

Action tmrUpdateList(Handle timer) {
	static int i;
	static bool show;
	g_sSpeaking[0] = '\0';
	show = false;

	for (i = 1; i <= MaxClients; i++) {
		if (!g_bSpeaking[i])
			continue;

		g_bSpeaking[i] = false;
		if (!IsClientInGame(i))
			continue;
		QueryClientConVar(i, "voice_vox", OnQueryFinished);
		
		if (GetClientListeningFlags(i) == VOICE_MUTED)
			continue;
			
		if (BaseComm_IsClientMuted(i))
			continue;

		if (Format(g_sSpeaking, sizeof g_sSpeaking, "%s\n%N", g_sSpeaking, i) >= (sizeof g_sSpeaking - 1))
			break;

		show = true;
	}

	if (show)
		PrintCenterTextAll("语音中:%s", g_sSpeaking);

	return Plugin_Continue;
}
void OnQueryFinished(QueryCookie cookie, int client, ConVarQueryResult result, const char[] cvarName, const char[] cvarValue)
{
	if (result == ConVarQuery_Okay)
	{
		if (StringToInt(cvarValue) != 0)
		{
			if (GetClientListeningFlags(client) != VOICE_MUTED)
			{
				SetClientListeningFlags(client, VOICE_MUTED);
				PrintToChat(client, "\x04[提示]\x05服务器自动静音使用\x04开放式麦克风\x05的玩家,更改为\x03按键通话\x05自动解除静音.");
			}
		}
		else 
		if (GetClientListeningFlags(client) != VOICE_NORMAL)
		{
			SetClientListeningFlags(client, VOICE_NORMAL);
			PrintToChat(client, "\x04[提示]\x05检测你是\x04按键通话\x05已解除静音.");
		}
	}
}