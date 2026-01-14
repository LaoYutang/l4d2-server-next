#pragma semicolon 1
#pragma newdecls required
#include <sourcemod>

#define PLUGIN_VERSION "1.2.4"

Menu g_hVoteMenu;
Handle g_hChangelevel;

char g_sName[4][32] = 
{
	"简单", 
	"普通", 
	"高级", 
	"专家"
};

char g_sCode[4][32] = 
{
	"Easy", 
	"Normal", 
	"Hard", 
	"Impossible"
};

int    g_iBanTime, g_iPercent, g_iMenuTime;
ConVar g_hBanTime, g_hPercent, g_hMenuTime;

public Plugin myinfo =  
{
	name = "l4d2_menu_vote", 
	author = "豆瓣酱な", 
	description = "投票更改难度,踢出玩家,重启当前章节.", 
	version = PLUGIN_VERSION,
	url = "N/A"
};

public void OnPluginStart()   
{
	RegConsoleCmd("sm_vt", Command_VoteMenu);

	g_hBanTime = CreateConVar("l4d2_vote_BanTime", "5", "设置被投票踢出玩家的封禁时间/分钟. -1=仅踢出, 0=永久封禁.", FCVAR_NOTIFY);
	g_hPercent = CreateConVar("l4d2_vote_Percent", "60", "设置通过投票所需的百分比(1/100).", FCVAR_NOTIFY);
	g_hMenuTime = CreateConVar("l4d2_vote_menuTime", "15", "设置同意或反对菜单的显示时间/秒.", FCVAR_NOTIFY);
	g_hBanTime.AddChangeHook(CvarChanged);
	g_hPercent.AddChangeHook(CvarChanged);
	g_hMenuTime.AddChangeHook(CvarChanged);

	AutoExecConfig(true, "l4d2_menu_vote");
}

public void CvarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	GetCvars();
}

void GetCvars()
{
	g_iBanTime = g_hBanTime.IntValue;
	g_iPercent = g_hPercent.IntValue;
	g_iMenuTime = g_hMenuTime.IntValue;

	if (g_iPercent < 1)
		g_iPercent = 1;
	if (g_iPercent > 100)
		g_iPercent = 100;
	if (g_iMenuTime < 1)
		g_iMenuTime = 1;
}

//开局提示.
public void OnClientPutInServer(int client)
{
	if(!IsFakeClient(client))
		CreateTimer(12.5, g_hTimerAnnounce, GetClientUserId(client), TIMER_FLAG_NO_MAPCHANGE);
}

public Action g_hTimerAnnounce(Handle timer, any client)
{
	if ((client = GetClientOfUserId(client)) && IsClientInGame(client))
		PrintToChat(client, "\x04[提示]\x05聊天窗输入指令\x03!vt\x05打开投票菜单.");//聊天窗提示.
	return Plugin_Continue;
}
//地图开始.
public void OnMapStart()
{
	GetCvars();
	delete g_hChangelevel;
}
//地图结束.
public void OnMapEnd()
{
	delete g_hChangelevel;
}

public Action Command_VoteMenu(int client, int args)
{
	IsChooseFunction(client);
	return Plugin_Handled;
}

void IsChooseFunction(int client)
{
	char line[32];
	Menu menu = new Menu(Menu_HandlerFunction);
	FormatEx(line, sizeof(line), "投票菜单:");
	SetMenuTitle(menu, "%s", line);
	menu.AddItem("0", "更改难度");
	menu.AddItem("1", "重启章节");
	menu.AddItem("2", "踢出玩家");
	menu.ExitButton = true;//默认值:true,设置为:false,则不显示退出选项.
	//menu.ExitBackButton = true;
	menu.Display(client, MENU_TIME_FOREVER);
}

int Menu_HandlerFunction(Menu menu, MenuAction action, int client, int itemNum)
{
	switch(action)
	{
		case MenuAction_End:
			delete menu;
		case MenuAction_Select:
		{
			char sItem[32];
			if(menu.GetItem(itemNum, sItem, sizeof(sItem)))
			{
				switch(StringToInt(sItem))
				{
					case 0:
						MenuVoteChangeDifficulty(client, sItem);
					case 1:
						MenuVoteReopenTheChapter(client, sItem);
					case 2:
						MenuVoteKickThePlayer(client, sItem);
				}
			}
		}
	}
	return 0;
}

void MenuVoteChangeDifficulty(int client, char[] g_sItem)
{
	if (IsVoteInProgress())
	{
		IsChooseFunction(client);
		ReplyToCommand(client, "\x04[提示]\x05当前正在进行投票.");
		return;
	}
	if (TestVoteDelay(client))
	{
		IsChooseFunction(client);
		return;
	}
	char line[32], sInfo[32], sData[3][32];
	Menu menu = new Menu(Menu_HandlerDifficulty);
	FormatEx(line, sizeof(line), "选择难度:");
	SetMenuTitle(menu, "%s", line);
	for (int i = 0; i < 4; i++)
	{
		if (strcmp(GetDifficulty(), g_sCode[i]) == 0)//不显示当前难度.
			continue;

		strcopy(sData[0], sizeof(sData[]), g_sItem);
		strcopy(sData[1], sizeof(sData[]), g_sCode[i]);
		strcopy(sData[2], sizeof(sData[]), g_sName[i]);
		ImplodeStrings(sData, sizeof(sData), "|", sInfo, sizeof(sInfo));//打包字符串.
		menu.AddItem(sInfo, g_sName[i]);
	}
	menu.ExitButton = true;//默认值:true,设置为:false,则不显示退出选项.
	menu.ExitBackButton = true;
	menu.Display(client, MENU_TIME_FOREVER);
}

int Menu_HandlerDifficulty(Menu menu, MenuAction action, int client, int itemNum)
{
	switch(action)
	{
		case MenuAction_End:
			delete menu;
		case MenuAction_Select:
		{
			char sItem[32], sName[32], sInfo[3][32];
			menu.GetItem(itemNum, sItem, sizeof(sItem), _, sName, sizeof(sName));
			ExplodeString(sItem, "|", sInfo, sizeof(sInfo), sizeof(sInfo[]));//拆分字符串.
			
			if (strcmp(GetDifficulty(), sInfo[2]) == 0)
				PrintToChat(client, "\x04[提示]\x05选择的难度与当前难度相同.");
			else
			{
				IsVoteChangeDifficulty(sItem, sName);
				PrintToChatAll("\x04[提示]\x03%N\x05发起投票更换难度为\x04:\x03%s", client, sName);
			}
		}
		case MenuAction_Cancel:
		{
			if (itemNum == MenuCancel_ExitBack)
				IsChooseFunction(client);
		}
	}
	return 0;
}

void IsVoteChangeDifficulty(char[] sItem, char[] sName)
{
	char line[32];
	g_hVoteMenu = new Menu(Menu_HandlerGetVotes, MENU_ACTIONS_ALL);
	FormatEx(line, sizeof(line), "更改难度为:%s?", sName);
	g_hVoteMenu.SetTitle("%s", line);
	g_hVoteMenu.AddItem(sItem, "同意");
	g_hVoteMenu.AddItem(sItem, "反对");
	g_hVoteMenu.ExitButton = false;//默认值:true,设置为:false,则不显示退出选项.
	//menu.ExitBackButton = true;
	g_hVoteMenu.DisplayVoteToAll(g_iMenuTime);
}

void MenuVoteReopenTheChapter(int client, char[] sItem)
{
	if (g_hChangelevel != null)
	{
		ReplyToCommand(client, "\x04[提示]\x05当前也存在重启章节的计时器.");
		return;
	}
	if (TestVoteDelay(client))
	{
		IsChooseFunction(client);
		return;
	}
	if (IsVoteInProgress())
	{
		IsChooseFunction(client);
		ReplyToCommand(client, "\x04[提示]\x05当前正在进行投票.");
		return;
	}
	char line[32], sInfo[32], sData[3][32];
	strcopy(sData[0], sizeof(sData[]), sItem);
	strcopy(sData[1], sizeof(sData[]), "内容");//这个里的内容用不着.
	strcopy(sData[2], sizeof(sData[]), "内容");//这个里的内容用不着.
	ImplodeStrings(sData, sizeof(sData), "|", sInfo, sizeof(sInfo));//打包字符串.
	g_hVoteMenu = new Menu(Menu_HandlerGetVotes, MENU_ACTIONS_ALL);
	FormatEx(line, sizeof(line), "重启当前章节?");
	g_hVoteMenu.SetTitle("%s", line);
	g_hVoteMenu.AddItem(sInfo, "同意");
	g_hVoteMenu.AddItem(sInfo, "反对");
	g_hVoteMenu.ExitButton = false;//默认值:true,设置为:false,则不显示退出选项.
	//menu.ExitBackButton = true;
	g_hVoteMenu.DisplayVoteToAll(g_iMenuTime);
	PrintToChatAll("\x04[提示]\x03%N\x05发起投票重启当前章节.", client);
}

void MenuVoteKickThePlayer(int client, char[] sItem)
{
	if (IsVoteInProgress())
	{
		IsChooseFunction(client);
		ReplyToCommand(client, "\x04[提示]\x05当前正在进行投票.");
		return;
	}
	if (TestVoteDelay(client))
	{
		IsChooseFunction(client);
		return;
	}
	if (!GetplayerTarget(client))
	{
		IsChooseFunction(client);
		PrintToChat(client, "\x04[提示]\x05当前有效玩家不足.");
		return;
	}
	char line[32], sInfo[32], sData[3][32];
	Menu menu = new Menu(Menu_HandlerKickPlayer);
	FormatEx(line, sizeof(line), "踢出玩家?");
	menu.SetTitle("%s", line);
	for (int i = 1; i <= MaxClients; i++) 
	{
		if (IsClientInGame(i) && !IsFakeClient(i) && client != i)
		{ 
			if (!CanUserTarget(client, i) && CanUserTarget(i, client))
				continue;

			strcopy (sData[0], sizeof(sData[]), sItem);
			FormatEx(sData[1], sizeof(sData[]), "%N", i);
			FormatEx(sData[2], sizeof(sData[]), "%d", GetClientUserId(i));
			ImplodeStrings(sData, sizeof(sData), "|", sInfo, sizeof(sInfo));//打包字符串.
			menu.AddItem(sInfo, sData[1]);
		}		
	}
	menu.ExitButton = true;//默认值:true,设置为:false,则不显示退出选项.
	menu.ExitBackButton = true;
	menu.Display(client, MENU_TIME_FOREVER);
}

int Menu_HandlerKickPlayer(Menu menu, MenuAction action, int client, int itemNum)
{
	switch(action)
	{
		case MenuAction_End:
			delete menu;
		case MenuAction_Select:
		{
			char sItem[32], sName[32];
			menu.GetItem(itemNum, sItem, sizeof(sItem), _, sName, sizeof(sName));
			IsVoteKickThePlayer(sItem, sName);
			PrintToChatAll("\x04[提示]\x03%N\x05发起投票踢出玩家\x04:\x03%s", client, sName);
		}
		case MenuAction_Cancel:
		{
			if (itemNum == MenuCancel_ExitBack)
				IsChooseFunction(client);
		}
	}
	return 0;
}

void IsVoteKickThePlayer(char[] sItem, char[] sName)
{
	char line[32];
	g_hVoteMenu = new Menu(Menu_HandlerGetVotes, MENU_ACTIONS_ALL);
	FormatEx(line, sizeof(line), "踢出玩家:%s?", sName);
	g_hVoteMenu.SetTitle("%s", line);
	g_hVoteMenu.AddItem(sItem, "同意");
	g_hVoteMenu.AddItem(sItem, "反对");
	g_hVoteMenu.ExitButton = false;//默认值:true,设置为:false,则不显示退出选项.
	//menu.ExitBackButton = true;
	g_hVoteMenu.DisplayVoteToAll(g_iMenuTime);
}

int Menu_HandlerGetVotes(Menu menu, MenuAction action, int param1, int param2)
{
	if(action == MenuAction_Select)
	{
		switch(param2)
		{
			case 0: 
				PrintToChatAll("\x04[提示]\x03%N\x05已投票.", param1);
			case 1: 
				PrintToChatAll("\x04[提示]\x03%N\x05已投票.", param1);
		}
	}
	int iPercent, iWinningVotes, iTotalVotes;
	GetMenuVoteInfo(param2, iWinningVotes, iTotalVotes);

	if (param1 == 1)
		iWinningVotes = iTotalVotes - iWinningVotes;

	char sItem[32], sInfo[3][32];
	menu.GetItem(param1, sItem, sizeof(sItem));
	ExplodeString(sItem, "|", sInfo, sizeof(sInfo), sizeof(sInfo[]));//拆分字符串.
	iPercent = GetVotePercent(iWinningVotes, iTotalVotes);
	
	if (action == MenuAction_End)
		delete g_hVoteMenu;
	else if (action == MenuAction_VoteCancel && param1 == VoteCancel_NoVotes)
		PrintToChatAll("\x04[提示]\x05本次没有玩家投票.");
	else if (action == MenuAction_VoteEnd)
	{
		if (FloatCompare(float(iPercent), float(g_iPercent)) < 0)
			PrintToChatAll("\x04[提示]\x05投票失败\x04.\x05至少需要\x03%d%%\x05支持\x04(\x05同意\x03%d%%\x05总共\x03%i\x05票\x04)\x05.", g_iPercent, iPercent, iTotalVotes);
		else
		{
			switch(StringToInt(sInfo[0]))
			{
				case 0:
				{
					ServerCommand("z_difficulty %s", sInfo[1]);
					PrintToChatAll("\x04[提示]\x05投票通过\x04.\x05难度已更换为\x04:\x03%s\x04(\x05同意\x03%d%%\x05总共\x03%i\x05票\x04)\x05.", sInfo[2], iPercent, iTotalVotes);
				}
				case 1:
				{
					delete g_hChangelevel;
					g_hChangelevel = CreateTimer(8.0, ChangelevelMap);
					PrintHintTextToAll("[提示] 投票通过,将在 8秒 后重启当前章节.");
					PrintToChatAll("\x04[提示]\x05投票通过\x04.\x038\x05秒后重启当前章节\x04(\x05同意\x03%d%%\x05总共\x03%i\x05票\x04)\x05.", iPercent, iTotalVotes);
				}
				case 2:
				{
					int client = GetClientOfUserId(StringToInt(sInfo[2]));

					if (g_iBanTime <= -1)
						KickClient(client, "你被投票踢出");
					else
					{
						char g_sBanTime[64];
						FormatEx(g_sBanTime, sizeof(g_sBanTime), "你被投票踢出,并且被封禁%d分钟", g_iBanTime);
						BanClient(client, g_iBanTime, BANFLAG_AUTO, "被投票踢出", g_sBanTime);
					}
					PrintToChatAll("\x04[提示]\x05投票通过\x04.\x05玩家\x03%s\x05已被踢出.\x04(\x05同意\x03%d%%\x05总共\x03%i\x05票\x04)\x05.", sInfo[1], iPercent, iTotalVotes);
				}
			}
		}
	}
	return 0;
}

public Action ChangelevelMap(Handle Timer)
{
	char sMap[32];
	GetCurrentMap(sMap, sizeof(sMap));
	ServerCommand("changelevel %s", sMap);
	g_hChangelevel = null;
	return Plugin_Stop;
}

int GetVotePercent(int iWinningVotes, int iTotalVotes)
{
	return RoundToNearest(float(iWinningVotes) / float(iTotalVotes) * 100.0);
}

char[] GetDifficulty()
{
	char sDifficulty[16];
	GetConVarString(FindConVar("z_difficulty"), sDifficulty, sizeof(sDifficulty));
	return sDifficulty;
}

bool GetplayerTarget(int client)
{
	for(int i = 1;i <= MaxClients; i++)
	{
		if(IsClientInGame(i) && !IsFakeClient(i) && client != i)
		{
			if (!CanUserTarget(client, i) && CanUserTarget(i, client))
				continue;
			return true;
		}
	}	
	return false;
}

bool TestVoteDelay(int client)
{
 	int delay = CheckVoteDelay();
 	
 	if (delay > 0)
	{
 		PrintToChat(client, "\x04[提示]\x05您必须再等待\x03%s\x05后才能发起新的投票.", StandardizeTime(delay));
 		return true;
 	}
	return false;
}

//https://forums.alliedmods.net/showthread.php?t=288686
char[] StandardizeTime(int remainder)
{
	char str[32], sD[32], sH[32], sM[32], sS[32];

	int D = RoundToFloor(float(remainder) / 86400.0);
	remainder = remainder - (D * 86400);
	int H = RoundToFloor(float(remainder) / 3600.0);
	remainder = remainder - (H * 3600);
	int M = RoundToFloor(float(remainder) / 60.0);
	remainder = remainder - (M * 60);
	int S = RoundToFloor(float(remainder));

	FormatEx(sD, sizeof(sD), "%d天", D);
	FormatEx(sH, sizeof(sH), "%d%s", H, !D && !M && !S ? "小时" : "时");
	FormatEx(sM, sizeof(sM), "%d%s", M, !D && !H && !S ? "分钟" : "分");
	FormatEx(sS, sizeof(sS), "%d秒", S);
	FormatEx(str, sizeof(str), "%s%s%s%s", !D ? "" : sD, !H ? "" : sH, !M ? "" : sM, !S ? "" : sS);
	return str;
}