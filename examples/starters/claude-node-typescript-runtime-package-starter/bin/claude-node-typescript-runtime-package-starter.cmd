@echo off
setlocal
set "ROOT=%~dp0.."
node "%ROOT%\dist\main.js" %*
