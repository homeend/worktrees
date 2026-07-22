@echo off
rem build-worktrees.cmd — build the full-name, self-installing worktrees.exe
rem into .\bin. Copy it anywhere and run it once — it materializes the wt
rem entry points (wt.bin.exe + wt.cmd) next to itself, same as a `go install`
rem binary does. Override the output directory with BIN_DIR.
setlocal

cd /d "%~dp0"

if "%BIN_DIR%"=="" set "BIN_DIR=bin"

if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
go build -trimpath -o "%BIN_DIR%\worktrees.exe" .
if errorlevel 1 (
    echo Build failed.
    exit /b 1
)

echo Built %BIN_DIR%\worktrees.exe (self-installing)
endlocal
