@echo off
setlocal enabledelayedexpansion

:: 切换到包根目录（保证 web/dist 路径正确）
set DIR=%~dp0..
cd /d "%DIR%" || exit /b 1

if not exist "logs" mkdir logs
if not exist "config" mkdir config

echo Starting IEC104 Sim...
start /B "" "bin\iec104-sim.exe" serve --http :8080 --config-dir config --log-dir logs --log info > logs\output.log 2>&1
if errorlevel 1 (
    echo Failed to start IEC104 Sim
    pause
    exit /b 1
)
echo IEC104 Sim started
echo Web UI: http://localhost:8080