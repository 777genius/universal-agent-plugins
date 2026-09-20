@echo off
setlocal
set "ROOT=%~dp0.."
if exist "%ROOT%\.venv\Scripts\python.exe" (
  set "PYTHON=%ROOT%\.venv\Scripts\python.exe"
) else if not "%PYTHON%"=="" (
  rem Preserve caller override when present.
) else (
  where python >nul 2>nul
  if not errorlevel 1 (
    set "PYTHON=python"
  ) else (
    where python3 >nul 2>nul
    if not errorlevel 1 (
      set "PYTHON=python3"
    ) else (
      echo plugin-kit-ai launcher: python not found >&2
      exit /b 1
    )
  )
)
"%PYTHON%" "%ROOT%\plugin\main.py" %*
