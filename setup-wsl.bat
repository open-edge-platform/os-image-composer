@echo off
SETLOCAL DisableDelayedExpansion

:: ==========================================
:: SETTINGS & VARIABLES
:: ==========================================
SET "REPO_URL=https://github.com/yockgen/madani-os-builder.git"
SET "REPO_DIR=madani-os-builder"
SET "GO_VERSION=go1.25.5"
SET "DISTRO=Ubuntu-24.04"
:: ==========================================

echo [1/5] Checking %DISTRO% status...

:: 1. Check if Distro is installed (silencing the ERROR_ALREADY_EXISTS)
wsl --list --quiet | findstr /C:"%DISTRO%" >nul
if %errorlevel% neq 0 (
    echo [!] %DISTRO% not found. Installing now...
    wsl --install -d %DISTRO% --no-launch
    echo.
    echo ---------------------------------------------------------
    echo  ACTION REQUIRED:
    echo  1. A new window has opened for Ubuntu setup.
    echo  2. Create your Username and Password in that window.
    echo  3. ONCE YOU SEE THE COMMAND PROMPT, return to THIS window.
    echo ---------------------------------------------------------
    pause
)

echo [2/5] Preparing System Directories...
wsl -d %DISTRO% -u root mkdir -p /data
wsl -d %DISTRO% -u root chmod 777 /data

echo [3/5] Installing Dependencies and Go...
wsl -d %DISTRO% -u root apt-get update
wsl -d %DISTRO% -u root apt-get install -y wget tar git

:: Install Go
wsl -d %DISTRO% -u root bash -c "if [ ! -d '/usr/local/go' ]; then wget -q https://go.dev/dl/%GO_VERSION%.linux-amd64.tar.gz -O /tmp/go.tar.gz && tar -C /usr/local -xzf /tmp/go.tar.gz && rm /tmp/go.tar.gz; fi"

:: FIXED PATH COMMAND: Using printf with hex \x24 for '$' prevents ALL Windows expansion
wsl -d %DISTRO% -u root bash -c "printf 'export PATH=\x24PATH:/usr/local/go/bin\n' > /etc/profile.d/golang.sh"

echo [4/5] Cloning Repository to /data...
wsl -d %DISTRO% -u root bash -c "if [ ! -d '/data/%REPO_DIR%' ]; then cd /data && git clone %REPO_URL%; fi"

echo [4.1/5] Setting proper permissions for everyone to access the repository...
:: Set ownership to allow all users to read/write
wsl -d %DISTRO% -u root chown -R nobody:nogroup /data/%REPO_DIR%
:: Set full read/write permissions for owner, group, and others
wsl -d %DISTRO% -u root chmod -R 777 /data/%REPO_DIR%
:: Ensure git directory is also accessible
wsl -d %DISTRO% -u root chmod -R 777 /data/%REPO_DIR%/.git

echo [5/5] Final Verification...
echo ----------------------------------------------
:: Explicitly calling the full path for verification to bypass profile errors if they existed
wsl -d %DISTRO% bash -c "export PATH=$PATH:/usr/local/go/bin; echo -n 'Go Version: ' && go version; echo -n 'Git Version: ' && git --version"
echo ----------------------------------------------

echo.
echo [OK] Everything is ready! 
echo [NOTE] If you still see a syntax error, it's from the OLD corrupted file.
echo [NOTE] Restarting the terminal will fix it.
pause
wsl -d %DISTRO%
