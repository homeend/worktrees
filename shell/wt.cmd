@echo off
rem wt shell integration for cmd.exe: make Enter in the `wt` TUI cd this
rem console into the selected worktree.
rem
rem Install: add the bin\ directory to PATH. The build names the real binary
rem wt.bin.exe on purpose — with no wt.exe around, typing `wt` resolves to
rem this wt.cmd, which launches the sibling wt.bin.exe and cd's afterwards.
rem
rem No setlocal here on purpose: the final cd must apply to the calling
rem cmd session (a child process cannot change its parent's directory).
rem Prefer the wt.bin.exe sitting next to this file (the materialized bin/
rem layout); fall back to PATH lookup otherwise.
set "WT_EXE=%~dp0wt.bin.exe"
if not exist "%WT_EXE%" set "WT_EXE=wt.bin.exe"
set "WT_CD_TMP=%TEMP%\wt-cd-%RANDOM%%RANDOM%.tmp"
"%WT_EXE%" --cd-file "%WT_CD_TMP%" %*
set "WT_CD_EXIT=%ERRORLEVEL%"
if not exist "%WT_CD_TMP%" goto wt_cd_done
set "WT_CD_DIR="
set /p WT_CD_DIR=<"%WT_CD_TMP%"
del "%WT_CD_TMP%" >nul 2>&1
if not defined WT_CD_DIR goto wt_cd_done
cd /d "%WT_CD_DIR%"
:wt_cd_done
set "WT_EXE="
set "WT_CD_TMP="
set "WT_CD_DIR="
exit /b %WT_CD_EXIT%
