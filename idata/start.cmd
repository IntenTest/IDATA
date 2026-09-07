@echo off
setlocal

set "APP_DIRECTORY=%~dp0app"
if not exist "%APP_DIRECTORY%\start.py" (
    set "APP_DIRECTORY=%~dp0IDATA-main\app"
)

if not exist "%APP_DIRECTORY%\start.py" (
    echo Unable to find the IDATA application directory.
    echo Expected it at "%~dp0app" or "%~dp0IDATA-main\app".
    pause
    exit /b 1
)

cd /d "%APP_DIRECTORY%"

python --version >nul 2>nul
if not errorlevel 1 (
    set "PYTHON_COMMAND=python"
    goto python_found
)

python3 --version >nul 2>nul
if not errorlevel 1 (
    set "PYTHON_COMMAND=python3"
    goto python_found
)

powershell -NoProfile -Command "Add-Type -AssemblyName PresentationFramework; [System.Windows.MessageBox]::Show('Python could not be started. Please open a command prompt in the app folder and run: python start.py', 'IDATA - Python required', 'OK', 'Error')" >nul
exit /b 1

:python_found
echo Using %PYTHON_COMMAND%.

echo Stopping the existing IDATA service...
%PYTHON_COMMAND% stop.py
if errorlevel 1 (
    echo.
    echo Failed to stop the existing service.
    pause
    exit /b 1
)

echo.
echo Starting IDATA...
start "IDATA" %PYTHON_COMMAND% start.py

if errorlevel 1 (
    echo.
    echo Failed to start IDATA.
    pause
    exit /b 1
)

echo Waiting for IDATA to become available...
powershell -NoProfile -Command "$deadline = (Get-Date).AddSeconds(30); do { try { $response = Invoke-WebRequest -UseBasicParsing -Uri 'http://localhost:54321' -TimeoutSec 1; if ($response.StatusCode -eq 200) { Start-Process 'http://localhost:54321'; exit 0 } } catch {}; Start-Sleep -Milliseconds 500 } while ((Get-Date) -lt $deadline); exit 1"

if errorlevel 1 (
    echo.
    echo IDATA did not become available at http://localhost:54321.
    pause
    exit /b 1
)

endlocal
