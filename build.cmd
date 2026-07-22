@echo off
rem build.cmd — build the wt.exe binary into .\bin and materialize the shell
rem cd-wrappers next to it (override the output directory with BIN_DIR).
setlocal

cd /d "%~dp0"

if "%BIN_DIR%"=="" set "BIN_DIR=bin"

if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"

rem The binary is deliberately NOT named wt.exe: cmd.exe prefers .exe over
rem .cmd within a directory, so a wt.exe would shadow the wt.cmd wrapper and
rem typing `wt` could never cd. As wt.bin.exe the bare name `wt` resolves to
rem wt.cmd, which launches the sibling wt.bin.exe.
go build -trimpath -o "%BIN_DIR%\wt.bin.exe" .
if errorlevel 1 (
    echo Build failed.
    exit /b 1
)

copy /Y shell\wt.cmd "%BIN_DIR%\wt.cmd" >nul
copy /Y shell\wt.sh "%BIN_DIR%\wt.sh" >nul
rem Drop artifacts of older layouts that would shadow the wrapper.
if exist "%BIN_DIR%\wt.exe" del "%BIN_DIR%\wt.exe"
if exist "%BIN_DIR%\w.cmd" del "%BIN_DIR%\w.cmd"

echo Built %BIN_DIR%\wt.bin.exe (+ wt.cmd, wt.sh wrappers)
endlocal
