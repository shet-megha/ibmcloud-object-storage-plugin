@echo off
setlocal

REM Get the directory where this script is located
set SCRIPT_DIR=%~dp0

REM Change to the script directory
cd /d "%SCRIPT_DIR%"

REM Check if kubeconfig exists
if not exist "kubeconfig\config" (
    echo Error: kubeconfig\config file not found!
    echo Please place your kubeconfig file at: %SCRIPT_DIR%kubeconfig\config
    echo.
    pause
    exit /b 1
)

REM Set KUBECONFIG environment variable
set KUBECONFIG=%SCRIPT_DIR%kubeconfig\config

REM Run the kubectl-flex-to-csi tool
kubectl-flex-to-csi.exe

REM Keep terminal open
pause

@REM Made with Bob
