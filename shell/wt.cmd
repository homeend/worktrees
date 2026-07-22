@echo off
rem wt shell integration for cmd.exe: make Enter in the `wt` TUI cd this
rem console into the selected worktree.
rem
rem Install: place this wt.cmd in a PATH directory that is searched BEFORE
rem the directory holding wt.exe (cmd prefers .exe over .cmd within the same
rem directory), or define a doskey macro instead:
rem   doskey wt="T:\path\to\shell\wt.cmd" $*
rem wt.exe must be on PATH.
rem
rem No setlocal here on purpose: the final cd must apply to the calling
rem cmd session (a child process cannot change its parent's directory).
set "WT_CD_TMP=%TEMP%\wt-cd-%RANDOM%%RANDOM%.tmp"
wt.exe --cd-file "%WT_CD_TMP%" %*
set "WT_CD_EXIT=%ERRORLEVEL%"
if not exist "%WT_CD_TMP%" goto wt_cd_done
set "WT_CD_DIR="
set /p WT_CD_DIR=<"%WT_CD_TMP%"
del "%WT_CD_TMP%" >nul 2>&1
if not defined WT_CD_DIR goto wt_cd_done
cd /d "%WT_CD_DIR%"
:wt_cd_done
set "WT_CD_TMP="
set "WT_CD_DIR="
exit /b %WT_CD_EXIT%
