#if defined _EvilSimulation
 #endinput
#endif
#define _EvilSimulation

public SharedPlugin __pl_l4d2_simulation = 
{
	name = "l4d2_simulation",
	file = "l4d2_simulation.smx",
#if defined REQUIRE_PLUGIN
	required = 1,
#else
	required = 0,
#endif
};

#if !defined REQUIRE_PLUGIN
public void __pl_l4d2_simulation_SetNTVOptional()
{
	MarkNativeAsOptional("GetCustomizeDifficulty");
	MarkNativeAsOptional("SetCustomizeDifficulty");
}
#endif

/**
 * 获取玩家当前难度.
 *
 * @param client	        传入玩家的index索引.
 *
 * @return					返回玩家难度索引(-1=默认,0=简单,1=普通,2=高级,=3=专家).
 */
native int GetCustomizeDifficulty(int client);

/**
 * 设置玩家当前难度.
 *
 * @param client	        传入玩家的index索引(传入的索引必须是生还者).
 * @param value				需要设置的难度(-1=默认,0=简单,1=普通,2=高级,=3=专家).
 *
 * @return					成功时=true,反之=false.
 */
native bool SetCustomizeDifficulty(int client, int value);