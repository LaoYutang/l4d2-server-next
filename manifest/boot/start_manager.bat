@echo off
cd /d %~dp0

:: ================= Configuration Area =================

:: Set Web Manager Port (Must be unique)
set L4D2_MANAGER_PORT=27020

:: Set Manager Password (DO NOT SHARE. Use authorization codes for temporary access)
set L4D2_MANAGER_PASSWORD=my_secure_password

:: Set Left4Dead2 Game Directory (e.g., D:\SteamCMD\steamapps\common\Left 4 Dead 2 Dedicated Server\left4dead2)
set L4D2_GAME_PATH=D:\path\to\left4dead2

:: Set Game Server RCON URL
set L4D2_RCON_URL=127.0.0.1:27015

:: Set RCON Password
set L4D2_RCON_PASSWORD=my_rcon_password

:: Set Restart by RCON (Server must support auto-restart)
set L4D2_RESTART_BY_RCON=true

:: Set Steam API Key (Optional, used for tracking playtime)
set STEAM_API_KEY=

:: ================= Check Area =================

if not exist "l4d2-manager.exe" (
    echo [ERROR] l4d2-manager.exe not found!
    echo Please ensure this script is in the same directory as l4d2-manager.exe.
    echo Or download the latest Windows release from the Release page.
    pause
    exit /b 1
)

if "%L4D2_GAME_PATH%"=="D:\path\to\left4dead2" (
    echo [WARNING] L4D2_GAME_PATH is not configured!
    echo Please edit this script to set the actual L4D2_GAME_PATH.
    echo Press any key to continue, but some file features may not work...
    pause
)

if "%L4D2_MANAGER_PASSWORD%"=="my_secure_password" (
    echo [WARNING] You are using the default manager password!
    echo It is recommended to edit this script to change L4D2_MANAGER_PASSWORD.
)

:: ================= Start Area =================

:loop
echo =================================================
echo       L4D2 Manager Starting...
echo =================================================

:: Start Manager
l4d2-manager.exe

echo.
echo [WARNING] Server crashed or closed!
echo Restarting in 3 seconds...
echo Press Ctrl+C to stop the monitor.
timeout /t 3 >nul
goto loop

