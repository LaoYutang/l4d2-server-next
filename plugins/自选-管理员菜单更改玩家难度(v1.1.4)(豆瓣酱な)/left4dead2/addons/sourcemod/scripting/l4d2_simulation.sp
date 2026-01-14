/*
 *
 *	v1.0.0
 *
 *	1:初始版本发布.
 *	2:女巫对生还者的伤害只修改了倒地后的持续攻击伤害.
 *	3:目前插件只适用于未修改特感对生还者伤害的服务器使用,并且只适用于战役模式.
 *
 *	v1.1.0
 *
 *	1:新增Native用于获取和设置玩家自定义难度.
 *
 *	v1.1.1
 *
 *	1:修复载图后客户端难度设置不正确的问题.
 *
 *	v1.1.2
 *
 *	1:修复使用命令打开菜单时会显示返回上一层的问题.
 *
 *	v1.1.3
 *
 *	1:修复是默认难度时数据库回调里数组报错的问题.
 *	2:新增是默认难度或数据库值不正确时删除对应的玩家数据.
 *
 *	v1.1.4
 *
 *	1:函数CacheSteamID()里某个变量多写了个[0].
 *
 */

#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#include <adminmenu>
#include <sdkhooks>

#define DEBUG	0	//0=禁用调试信息,1=显示调试信息.
#define PLUGIN_VERSION	"1.1.4"

Database g_dbSQL;

bool g_bLateLoad;

ConVar g_hDifficulty;

int g_iDamageMultiple[MAXPLAYERS + 1] = {-1,...};

char g_sAuthId[MAXPLAYERS + 1][128];
char g_sDifficultyName[][] = {"简单", "普通", "高级", "专家"};
char g_sDifficultyCode[][] = {"Easy", "Normal", "Hard", "Impossible"};

float g_fSmokerDamageMultiple[][] = 
{
	{1.0,2.0,5.0,20.0},
	{10.0,10.0,20.0,30.0}
};
float g_fBoomerDamageMultiple[][] = 
{
	{1.0,2.0,5.0,20.0},
	{1.0,2.0,5.0,20.0}
};
float g_fHunterDamageMultiple[][] = 
{
	{10.0,10.0,20.0,40.0},
	{5.0,5.0,10.0,15.0}
};
float g_fSpitterDamageMultiple[][] = 
{
	{1.0,2.0,5.0,20.0},
	{0.5,1.0,1.0,1.0}
};
float g_fJockeyDamageMultiple[][] = 
{
	{1.0,2.0,5.0,20.0},
	{4.0,4.0,8.0,12.0}
};
float g_fChargerDamageMultiple[][] = 
{
	{10.0,20.0,30.0,40.0},
	{10.0,10.0,15.0,20.0}
};
float g_fTankDamageMultiple[][] = 
{
	{24.0,24.0,33.0,100.0},
	{75.0,75.0,75.0,150.0}
};
float g_fWitchDamageMultiple[][] = 
{
	{15.0,30.0,60.0,300.0},
	{100.0,100.0,100.0,100.0}
};
float g_fInfectedDamageMultiple[][] = 
{
	{1.0,2.0,5.0,20.0},
	{10.0,10.0,10.0,10.0}
};

TopMenu g_hTopMenu;
TopMenuObject hOtherFeatures = INVALID_TOPMENUOBJECT;

public APLRes AskPluginLoad2(Handle myself, bool late, char[] error, int err_max)
{
	CreateNatives();
	g_bLateLoad = late;
	RegPluginLibrary("l4d2_simulation");
	return APLRes_Success;
}
//定义插件信息.
public Plugin myinfo = 
{
	name 			= "l4d2_simulation",
	author 			= "豆瓣酱な",
	description 	= "在当前游戏难度下模拟其它游戏难度,适用于未修改特感对生还者伤害的战役服务器使用.",
	version 		= PLUGIN_VERSION,
	url 			= "N/A"
}
public void OnPluginStart()
{
	if (!g_dbSQL)
		IniSQLite();

	if (g_bLateLoad)
		SQL_LoadAll();
	
	g_hDifficulty = FindConVar("z_Difficulty");
	
	RegConsoleCmd("sm_difficulty", Command_DifficultyMenu, "打开难度菜单.");
	HookEvent("player_disconnect", Event_PlayerDisconnect);//玩家离开.

	TopMenu topmenu;
	if (LibraryExists("adminmenu") && ((topmenu = GetAdminTopMenu()) != null))
		OnAdminMenuReady(topmenu);
}
void IniSQLite() 
{	
	char sError[1024];
	if (!(g_dbSQL = SQLite_UseDatabase("PlayerDifficulty", sError, sizeof sError)))
		SetFailState("Could not connect to the database \"PlayerDifficulty\" at the following error:\n%s", sError);

	SQL_FastQuery(g_dbSQL, "CREATE TABLE IF NOT EXISTS Difficulty(SteamID NVARCHAR(32) NOT NULL DEFAULT '', difficulty INT NOT NULL DEFAULT -1);");
}
//读取所有玩家数据.
stock void SQL_LoadAll() 
{
	for (int i = 1; i <= MaxClients; i++) 
	{
		if (IsClientInGame(i) && !IsFakeClient(i)) 
		{
			SQL_Load(i, false);//读取玩家数据.
		}
	}
}
//玩家加入游戏时.
public void OnClientPostAdminCheck(int client)
{
	if(!IsFakeClient(client))
	{
		g_iDamageMultiple[client] = -1;
		SQL_Load(client, false);//读取玩家数据.
	}
		
}
//读取玩家数据.
void SQL_Load(int client, bool write) 
{
	if (!g_dbSQL)
		return;

	if (!CacheSteamID(client))
		return;

	char query[256];
	FormatEx(query, sizeof query, "SELECT * FROM Difficulty WHERE SteamId = '%s';", g_sAuthId[client]);
	DataPack hPack = new DataPack();
	hPack.WriteCell(GetClientUserId(client));
	hPack.WriteCell(write);
	g_dbSQL.Query(SQL_CallbackLoad, query, hPack);
}
//数据库查询回调.
void SQL_CallbackLoad(Database db, DBResultSet results, const char[] error, DataPack hPack) 
{
	if (!db || !results) 
	{
		LogError(error);
		return;
	}
	int client;
	hPack.Reset();
	if (!(client = GetClientOfUserId(hPack.ReadCell())))
		return;

	bool write = view_as<bool>(hPack.ReadCell());

	if (results.FetchRow())
	{	
		if(write)
			SQL_Save(g_sAuthId[client], g_iDamageMultiple[client]);//如果已存在数据就更新.
		else
		{
			g_iDamageMultiple[client] = results.FetchInt(1);
			if (g_iDamageMultiple[client] < 0 || g_iDamageMultiple[client] > 3)
				SQL_Delete(g_sAuthId[client]);//删除指定的玩家数据.
			else
				g_hDifficulty.ReplicateToClient(client, g_sDifficultyCode[g_iDamageMultiple[client]]);
		}
	}
	else 
	{
		if(write)
		{
			if(g_iDamageMultiple[client] != -1)
			{
				char query[256];
				FormatEx(query, sizeof query, "INSERT INTO Difficulty(SteamID, difficulty) VALUES ('%s', %d);", g_sAuthId[client], g_iDamageMultiple[client]);
				SQL_FastQuery(g_dbSQL, query);
			}
		}
	}
	delete hPack;
}
//更新玩家数据.
stock void SQL_Save(char[] auth, int difficulty) 
{
	if (!g_dbSQL)
		return;

	char query[256];
	FormatEx(query, sizeof query, "UPDATE Difficulty SET difficulty = %d WHERE SteamID = '%s';", difficulty, auth);
	SQL_FastQuery(g_dbSQL, query);
}
//删除指定的玩家数据.
stock void SQL_Delete(const char[] auth) 
{
	char query[1024];
	FormatEx(query, sizeof query, "DELETE FROM Difficulty WHERE SteamID = '%s';", auth);
	g_dbSQL.Query(SQL_CallbackDelOld, query);
}
//删除数据回调.
void SQL_CallbackDelOld(Database db, DBResultSet results, const char[] error, any data) 
{
	if (!db || !results) 
	{
		LogError(error);
		return;
	}
}
stock bool CacheSteamID(int client) 
{
	if (g_sAuthId[client][0])
		return true;

	if (GetClientAuthId(client, AuthId_Steam2, g_sAuthId[client][0], sizeof(g_sAuthId[])))
		return true;

	g_sAuthId[client][0] = '\0';
	return false;
}
//玩家离开.
public void Event_PlayerDisconnect(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(GetEventInt(event,"userid"));
	
	if (client > 0 && !IsFakeClient(client))
		g_iDamageMultiple[client] = -1;
}
//打开难度菜单.
public Action Command_DifficultyMenu(int client, int args)
{
	if(bCheckClientAccess(client))
		OpenPlayerMenu(client, 0, false);
	else
		ReplyToCommand(client, "\x04[提示]\x05你无权使用该指令.");
	
	return Plugin_Handled;
}
public void OnLibraryRemoved(const char[] name)
{
	if (StrEqual(name, "adminmenu"))
		g_hTopMenu = null;
}
 
public void OnAdminMenuReady(Handle aTopMenu)
{
	TopMenu topmenu = TopMenu.FromHandle(aTopMenu);

	if (topmenu == g_hTopMenu)
		return;
	
	g_hTopMenu = topmenu;
	
	TopMenuObject hTopMenuObject = FindTopMenuCategory(g_hTopMenu, "OtherFeatures");
	if (hTopMenuObject == INVALID_TOPMENUOBJECT)
		hTopMenuObject = AddToTopMenu(g_hTopMenu, "OtherFeatures", TopMenuObject_Category, hMenuHandler, INVALID_TOPMENUOBJECT);
	
	hOtherFeatures = AddToTopMenu(g_hTopMenu,"sm_difficulty",TopMenuObject_Item, hHandlerMenu, hTopMenuObject,"sm_difficulty",ADMFLAG_ROOT);
}

void hMenuHandler(Handle topmenu, TopMenuAction action, TopMenuObject object_id, int param, char[] buffer, int maxlength)
{
	if (action == TopMenuAction_DisplayTitle)
	{
		Format(buffer, maxlength, "选择功能:", param);
	}
	else if (action == TopMenuAction_DisplayOption)
	{
		Format(buffer, maxlength, "其它功能", param);
	}
}

void hHandlerMenu(Handle topmenu, TopMenuAction action, TopMenuObject object_id, int param, char[] buffer, int maxlength)
{
	if (action == TopMenuAction_DisplayOption)
	{
		if (object_id == hOtherFeatures)
			Format(buffer, maxlength, "玩家难度", param);
	}
	else if (action == TopMenuAction_SelectOption)
	{
		if (object_id == hOtherFeatures)
		{
			OpenPlayerMenu(param, 0, true);
		}
	}
}
void OpenPlayerMenu(int client, int index, bool bButton = false)
{
	char line[32];
	char sInfo[128];
	char sName[64];
	char sData[4][64];
	Menu menu = new Menu(MenuOpenPlayerHandler);
	Format(line, sizeof(line), "选择玩家:");
	SetMenuTitle(menu, "%s", line);

	IntToString(-1, sData[0], sizeof(sData[]));
	strcopy(sData[1], sizeof(sData[]), "全部玩家");
	IntToString(index, sData[2], sizeof(sData[]));
	IntToString(bButton, sData[3], sizeof(sData[]));
	ImplodeStrings(sData, sizeof(sData), "|", sInfo, sizeof(sInfo));//打包字符串.
	menu.AddItem(sInfo, sData[1]);

	for(int i = 1; i <= MaxClients; i++)
	{
		if(IsClientInGame(i) && GetClientTeam(i) == 2)
		{
			int Bot = IsClientIdle(i);
			GetClientName(Bot != 0 ? Bot : i, sData[1], sizeof(sData[]));
			FormatEx(sName, sizeof(sName), "(%s)%s", g_iDamageMultiple[Bot != 0 ? Bot : i] == -1 ? "默认" : g_sDifficultyName[g_iDamageMultiple[Bot != 0 ? Bot : i]], sData[1]);
			IntToString(GetClientUserId(Bot != 0 ? Bot : i), sData[0], sizeof(sData[]));
			IntToString(bButton, sData[3], sizeof(sData[]));
			IntToString(index, sData[2], sizeof(sData[]));
			ImplodeStrings(sData, sizeof(sData), "|", sInfo, sizeof(sInfo));//打包字符串.
			menu.AddItem(sInfo, sName);
		}
	}
	menu.ExitButton = true;//默认值:true,设置为:false,则不显示退出选项.
	menu.ExitBackButton = bButton;
	menu.DisplayAt(client, index, MENU_TIME_FOREVER);
}

int MenuOpenPlayerHandler(Menu menu, MenuAction action, int client, int itemNum)
{
	switch(action)
	{
		case MenuAction_Select:
		{
			char sItem[128], sName[32], sInfo[4][64];
			menu.GetItem(itemNum, sItem, sizeof(sItem), _, sName, sizeof(sName));
			ExplodeString(sItem, "|", sInfo, sizeof(sInfo), sizeof(sInfo[]));//拆分字符串.
			IntToString(menu.Selection, sInfo[2], sizeof(sInfo[]));
			OpenDifficultyMenu(client, sInfo[0], sInfo[1], sInfo[2], sInfo[3]);
		}
		case MenuAction_Cancel:
		{
			if (itemNum == MenuCancel_ExitBack && g_hTopMenu != null)
				g_hTopMenu.Display(client, TopMenuPosition_LastCategory);
		}
		case MenuAction_End:
			delete menu;
	}
	return 0;
}
void OpenDifficultyMenu(int client, char[] sItem, char[] sName, char[] sIndex, char[] sButton)
{
	char line[32];
	char sInfo[128];
	char sData[7][32];
	Menu menu = new Menu(MenuOpenDifficultyHandler);
	
	Format(line, sizeof(line), "选择难度:");
	SetMenuTitle(menu, "%s", 	line);
	
	IntToString(-1, sData[0], sizeof(sData[]));
	strcopy(sData[1], sizeof(sData[]), sItem);
	strcopy(sData[2], sizeof(sData[]), sName);
	strcopy(sData[4], sizeof(sData[]), "默认");
	strcopy(sData[5], sizeof(sData[]), sButton);
	strcopy(sData[6], sizeof(sData[]), sIndex);
	ImplodeStrings(sData, sizeof(sData), "|", sInfo, sizeof(sInfo));//打包字符串.
	menu.AddItem(sInfo, "默认");

	for (int i = 0; i < sizeof(g_sDifficultyCode); i++)
	{
		IntToString(i, sData[0], sizeof(sData[]));
		strcopy(sData[1], sizeof(sData[]), sItem);
		strcopy(sData[2], sizeof(sData[]), sName);
		strcopy(sData[3], sizeof(sData[]), g_sDifficultyCode[i]);
		strcopy(sData[4], sizeof(sData[]), g_sDifficultyName[i]);
		strcopy(sData[5], sizeof(sData[]), sButton);
		strcopy(sData[6], sizeof(sData[]), sIndex);
		ImplodeStrings(sData, sizeof(sData), "|", sInfo, sizeof(sInfo));//打包字符串.
		menu.AddItem(sInfo, g_sDifficultyName[i]);
	}

	menu.ExitButton = true;//默认值:true,设置为:false,则不显示退出选项.
	menu.ExitBackButton = view_as<bool>(StringToInt(sButton));
	DisplayMenu(menu, client, MENU_TIME_FOREVER);
}

int MenuOpenDifficultyHandler(Menu menu, MenuAction action, int client, int itemNum)
{
	switch(action)
	{
		case MenuAction_Select:
		{
			char sItem[128];
			if(menu.GetItem(itemNum, sItem, sizeof(sItem)))
			{
				char sInfo[7][32];
				ExplodeString(sItem, "|", sInfo, sizeof(sInfo), sizeof(sInfo[]));//拆分字符串.
				if(StringToInt(sInfo[1]) == -1)
				{
					for(int i = 1; i <= MaxClients; i++)
					{
						if(IsClientInGame(i) && GetClientTeam(i) == 2)
						{
							int Bot = IsClientIdle(i);
							
							if(Bot != 0)
							{
								g_iDamageMultiple[Bot] = StringToInt(sInfo[0]);
								SQL_Load(Bot, true);//读取玩家数据.
								PrintToChat(Bot, "\x04[提示]\x05已设置\x03%s\x05为\x04%s\x05难度.", sInfo[2], sInfo[4]);
								g_hDifficulty.ReplicateToClient(Bot, g_iDamageMultiple[Bot] == -1 ? GetGameDifficultyName() : g_sDifficultyCode[g_iDamageMultiple[Bot]]);
							}
							else
							{
								g_iDamageMultiple[i] = StringToInt(sInfo[0]);

								if(!IsFakeClient(i))
								{
									SQL_Load(i, true);//读取玩家数据.
									PrintToChat(i, "\x04[提示]\x05已设置\x03%s\x05为\x04%s\x05难度.", sInfo[2], sInfo[4]);
									g_hDifficulty.ReplicateToClient(i, g_iDamageMultiple[i] == -1 ? GetGameDifficultyName() : g_sDifficultyCode[g_iDamageMultiple[i]]);
								}
							}
						}
					}
				}
				else
				{
					int victim = GetClientOfUserId(StringToInt(sInfo[1]));
					if(IsValidClient(victim))
					{
						g_iDamageMultiple[victim] = StringToInt(sInfo[0]);
						
						if(!IsFakeClient(victim))
						{
							SQL_Load(victim, true);//读取玩家数据.
							if(victim != client)
								PrintToChat(client, "\x04[提示]\x05已设置\x03%s\x05为\x04%s\x05难度.", sInfo[2], sInfo[4]);
							PrintToChat(victim, "\x04[提示]\x05已设置你的难度为\x03%s\x05难度.", sInfo[4]);
							g_hDifficulty.ReplicateToClient(victim, g_iDamageMultiple[victim] == -1 ? GetGameDifficultyName() : g_sDifficultyCode[g_iDamageMultiple[victim]]);
						}
					}
				}
				
				OpenPlayerMenu(client, StringToInt(sInfo[6]), view_as<bool>(StringToInt(sInfo[5])));
			}
		}
		case MenuAction_Cancel:
		{
			if (itemNum == MenuCancel_ExitBack && g_hTopMenu != null)
				g_hTopMenu.Display(client, TopMenuPosition_LastCategory);
		}
		case MenuAction_End:
			delete menu;
	}
	return 0;
}
//玩家加入游戏时.
public void OnClientPutInServer(int client)
{
	//钩住玩家受伤.
	SDKHook(client, SDKHook_OnTakeDamage, OnTakeDamage);
}
//玩家受伤钩子回调.
public Action OnTakeDamage(int victim, int &attacker, int &inflictor, float &damage, int &damagetype, int &weapon, float damageForce[3], float damagePosition[3])
{ 
	if(IsValidClient(victim) && GetClientTeam(victim) == 2)
	{
		int Bot = IsClientIdle(victim);
		if(GetGameDifficultyIndex() == g_iDamageMultiple[Bot != 0 ? Bot : victim])
		{
			#if DEBUG
			PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05选择的难度与当前难度相同.");
			#endif
			return Plugin_Continue;//不执行任何操作.
		}

		if(g_iDamageMultiple[Bot != 0 ? Bot : victim] == -1)
			return Plugin_Continue;//不执行任何操作.

		if(IsValidClient(attacker) && GetClientTeam(attacker) == 3)
		{
			int iHLZClass = GetEntProp(attacker, Prop_Send, "m_zombieClass");

			switch(iHLZClass)
			{
				case 1:
				{
					if (GetEntPropEnt(victim, Prop_Send, "m_tongueOwner") > 0)
					{
						damage = g_fSmokerDamageMultiple[1][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Changed;
					}
					else
					{
						damage = g_fSmokerDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Changed;
					}
				}
				case 2:
				{
					damage = g_fBoomerDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
					#if DEBUG
					PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
					#endif
					return Plugin_Changed;
				}
				case 3:
				{
					if (GetEntPropEnt(victim, Prop_Send, "m_pounceAttacker") > 0)
					{
						damage = g_fHunterDamageMultiple[1][g_iDamageMultiple[Bot != 0 ? Bot : victim]] * (IsPlayerFallen(victim) ? 3.0 : 1.0);//猎人对倒地的三倍伤害.
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Changed;
					}
					else
					{
						damage = g_fHunterDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Changed;
					}
				}
				case 4:
				{
					if(IsValidEntity(inflictor))
					{
						char classname[32];
						GetEntityClassname(inflictor, classname, sizeof classname);
						if (strcmp(classname, "insect_swarm") != 0)
						{
							damage = g_fSpitterDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
							#if DEBUG
							PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
							#endif
							return Plugin_Changed;
						}
						else
						{
							if(g_iDamageMultiple[Bot != 0 ? Bot : victim] == 0 && GetGameDifficultyIndex() > 0)
							{
								damage *= g_fSpitterDamageMultiple[1][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
								#if DEBUG
								PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示1]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
								#endif
								return Plugin_Changed;
							}
						}
					}
				}
				case 5:
				{
					if (GetEntPropEnt(victim, Prop_Send, "m_jockeyAttacker") > 0)
					{
						damage = g_fJockeyDamageMultiple[1][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Changed;
					}
					else
					{
						damage = g_fJockeyDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Changed;
					}
				}
				case 6:
				{
					if(GetEntPropEnt(victim, Prop_Send, "m_carryAttacker") > 0)
					{
						damage = g_fChargerDamageMultiple[1][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示1]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Changed;
					}
					else if(GetEntPropEnt(victim, Prop_Send, "m_pummelAttacker") > 0)
					{
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示2]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Continue;//不执行任何操作.
					}
					else
					{
						damage = g_fChargerDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示3]\x05%s受到了%f点伤害(%d).", GetTrueName(victim), damage, iHLZClass);
						#endif
						return Plugin_Changed;
					}
				}
				case 8:
				{
					int Weapon = GetEntPropEnt(attacker, Prop_Send, "m_hActiveWeapon");
					if(IsValidEntity(Weapon))
					{
						char classname[32];
						GetEntityClassname(Weapon, classname, sizeof classname);
						if (strcmp(classname, "weapon_tank_claw") == 0)//坦克拳头(坦克石头也可以触发这个类名,而且伤害是一样的).
						{
							if(IsPlayerFallen(victim))
							{
								damage = g_fTankDamageMultiple[1][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
								#if DEBUG
								PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d)(%s).", GetTrueName(victim), damage, iHLZClass, classname);
								#endif
								return Plugin_Changed;
							}
							else
							{
								damage = g_fTankDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
								#if DEBUG
								PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害(%d)(%s).", GetTrueName(victim), damage, iHLZClass, classname);
								#endif
								return Plugin_Changed;
							}
						}
					}
					//if(IsValidEntity(inflictor))
					//{
					//	GetEntityClassname(inflictor, classname, sizeof classname);//坦克石头.
					//	PrintToChat(victim, "\x04[提示2]\x05%s受到了%f点伤害(%d)(%s).", GetTrueName(victim), damage, iHLZClass, classname);
					//}
				}
			}
		}
		else
		{
			if (IsValidEntity(attacker)) 
			{
				char classname[32];
				GetEntityClassname(attacker, classname, sizeof classname);

				if (strcmp(classname, "insect_swarm") == 0)
				{
					if(g_iDamageMultiple[Bot != 0 ? Bot : victim] == 0 && GetGameDifficultyIndex() > 0)
					{
						damage *= g_fSpitterDamageMultiple[1][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示1]\x05%s受到了%f点伤害.", GetTrueName(victim), damage);
						#endif
						return Plugin_Changed;
					}
				}
				else if (strcmp(classname, "infected") == 0)
				{
					if(IsPlayerState(victim))
					{
						damage = g_fInfectedDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害.", GetTrueName(victim), damage);
						#endif
						return Plugin_Changed;
					}
					//else//伤害好像全是一样的,不需要改.
					//{
					//	damage = g_fInfectedDamageMultiple[1][g_iDamageMultiple[victim]];
					//	PrintToChat(victim, "\x04[提示]\x05%s受到了%f点伤害.", GetTrueName(victim), damage);
					//	return Plugin_Changed;
					//}
				}
				else if (strcmp(classname, "witch") == 0)
				{
					if(!IsPlayerState(victim))
					{
						damage = g_fWitchDamageMultiple[0][g_iDamageMultiple[Bot != 0 ? Bot : victim]];
						#if DEBUG
						PrintToChat(Bot != 0 ? Bot : victim, "\x04[提示]\x05%s受到了%f点伤害.", GetTrueName(victim), damage);
						#endif
						return Plugin_Changed;
					}
					//else
					//{
					//	if(g_iDamageMultiple[victim] != 3 && GetGameDifficultyIndex() == 3 && damage >= 100.0)//判断血量大于或等于100伤害.
					//	{
					//		SDKHooks_TakeDamage(victim, inflictor, attacker, damage);//设置指定的伤害(专家难度下设置女巫对生还者的伤害也会秒杀生还者).
					//		return Plugin_Handled;
					//	}
					//	damage = g_fWitchDamageMultiple[1][g_iDamageMultiple[victim]];
					//	PrintToChat(victim, "\x04[提示]\x05%s受到了%f点伤害.", GetTrueName(victim), damage);
					//	return Plugin_Changed;
					//}
				}
			}
		}
	}
	return Plugin_Continue;//不执行任何操作.
}
//倒地状态.
bool IsPlayerFallen(int client)
{
	return GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}
//正常状态.
stock bool IsPlayerState(int client)
{
	return !GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}
stock int GetGameDifficultyIndex()
{
	char sDifficulty[32];
	GetConVarString(FindConVar("z_Difficulty"), sDifficulty, sizeof(sDifficulty));

	for (int i = 0; i < sizeof(g_sDifficultyCode); i++)
		if(strcmp(g_sDifficultyCode[i], sDifficulty, false) == 0)
			return i;
	return -1;
}
stock char[] GetGameDifficultyName()
{
	char sDifficulty[32];
	GetConVarString(FindConVar("z_Difficulty"), sDifficulty, sizeof(sDifficulty));
	return sDifficulty;
}
stock bool bCheckClientAccess(int client)
{
	if(GetUserFlagBits(client) & ADMFLAG_ROOT)
		return true;
	return false;
}
//玩家有效.
stock bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}
stock char[] GetTrueName(int client)
{
	char sName[32];
	int Bot = IsClientIdle(client);
	
	if(Bot != 0)
		FormatEx(sName, sizeof(sName), "闲置:%N", Bot);
	else
		GetClientName(client, sName, sizeof(sName));
	return sName;
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
int IsClientIdle(int client) 
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}
void CreateNatives()
{
	CreateNative("GetCustomizeDifficulty",	GetNativeCustomizeDifficulty);
	CreateNative("SetCustomizeDifficulty",	SetNativeCustomizeDifficulty);
}
int GetNativeCustomizeDifficulty(Handle plugin, int numParams)
{
	int client = GetNativeCell(1);
	if(IsValidClient(client) && GetClientTeam(client) == 2)
	{
		int Bot = IsClientIdle(client);
		return g_iDamageMultiple[Bot != 0 ? Bot : client];
	}
	return 0;
}
int SetNativeCustomizeDifficulty(Handle plugin, int numParams)
{
	int client = GetNativeCell(1);
	int value = GetNativeCell(2);
	if(value >= -1 && value <= 3 && IsValidClient(client))
	{
		switch(GetClientTeam(client))
		{
			case 1:
			{
				if(!IsFakeClient(client))
				{
					int iBot = iGetBotOfIdlePlayer(client);
					if(iBot != 0)
					{
						g_iDamageMultiple[client] = value;
						SQL_Load(client, true);//读取玩家数据.
						PrintToChat(client, "\x04[提示]\x05已设置\x03%N\x05为\x04%s\x05难度.", client, g_sDifficultyName[g_iDamageMultiple[client]]);
						g_hDifficulty.ReplicateToClient(client, g_iDamageMultiple[client] == -1 ? GetGameDifficultyName() : g_sDifficultyCode[g_iDamageMultiple[client]]);
						return 1;
					}
				}
			}
			case 2:
			{
				int iBot = IsClientIdle(client);
		
				if(iBot != 0)
				{
					g_iDamageMultiple[iBot] = value;
					SQL_Load(iBot, true);//读取玩家数据.
					PrintToChat(iBot, "\x04[提示]\x05已设置\x03%N\x05为\x04%s\x05难度.", iBot, g_sDifficultyName[g_iDamageMultiple[iBot]]);
					g_hDifficulty.ReplicateToClient(iBot, g_iDamageMultiple[iBot] == -1 ? GetGameDifficultyName() : g_sDifficultyCode[g_iDamageMultiple[iBot]]);
					return 1;
				}
				else
				{
					g_iDamageMultiple[client] = value;

					if(!IsFakeClient(client))
					{
						SQL_Load(client, true);//读取玩家数据.
						PrintToChat(client, "\x04[提示]\x05已设置\x03%N\x05为\x04%s\x05难度.", client, g_sDifficultyName[g_iDamageMultiple[client]]);
						g_hDifficulty.ReplicateToClient(client, g_iDamageMultiple[client] == -1 ? GetGameDifficultyName() : g_sDifficultyCode[g_iDamageMultiple[client]]);
					}
					return 1;
				}
			}
		}
	}
	return 0;
}