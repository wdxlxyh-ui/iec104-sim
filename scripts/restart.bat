@echo off
echo Restarting IEC104 Sim...
call "%~dp0stop.bat"
timeout /t 2 /nobreak >nul
call "%~dp0start.bat"
echo Restart completed.
