@echo off
rem ============================================================
rem  SpeedTickr installer for Command Prompt (cmd.exe).
rem
rem  This simply runs the PowerShell installer with the right
rem  settings, so you don't have to type any of that yourself.
rem  It installs per-user (no admin) and launches SpeedTickr,
rem  which then starts at login automatically.
rem ============================================================
echo Installing SpeedTickr...
powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://speedtickr.com/install.ps1 | iex"
