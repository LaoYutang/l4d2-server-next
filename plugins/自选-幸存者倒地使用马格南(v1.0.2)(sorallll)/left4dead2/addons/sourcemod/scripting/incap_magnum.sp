#pragma semicolon 1
#pragma newdecls required
#include <sourcemod>
#include <sourcescramble>

#define GAMEDATA	"incap_magnum"

public Plugin myinfo =
{
	name = "Incapped Magnum",
	author = "sorallll",
	version	= "1.0.2",
	description	= "将倒地武器修改为Magnum",
	url = "https://github.com/umlka/l4d2/tree/main/incap_magnum"
};

public void OnPluginStart()
{
	char sPath[PLATFORM_MAX_PATH];
	BuildPath(Path_SM, sPath, sizeof sPath, "gamedata/%s.txt", GAMEDATA);
	if (!FileExists(sPath))
		SetFailState("\n==========\nMissing required file: \"%s\".\n==========", sPath);

	GameData hGameData = new GameData(GAMEDATA);
	if (!hGameData)
		SetFailState("Failed to load \"%s.txt\" gamedata.", GAMEDATA);

	MemoryPatch patch = MemoryPatch.CreateFromConf(hGameData, "CTerrorPlayer::OnIncapacitatedAsSurvivor::IncappedWeapon");
	if (!patch.Validate())
		SetFailState("Failed to verify patch: \"CTerrorPlayer::OnIncapacitatedAsSurvivor::IncappedWeapon\"");
	else if (patch.Enable()) {
		StoreToAddress(patch.Address + view_as<Address>(hGameData.GetOffset("OS") ? 4 : 1), view_as<int>(GetAddressOfString("weapon_pistol_magnum")), NumberType_Int32);
		PrintToServer("[%s] Enabled patch: \"CTerrorPlayer::OnIncapacitatedAsSurvivor::IncappedWeapon\"", GAMEDATA);
	}

	delete hGameData;
}