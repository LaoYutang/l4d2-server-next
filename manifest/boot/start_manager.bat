@echo off
:: 设置 Web 管理后台的端口，不能重复
set L4D2_MANAGER_PORT=27020

:: 设置管理器密码，不要提供给别人，临时请使用授权码
set L4D2_MANAGER_PASSWORD=管理器密码

:: 设置addons路径
set L4D2_ADDONS_PATH=addons文件夹目录

:: 设置对应的游戏服务器 RCON 地址
set L4D2_RCON_URL=127.0.0.1:27015

:: 设置RCON 密码 (如果需要)
set L4D2_RCON_PASSWORD=RCON密码

:: 设置RCON方式进行重启，需要服务器具备自动重启功能
set L4D2_RESTART_BY_RCON=true

:: 设置steamapi key，用于偷看服务器中文件的游戏时长
set STEAM_API_KEY=


:loop
echo =================================================
echo       L4D2 Manager Starting...
echo =================================================

:: 启动管理程序
l4d2-manager.exe

echo.
echo [WARNING] Server crashed or closed!
echo Restarting in 3 seconds...
echo Press Ctrl+C to stop the monitor.
timeout /t 3 >nul
goto loop

