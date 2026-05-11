@echo off
echo Stopping IEC104 Sim...
taskkill /F /IM iec104-sim.exe 2>nul
if errorlevel 1 (
    echo IEC104 Sim not running.
) else (
    echo IEC104 Sim stopped.
)