/*
 *
 *	v1.0.0
 *
 *	1:初始版本发布.
 *
 *	v1.1.0
 *
 *	1:新增什么情况下玩家主动离开游戏会被临时封禁.
 *
 *	v1.1.1
 *
 *	1:第二个参数忘记加大于0的判断.
 *
 *	v1.1.2
 *
 *	1:如果玩家离开游戏是因为被踢出则不执行离开游戏封禁检查(防止有憨批设置离开原因为主动离开原因卡BUG).
 *
 */
#pragma semicolon 1
//強制1.7以後的新語法
#pragma newdecls required
#include <sourcemod>
#undef REQUIRE_PLUGIN	//标记为可选开始.
#include <l4d2_banned>	//数据库封禁玩家.
#define REQUIRE_PLUGIN	//标记为可选结束.

#define CVAR_FLAGS		FCVAR_NOTIFY
#define PLUGIN_VERSION	"1.1.2"	//插件的版本.

#define	Number_1		(1 << 0)
#define Number_2		(1 << 1)
#define Number_4		(1 << 2)
#define Number_8		(1 << 3)
#define Number_16		(1 << 4)
#define Number_32		(1 << 5)
#define Number_64		(1 << 6)
#define Number_128		(1 << 7)
#define Number_256		(1 << 8)

bool g_bLibraries;

int    g_iRepeatEnterTimer, g_iRepeatEnterType;
ConVar g_hRepeatEnterTimer, g_hRepeatEnterType;

//定义插件信息.
public Plugin myinfo =  
{
	name = "l4d2_repeat_enter",
	author = "豆瓣酱な",
	description = "玩家主动离开游戏时自动封禁",
	version = PLUGIN_VERSION,
	url = "N/A"
};
//插件开始.
public void OnPluginStart()
{
	HookEvent("player_disconnect", Event_PlayerDisconnect, EventHookMode_Pre);//玩家离开.

	g_hRepeatEnterTimer = CreateConVar("l4d2_repeat_enter_time", "1", "自动封禁离开的玩家(分钟). 0=禁用.", CVAR_FLAGS);
	g_hRepeatEnterType = CreateConVar("l4d2_repeat_enter_type", "63", "什么情况下离开时执行封禁(只限主动离开游戏的情况). 0=禁用, 1=倒地, 2=挂边, 4=瘸腿血量, 8=死亡, 16=黑白, 32=被控时, 63=全部.", CVAR_FLAGS);
	g_hRepeatEnterTimer.AddChangeHook(ConVarChanged);
	g_hRepeatEnterType.AddChangeHook(ConVarChanged);
	AutoExecConfig(true, "l4d2_repeat_enter");//生成指定文件名的CFG.
}
//地图开始
public void OnMapStart()
{
	GetCvars();
}
//参数更改回调.
public void ConVarChanged(ConVar convar, const char[] oldValue, const char[] newValue)
{
	GetCvars();
}
//重新赋值.
void GetCvars()
{
	g_iRepeatEnterTimer = g_hRepeatEnterTimer.IntValue;
	g_iRepeatEnterType = g_hRepeatEnterType.IntValue;
}
//所有插件加载完成后执行一次(延迟加载插件也会执行一次).
public void OnAllPluginsLoaded()   
{
	g_bLibraries = LibraryExists("l4d2_banned");
}
//库被加载时.
public void OnLibraryAdded(const char[] name) 
{
	if (strcmp(name, "l4d2_banned") == 0)
		g_bLibraries = true;
}
//库被卸载时.
public void OnLibraryRemoved(const char[] name) 
{
	if (strcmp(name, "l4d2_banned") == 0)
		g_bLibraries = false;
}
//玩家离开.
public void Event_PlayerDisconnect(Event event, const char[] name, bool dontBroadcast)
{
	int client = GetClientOfUserId(GetEventInt(event,"userid"));
	
	if (IsValidClient(client) && !IsFakeClient(client) && !IsClientInKickQueue(client) && g_iRepeatEnterTimer > 0 && g_iRepeatEnterType > 0)//(IsClientInKickQueue)函数是即将被踢的玩家列表.
	{
		char sName[128], sReason[128];
		event.GetString("name", sName, sizeof(sName));
		event.GetString("reason", sReason, sizeof(sReason));//获取玩家离开游戏的原因(好像被踢出时填写的原因也会显示到这里).
		
		if (strcmp(sReason, "Disconnect by user.") == 0)//这里是主动离开游戏.
		{
			switch(GetClientTeam(client))//判断玩家团队
			{
				case 1://玩家是观察者.
				{
					int iBot = iGetBotOfIdlePlayer(client);//获取观察者对应的电脑生还者索引.
					if(iBot != 0)//玩家是闲置状态(反之是旁观者状态).
						if(GetExecutionType(iBot))//判断闲置玩家对应的电脑生还者状态.
							IsExecuteBanClient(client);//结果为真时执行封禁玩家.
				}
				case 2://玩家是生还者.
				{
					if(GetExecutionType(client))//判断玩家对应电脑的生还者状态.
						IsExecuteBanClient(client);//结果为真时执行封禁玩家.
				}
				case 3://玩家是感染者.
				{}
			}
		}
	}
}
//返回闲置玩家对应的电脑.
stock int iGetBotOfIdlePlayer(int client)
{
	for (int i = 1; i <= MaxClients; i++)
	{
		if (IsClientInGame(i) && IsFakeClient(i) && GetClientTeam(i) == 2 && IsClientIdle(i) == client)
			return i;
	}
	return 0;
}
//返回生还者对应的玩家.
stock int IsClientIdle(int client)
{
	if (!HasEntProp(client, Prop_Send, "m_humanSpectatorUserID"))
		return 0;

	return GetClientOfUserId(GetEntProp(client, Prop_Send, "m_humanSpectatorUserID"));
}
//判断当前类型.
bool GetExecutionType(int client)
{
	if(g_iRepeatEnterType & Number_1)
		if(IsPlayerAlive(client) && IsPlayerFallen(client))//倒地状态.
			return true;

	if(g_iRepeatEnterType & Number_2)
		if(IsPlayerAlive(client) && IsPlayerFalling(client))//挂边状态.
			return true;
	
	if(g_iRepeatEnterType & Number_4)
		if(IsPlayerAlive(client) && GetSurvivorHP(client) < IsLamePerson())//总血量小于瘸腿血量.
			return true;
	
	if(g_iRepeatEnterType & Number_8)
		if(!IsPlayerAlive(client))//死亡的生还者(如果没有判断团队等情况也会过这个判断,因为这个函数准确来说是非存活).
			return true;
	
	if(g_iRepeatEnterType & Number_16)
		if(IsPlayerAlive(client) && GetDisabilityNumber() > 0 && IsReviveCount(client))//最大倒地次数必须大于0才会执行后面的,黑白的生还者.
			return true;

	if(g_iRepeatEnterType & Number_32)
		if(IsPlayerAlive(client) && IsSurvivorControl(client))//生还者被特感控制时.
			return true;
	
	return false;
}
//获取失能次数.
stock int GetDisabilityNumber()
{
	return GetConVarInt(FindConVar("survivor_max_incapacitated_count"));
}
//判断黑白状态.
stock bool IsReviveCount(int client)
{
	return GetEntProp(client, Prop_Send, "m_currentReviveCount") >= GetDisabilityNumber();
}
//正常状态.
stock bool IsPlayerState(int client)
{
	return !GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}
//倒地状态.
stock bool IsPlayerFallen(int client)
{
	return GetEntProp(client, Prop_Send, "m_isIncapacitated") && !GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}
//挂边状态.
stock bool IsPlayerFalling(int client)
{
	return GetEntProp(client, Prop_Send, "m_isIncapacitated") && GetEntProp(client, Prop_Send, "m_isHangingFromLedge");
}
//瘸腿血量.
stock int IsLamePerson()
{
	return GetConVarInt(FindConVar("survivor_limp_health"));
}
//幸存者总血量.
stock int GetSurvivorHP(int client)
{
	return GetClientHealth(client) + GetPlayerTempHealth(client);
}
//幸存者虚血量.
stock int GetPlayerTempHealth(int client)
{
	static Handle painPillsDecayCvar;
	painPillsDecayCvar = FindConVar("pain_pills_decay_rate");
	if (painPillsDecayCvar == null)
		return -1;

	int tempHealth = RoundToCeil(GetEntPropFloat(client, Prop_Send, "m_healthBuffer") - ((GetGameTime() - GetEntPropFloat(client, Prop_Send, "m_healthBufferTime")) * GetConVarFloat(painPillsDecayCvar))) - 1;
	return tempHealth < 0 ? 0 : tempHealth;
}
//根据生还者索引判断是否被感染者控制.
stock bool IsSurvivorControl(int client)
{
	if(GetEntPropEnt(client, Prop_Send, "m_tongueOwner") > 0)
		return true;

	if(GetEntPropEnt(client, Prop_Send, "m_pounceAttacker") > 0)
		return true;

	if(GetEntPropEnt(client, Prop_Send, "m_jockeyAttacker") > 0)
		return true;

	if(GetEntPropEnt(client, Prop_Send, "m_carryAttacker") > 0 || GetEntPropEnt(client, Prop_Send, "m_pummelAttacker") > 0)
		return true;

	return false;
}
//根据感染者索引判断生还者是否被控制.
stock bool IsControlSurvivor(int attacker)
{
	if(GetEntPropEnt(attacker, Prop_Send, "m_tongueVictim") > 0)
		return true;

	if(GetEntPropEnt(attacker, Prop_Send, "m_pounceVictim") > 0)
		return true;

	if(GetEntPropEnt(attacker, Prop_Send, "m_jockeyVictim") > 0)
		return true;

	if(GetEntPropEnt(attacker, Prop_Send, "m_carryVictim") > 0 || GetEntPropEnt(attacker, Prop_Send, "m_pummelVictim") > 0)
		return true;

	return false;
}
stock bool IsValidClient(int client)
{
	return client > 0 && client <= MaxClients && IsClientInGame(client);
}
//执行封禁玩家.
void IsExecuteBanClient(int client)
{
	if(g_bLibraries == false)
		BanClient(client, g_iRepeatEnterTimer, BANFLAG_AUTO, "防止重复加入的自动封禁", "防止重复加入的自动封禁", "sm_ban", client);
	else
		BanPlayers(client, g_iRepeatEnterTimer, BANFLAG_AUTO, "防止重复加入的自动封禁", "防止重复加入的自动封禁", "sm_ban", client);
}