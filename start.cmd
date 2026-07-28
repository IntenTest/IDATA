@echo off
setlocal

cd /d "%~dp0app"

where py >nul 2>nul
if %errorlevel% equ 0 (
    set "PYTHON_COMMAND=py -3"
) else (
    set "PYTHON_COMMAND=python"
)

echo Stopping the existing Oh Wemby service...
%PYTHON_COMMAND% stop.py
if errorlevel 1 (
    echo.
    echo Failed to stop the existing service.
    pause
    exit /b 1
)

echo.
echo Starting Oh Wemby...
%PYTHON_COMMAND% start.py

if errorlevel 1 (
    echo.
    echo Failed to start Oh Wemby.
    pause
    exit /b 1
)

endlocal
