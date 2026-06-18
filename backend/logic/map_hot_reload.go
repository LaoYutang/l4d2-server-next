package logic

var executeMapHotReloadRconCommand = ExecuteRconCommand

func ExecuteMapHotReloadCommand() (string, error) {
	return executeMapHotReloadRconCommand(GetMapHotReloadCommand())
}
