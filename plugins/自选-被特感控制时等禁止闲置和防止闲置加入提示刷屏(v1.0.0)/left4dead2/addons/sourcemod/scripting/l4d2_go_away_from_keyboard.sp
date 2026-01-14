#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#include <sdkhooks>
#include <sdktools>

#define PLUGIN_VERSION	"1.0.0"

#define TEAM_SPECTATOR	1
#define TEAM_SURVIVOR	2
#define TEAM_INFECTED   3
#define TEAM_PASSING	4

char QsPath[PLATFORM_MAX_PATH];

Handle QmFileK = null;
Handle g_hSDK_Call_GoAwayFromKeyboard;

ConVar g_hAllBotGame, l4d2_awayfromserver, l4d2_timerkeyboard, l4d2_timerSpecNext;
int g_awayfromserver, g_iAllBotGame;
float g_timerkeyboard, g_timerSpecNext;

float g_awaySpecNextTime[MAXPLAYERS+1];
float g_awayfromserverTime[MAXPLAYERS+1];

#define NAME_GoAwayFromKeyboard "CTerrorPlayer::GoAwayFromKeyboard"
#define SIG_GoAwayFromKeyboard_LINUX "@_ZN13CTerrorPlayer18GoAwayFromKeyboardEv"
#define SIG_GoAwayFromKeyboard_WINDOWS "\\x2A\\x2A\\x2A\\x2A\\x2A\\x2A\\x53\\x56\\x57\\x8B\\xF1\\x8B\\x06\\x8B\\x90\\xC8\\x08\\x00\\x00"

ArrayList LastOwner;

public Plugin myinfo = 
{
	name 			= "l4d2_go_away_from_keyboard",
	author 			= "豆瓣酱な(全部功能都是白嫖的)",
	description 	= "多种状态禁止使用闲置.",
	version 		= PLUGIN_VERSION,
	url 			= "N/A"
}

public void OnPluginStart()
{
	vFromLoadGameData();
	g_hAllBotGame		= FindConVar("sb_all_bot_game");
	l4d2_awayfromserver	= CreateConVar("l4d2_enabled_away_from_server", "1", "启用被特感控制,发射榴弹,投掷燃烧瓶时禁止使用休息? 0=禁用, 1=启用.", FCVAR_NOTIFY);
	l4d2_timerkeyboard	= CreateConVar("l4d2_enabled_away_timer_keyboard", "8.0", "设置多少秒后才能再次使用休息.", FCVAR_NOTIFY);
	l4d2_timerSpecNext	= CreateConVar("l4d2_enabled_away_timer_specnext", "3.0", "设置多少秒后才能再加入幸存者.", FCVAR_NOTIFY);
	
	g_hAllBotGame.AddChangeHook(l4d2keyboardConVarChanged);
	l4d2_awayfromserver.AddChangeHook(l4d2keyboardConVarChanged);
	l4d2_timerkeyboard.AddChangeHook(l4d2keyboardConVarChanged);
	l4d2_timerSpecNext.AddChangeHook(l4d2keyboardConVarChanged);
	
	AutoExecConfig(true, "l4d2_go_away_from_keyboard");//生成指定文件名的CFG.
	
	//监听休息命令.
	AddCommandListener(away_from_keyboard, "go_away_from_keyboard");
	//监听鼠标左键.
	AddCommandListener(CommandListener_SpecNext, "spec_next");
	
	LastOwner = new ArrayList(2);
}

public void OnMapStart()
{
	l4d2GetkeyboardCvars();
}

public void l4d2keyboardConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	l4d2GetkeyboardCvars();
}

void l4d2GetkeyboardCvars()
{
	g_iAllBotGame = g_hAllBotGame.IntValue;
	g_awayfromserver = l4d2_awayfromserver.IntValue;
	g_timerkeyboard = l4d2_timerkeyboard.FloatValue;
	g_timerSpecNext = l4d2_timerSpecNext.FloatValue;
}

public void OnConfigsExecuted()
{
	vFromLoadGameData();
}

void vFromLoadGameData()
{
	QmFileK = CreateKeyValues("Games");
	BuildPath(Path_SM, QsPath, sizeof(QsPath), "gamedata/l4d2_go_away_from_keyboard.txt");
	if(FileExists(QsPath)) 
	{
		//读取数据
		FileToKeyValues(QmFileK, QsPath);
		GameData hGameData = new GameData("l4d2_go_away_from_keyboard");

		StartPrepSDKCall(SDKCall_Player);
		if(PrepSDKCall_SetFromConf(hGameData, SDKConf_Signature, "CTerrorPlayer::GoAwayFromKeyboard") == false)
			SetFailState("Failed to find signature: CTerrorPlayer::GoAwayFromKeyboard");
		PrepSDKCall_SetReturnInfo(SDKType_Bool, SDKPass_Plain);
		g_hSDK_Call_GoAwayFromKeyboard = EndPrepSDKCall();
		if(g_hSDK_Call_GoAwayFromKeyboard == null)
			SetFailState("Failed to create SDKCall: CTerrorPlayer::GoAwayFromKeyboard");

		delete hGameData;
	}
	else
	{
		//在控制台输出。游戏中看不到
		PrintToServer("[提示] 未发现%s文件,创建中...", QsPath);
		vKeyboardLoadGameData();
	}
}

/// 签名与偏移文件生成
public void vKeyboardLoadGameData()
{
	QmFileK = OpenFile(QsPath, "w");
	if (QmFileK == null)
	{
		SetFailState("[提示] 创建 %s 文件失败.", QsPath);
	}
	
	WriteFileLine(QmFileK, "\"Games\"");
	WriteFileLine(QmFileK, "{");

	WriteFileLine(QmFileK, "	\"left4dead2\"");
	WriteFileLine(QmFileK, "	{");
	WriteFileLine(QmFileK, "		\"Signatures\"");
	WriteFileLine(QmFileK, "		{");
	
	WriteFileLine(QmFileK, "			\"%s\"", NAME_GoAwayFromKeyboard);
	WriteFileLine(QmFileK, "			{");
	WriteFileLine(QmFileK, "				\"library\"	\"server\"");
	WriteFileLine(QmFileK, "				\"linux\"	\"%s\"", SIG_GoAwayFromKeyboard_LINUX);
	WriteFileLine(QmFileK, "				\"windows\"	\"%s\"", SIG_GoAwayFromKeyboard_WINDOWS);
	WriteFileLine(QmFileK, "			}");
	
	WriteFileLine(QmFileK, "		}");
	WriteFileLine(QmFileK, "	}");
	WriteFileLine(QmFileK, "}");
	
	delete QmFileK;
}

public void OnMapEnd()
{
	LastOwner.Clear();
}

public void OnEntityCreated(int entity, const char[] classname)
{
	if(classname[0] != 'm' && classname[0] != 'g')
		return;

	if(strncmp(classname, "molotov_projectile", 18) == 0 || strncmp(classname, "grenade_launcher_projectile", 27) == 0)
		SDKHook(entity, SDKHook_SpawnPost, SpawnPost_Grenade);
}

public void SpawnPost_Grenade(int entity)
{
	SDKUnhook(entity, SDKHook_SpawnPost, SpawnPost_Grenade);
	if(entity <= MaxClients || !IsValidEntity(entity))
		return;

	int client = GetEntPropEnt(entity, Prop_Send, "m_hOwnerEntity");
	if(IsValidClient(client) && GetClientTeam(client) == 2 && !IsFakeClient(client))
	{
		int index = LastOwner.Push(GetClientUserId(client));
		LastOwner.Set(index, EntIndexToEntRef(entity), 1);
	}
}

public void OnEntityDestroyed(int entity)
{
	if(entity <= MaxClients || !IsValidEntity(entity))
		return;
	
	static char sItem[32];
	GetEdictClassname(entity, sItem, sizeof(sItem));
	if(sItem[0] != 'm' && sItem[0] != 'g')
        return;
		
	if(strncmp(sItem, "molotov_projectile", 18) == 0 || strncmp(sItem, "grenade_launcher_projectile", 27) == 0)
	{
		int index = LastOwner.FindValue(EntIndexToEntRef(entity), 1);
		if(index != -1)
			LastOwner.Erase(index);
	}
}

public Action away_from_keyboard(int client, const char[] command, int args)
{
	if (g_awayfromserver == 0)
		return Plugin_Continue;
	
	if (g_awayfromserver == 1)
	{
		if(IsValidClient(client))
		{
			if(IsPlayerAlive(client) && GetClientTeam(client) == 2)
			{
				if (g_iAllBotGame == 0 && GetSurvivorsNumber() <= 1)
				{
					PrintToChat(client, "\x04[提示]\x05必须要两名玩家才能使用休息指令.");
					return Plugin_Handled;
				}
				if (go_away_from(client))
				{
					PrintCenterText(client, "被特感控制时禁止使用休息.");
					return Plugin_Handled;
				}
				
				float fromTime = GetEngineTime() - g_awayfromserverTime[client];
				
				if(LastOwner.FindValue(GetClientUserId(client), 0) != -1)
				{
					PrintCenterText(client, "发射榴弹或投掷燃烧瓶时禁止使用休息.");
					return Plugin_Handled;
				}
				else if (fromTime < g_timerkeyboard)
				{
					PrintCenterText(client, "请等待 %.1f 秒后再使用休息.", g_timerkeyboard - fromTime);
					return Plugin_Handled;
				}
				else if (IsInReload(client))
				{
					PrintCenterText(client, "更换弹药时禁止使用休息.");
					return Plugin_Handled;
				}
				else if (bIsGettingUp(client))
				{
					PrintToChat(client, "\x04[提示]\x05起身过程中禁止使用休息.");
					return Plugin_Handled;
				}
				else if (L4D_IsPlayerStaggering(client))
				{
					PrintToChat(client, "\x04[提示]\x05硬直状态时禁止使用休息.");
					return Plugin_Handled;
				}
				else 
				{
					g_awayfromserverTime[client] = g_awaySpecNextTime[client] = GetEngineTime();
					SDKCall(g_hSDK_Call_GoAwayFromKeyboard, client);
					return Plugin_Handled;//阻止游戏自带的闲置命令,使用签名闲置.
				}
			}
			else if(GetClientTeam(client) == 1)
			{
				PrintToChat(client,"\x04[提示]\x05你当前已经加入了观察者.");
				return Plugin_Handled;
			}
			else if(!IsPlayerAlive(client))
			{
				PrintToChat(client,"\x04[提示]\x05死亡状态禁止使用休息.");
				return Plugin_Handled;
			}
			else if(GetClientTeam(client) == 3)
			{
				PrintToChat(client,"\x04[提示]\x05休息指令只限幸存者使用.");
				return Plugin_Handled;
			}
		}
	}
	return Plugin_Continue;
}

//https://github.com/LuxLuma/L4D2_Adrenaline_Recovery
bool bIsGettingUp(int client) {
	static char sModel[31];
	GetClientModel(client, sModel, sizeof sModel);
	switch (sModel[29]) {
		case 'b': {	//nick
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 680, 667, 671, 672, 630, 620, 627:
					return true;
			}
		}

		case 'd': {	//rochelle
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 687, 679, 678, 674, 638, 635, 629:
					return true;
			}
		}

		case 'c': {	//coach
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 669, 661, 660, 656, 630, 627, 621:
					return true;
			}
		}

		case 'h': {	//ellis
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 684, 676, 675, 671, 625, 635, 632:
					return true;
			}
		}

		case 'v': {	//bill
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 772, 764, 763, 759, 538, 535, 528:
					return true;
			}
		}

		case 'n': {	//zoey
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 824, 823, 819, 809, 547, 544, 537:
					return true;
			}
		}

		case 'e': {	//francis
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 775, 767, 766, 762, 541, 539, 531:
					return true;
			}
		}

		case 'a': {	//louis
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 772, 764, 763, 759, 538, 535, 528:
					return true;
			}
		}

		case 'w': {	//adawong
			switch (GetEntProp(client, Prop_Send, "m_nSequence")) {
				case 687, 679, 678, 674, 638, 635, 629:
					return true;
			}
		}
	}

	return false;
}

bool IsInReload(int client)
{
	int iActiveWeapon = GetEntPropEnt(client, Prop_Data, "m_hActiveWeapon");
	
	return IsValidEdict(iActiveWeapon) && GetEntProp(iActiveWeapon, Prop_Data, "m_bInReload") == 1;
}

//闲置后短时间内禁止加入幸存者(只限闲置状态,旁观者不受此限制).
public Action CommandListener_SpecNext(int client, char[] command, int argc)
{
	if(IsValidClient(client) && !IsFakeClient(client))
	{
		if(GetClientTeam(client) == 1 && iGetBotOfIdle(client))
		{
			float SpecTime = GetEngineTime() - g_awaySpecNextTime[client];
			if (SpecTime < g_timerSpecNext)
			{
				PrintCenterText(client, "请等待 %.1f 秒后再加入幸存者.", g_timerSpecNext - SpecTime);
				return Plugin_Handled;
			}
		}
	}
	return Plugin_Continue;
}

bool IsValidClient(int client)
{
	return (client > 0 && client <= MaxClients && IsClientInGame(client));
}

int GetSurvivorsNumber()
{
	int count1 = 0;
	for (int i = 1; i <= MaxClients; i++)
		if (IsClientInGame(i) && !IsFakeClient(i) && GetClientTeam(i) == 2)
			count1++;
	
	return count1;
}

int iGetBotOfIdle(int client)
{
	for(int i = 1; i <= MaxClients; i++)
	{
		if(IsClientInGame(i) && IsFakeClient(i) && GetClientTeam(i) == TEAM_SURVIVOR && (iHasIdlePlayer(i) == client))
			return i;
	}
	return 0;
}

static int iHasIdlePlayer(int client)
{
	char sNetClass[64];
	if(!GetEntityNetClass(client, sNetClass, sizeof(sNetClass)))
		return 0;

	if(FindSendPropInfo(sNetClass, "m_humanSpectatorUserID") < 1)
		return 0;

	client = GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));			
	if(client && IsClientInGame(client) && !IsFakeClient(client) && GetClientTeam(client) == TEAM_SPECTATOR)
		return client;

	return 0;
}

bool go_away_from(int client)
{
	if(GetEntPropEnt(client, Prop_Send, "m_pummelAttacker") > 0)
		return true;
	if(GetEntPropEnt(client, Prop_Send, "m_carryAttacker") > 0)
		return true;
	if(GetEntPropEnt(client, Prop_Send, "m_pounceAttacker") > 0)
		return true;
	if(GetEntPropEnt(client, Prop_Send, "m_jockeyAttacker") > 0)
		return true;
	if(GetEntPropEnt(client, Prop_Send, "m_tongueOwner") > 0)
		return true;
	return false;
}

/**
 * @brief Checks if a Survivor is currently staggering
 *
 * @param client			Client ID of the player to affect
 *
 * @return Returns true if player is staggering, false otherwise
 */
stock bool L4D_IsPlayerStaggering(int client)
{
	static int m_iQueuedStaggerType = -1;
	if( m_iQueuedStaggerType == -1 )
	m_iQueuedStaggerType = FindSendPropInfo("CTerrorPlayer", "m_staggerDist") + 4;

	if( GetEntData(client, m_iQueuedStaggerType, 4) == -1 )
	{
		if( GetGameTime() >= GetEntPropFloat(client, Prop_Send, "m_staggerTimer", 1) )
		{
			return false;
		}

		static float vStgDist[3], vOrigin[3];
		GetEntPropVector(client, Prop_Send, "m_staggerStart", vStgDist);
		GetEntPropVector(client, Prop_Send, "m_vecOrigin", vOrigin);

		static float fStgDist2;
		fStgDist2 = GetEntPropFloat(client, Prop_Send, "m_staggerDist");

		return GetVectorDistance(vStgDist, vOrigin) <= fStgDist2;
	}

	return true;
}