@echo off
rem build-worktrees.cmd — build the full-name installer worktrees.exe into
rem .\bin. Run it with a target directory — `worktrees <dir>` copies the wt
rem entry points (wt.bin.exe + wt.cmd) into that directory, same as a
rem `go install` binary does. Override the output directory with BIN_DIR.
setlocal

cd /d "%~dp0"

if "%BIN_DIR%"=="" set "BIN_DIR=bin"

if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
go build -trimpath -o "%BIN_DIR%\worktrees.exe" .
if errorlevel 1 (
    echo Build failed.
    exit /b 1
)

echo Built %BIN_DIR%\worktrees.exe (installer: worktrees ^<dir^>)
endlocal
