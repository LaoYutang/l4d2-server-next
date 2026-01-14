//小手枪替换成马格南
DirectorOptions <-
{
	weaponsToConvert =
	{
		weapon_pistol = "weapon_pistol_magnum_spawn"
	}
	function ConvertWeaponSpawn( classname )
	{
		if ( classname in weaponsToConvert )
		{
			return weaponsToConvert[classname];
		}
		return 0;
	}
}
