#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#include <sdktools>

ConVar g_hBlockCmds, g_hPunishType, g_hBanTime;
char g_sBlockCmds[64][32];
int g_iBlockCmdsCount;

public Plugin myinfo = 
{
	name = "block cmds",
	author = "sorallll",
	description = "",
	version = "1.0",
	url = ""
}

public void OnPluginStart()
{
	g_hBlockCmds	= CreateConVar("blockcmds_list", "sm_pw;sm_rpg;sm_boom;sm_explode;sm_vip;sm_help", "使用';'号分隔要禁用的命令.");
	g_hPunishType	= CreateConVar("l4d2_blockcmds_punish_Type", "0", "玩家输入了限制的指令后的惩罚方式. 0=仅提示, 1=处死, 2=踢出, 3=封禁.");
	g_hBanTime		= CreateConVar("l4d2_blockcmds_punish_time", "5", "设置被封禁的时间/分钟. 0=永久封禁.");

	AutoExecConfig(true,"l4d2_block_cmds");

	g_hBlockCmds.AddChangeHook(ConVarChanged);
}

public void OnConfigsExecuted()
{
	GetCmds();
}

public void ConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	GetCmds();
}

void GetCmds()
{
	for(int i = 0; i < sizeof(g_sBlockCmds); i++)
	{
		if(g_sBlockCmds[i][0] != 0)
		{
			RemoveCommandListener(CmdIntercept, g_sBlockCmds[i]);
			g_sBlockCmds[i][0] = 0;
		}
	}

	char sCmds[2048];
	g_hBlockCmds.GetString(sCmds, sizeof(sCmds));
	g_iBlockCmdsCount = ReplaceString(sCmds, sizeof(sCmds), ";", ";", false);
	ExplodeString(sCmds, ";", g_sBlockCmds, g_iBlockCmdsCount + 1, sizeof(g_sBlockCmds));

	for(int i = 0; i <= g_iBlockCmdsCount; i++)
		AddCommandListener(CmdIntercept, g_sBlockCmds[i]);
}

public Action CmdIntercept(int client, const char[] Command, int args)
{
	if(IsValidClient(client))
		PunishType(client);
	return Plugin_Stop;
}

public Action OnClientSayCommand(int client, const char[] commnad, const char[] args)
{
	if(strlen(args) <= 1 || strncmp(commnad, "say", 3, false) != 0)
		return Plugin_Continue;

	if((args[0] == '!' || args[0] == '/') && IsAllowChatBlock(args))
	{
		if(IsValidClient(client))
			PunishType(client, args);
		return Plugin_Handled;
	}
	return Plugin_Continue;
}

void PunishType(int client, char[] Command)
{
	switch(g_hPunishType.IntValue)
	{
		case 0:
			PrintToChat(client, "\x04[提示]\x05请勿使用\x03%s\x05指令.", Command);
		case 1:
			PrintToChat(GetCheckIdleBot(client), "\x04[提示]\x05由于你使用了\x03%s\x05指令,已被处死.", Command);
		case 2:
			KickClient(client, "[提示]由于你使用了奇怪的指令,已被踢出服务器.");
		case 3:
			BanClient(client, g_hBanTime.IntValue, BANFLAG_AUTO, "Banned", "[提示]由于你使用了奇怪的指令,已被封禁.");
	}
}

bool IsAllowChatBlock(const char[] Command)
{
	for(int i = 0; i <= g_iBlockCmdsCount; i++)
	{
		if((strncmp(g_sBlockCmds[i], "sm_", 3, false) == 0 && strncmp(g_sBlockCmds[i][3], Command[1], strlen(g_sBlockCmds[i][3]), false) == 0) || strncmp(g_sBlockCmds[i], Command[1], strlen(g_sBlockCmds[i]), false) == 0)
			return true;
	}
	return false;
}

int GetCheckIdleBot(int client)
{
	int bot = iGetBotOfIdlePlayer(client);
	if(bot != 0)
		ForcePlayerSuicide(bot);
	else
		ForcePlayerSuicide(client);
	return bot != 0 ? bot : client;
}

//返回闲置玩家对应的电脑.
int iGetBotOfIdlePlayer(int client)
{
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientInGame(i) && IsFakeClient(i) && GetClientTeam(i) == 2 && IsClientIdle(i) == client)
			return i;
	}
	return 0;
}

//返回电脑幸存者对应的玩家.
int IsClientIdle(int client)
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}

//判断玩家有效.
bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}