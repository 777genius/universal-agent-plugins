@echo off
setlocal
set "ROOT=%~dp0.."
node "%ROOT%\plugin\main.mjs" %*
