#!/bin/bash

set -e

OS_TYPE="Unknown"
GetOSType() {
    uNames=`uname -s`
    osName=${uNames: 0: 4}
    if [ "$osName" == "Darw" ] # Darwin
    then
        OS_TYPE="Darwin"
    elif [ "$osName" == "Linu" ] # Linux
    then
        OS_TYPE="Linux"
    elif [ "$osName" == "MING" ] # MINGW, windows, git-bash
    then
        OS_TYPE="Windows"
    else
        OS_TYPE="Unknown"
    fi
}
GetOSType

function find_windows_msys2_root() {
  for d in /c/msys64 /d/msys64 /e/msys64; do
    if [ -x "$d/usr/bin/bash.exe" ]; then
      echo "$d"
      return 0
    fi
  done
  return 1
}

function install_windows_msys2() {

    MSYS_ROOT=$(find_windows_msys2_root || true)

    if [ -n "$MSYS_ROOT" ]; then
      echo "MSYS2 found at $MSYS_ROOT, skip winget install"
    else
      echo "Installing MSYS2 via winget..."
      winget install --id MSYS2.MSYS2 -e --location C:\msys64
      MSYS_ROOT="/c/msys64"
    fi

    "$MSYS_ROOT/usr/bin/bash.exe" -lc "pacman -Syu --noconfirm || true"
    "$MSYS_ROOT/usr/bin/bash.exe" -lc "pacman -Syu --noconfirm"
    "$MSYS_ROOT/usr/bin/bash.exe" -lc \
      "pacman -S --noconfirm base-devel mingw-w64-x86_64-toolchain mingw-w64-x86_64-cmake"

    MSYS_ROOT_WIN=$(cygpath -w "$MSYS_ROOT")
    powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "\
      \$bin='${MSYS_ROOT_WIN}\\mingw64\\bin'; \
      \$p=[Environment]::GetEnvironmentVariable('Path','User'); \
      if ([string]::IsNullOrEmpty(\$p)) { \
        [Environment]::SetEnvironmentVariable('Path', \$bin,'User') \
      } elseif (\$p -notlike \"*\$bin*\") { \
        [Environment]::SetEnvironmentVariable('Path', (\$p + ';' + \$bin),'User') \
      }"
}


function install_dev_tools_windows() {
    install_windows_msys2
    echo "windows develop tools installed"
}

function install_dev_tools_darwin() {

    echo "windows develop tools installed"
}

function install_dev_tools_linux() {

    echo "windows develop tools installed"
}

function run_install() {
        if [ "${OS_TYPE}" == "Windows" ]
        then
            install_dev_tools_windows
        elif [ "${OS_TYPE}" == "Darwin" ]
        then
            install_dev_tools_darwin
        elif [ "${OS_TYPE}" == "Linux" ]
        then
          install_dev_tools_linux
        else
            echo "no supported os"
        fi
}

run_install