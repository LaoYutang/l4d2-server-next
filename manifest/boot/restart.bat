@echo off
REM 关闭 L4D2 服务器
taskkill /IM srcds.exe /F

@REM 备选，按照端口关闭进程
@REM for /f "tokens=5" %%a in ('netstat -ano ^| findstr :27015') do (
@REM     taskkill /PID %%a /F 2>nul
@REM )

REM 等待 2 秒避免端口占用
timeout /t 2 /nobreak >nul

REM 启动 L4D2 服务器（后台运行）
start "" "./srcds.exe" -game left4dead2 -console -insecure -tickrate 60 -condebug +hostport 27015 +exec server.cfg