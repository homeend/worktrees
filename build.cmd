@echo off
rem build.cmd — build the wt binary into .\bin (override with BIN_DIR).
setlocal

cd /d "%~dp0"

if "%BIN_DIR%"=="" set "BIN_DIR=bin"
set "OUT=%BIN_DIR%\wt.exe"

if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
go build -trimpath -o "%OUT%" .
if errorlevel 1 (
    echo Build failed.
    exit /b 1
)

echo Built %OUT%
endlocal
