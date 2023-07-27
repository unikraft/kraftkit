#!/bin/sh
# shellcheck shell=sh

# SPDX-License-Identifier: BSD-3-Clause
# Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
# Licensed under the BSD-3-Clause License (the "License").
# You may not use this file expect in compliance with the License.

# File taken and modified from: https://github.com/rust-lang/rustup
# Original credits go to the Rust project and its contributors.

# It runs on Unix shells like {a,ba,da,k,z}sh. It uses the common `local`
# extension. Note: Most shells limit `local` to 1 var per line, contra bash.

if [ "$KSH_VERSION" = 'Version JM 93t+ 2010-03-05' ]; then
    # The version of ksh93 that ships with many illumos systems does not
    # support the "local" extension.  Print a message rather than fail in
    # subtle ways later on:
    echo 'Installation does not work with this ksh93; please try bash!' >&2
    exit 1
fi

# The following variables can be set to override the defaults:
PREFIX=${PREFIX:-/usr/local/bin}
INSTALL_TLS_CIPHERSUITES=${INSTALL_TLS_CIPHERSUITES:-}
INSTALL_CPUTYPE=${INSTALL_CPUTYPE:-}
INSTALL_SERVER=${INSTALL_SERVER:-https://get.kraftkit.sh}
INSTALL_TLS=${INSTALL_TLS:-y}
DEBUG=${DEBUG:-n}
NEED_TTY=${NEED_TTY:-y}

# Commands as variables to make them easier to override
GREP=${GREP:-grep}
CAT=${CAT:-cat}
SUDO=${SUDO:-sudo}
AWK=${AWK:-awk}
HEAD=${HEAD:-head}
TAIL=${TAIL:-tail}
UNAME=${UNAME:-uname}
LDD=${LDD:-ldd}
SYSCTL=${SYSCTL:-sysctl}
CURL=${CURL:-curl}
WGET=${WGET:-wget}
YUM=${YUM:-yum}
APT=${APT:-apt-get}
APK=${APK:-apk}
MAKEPKG=${MAKEPKG:-makepkg}
GIT=${GIT:-git}
TAR=${TAR:-tar}
INSTALL=${INSTALL:-install}
RM=${RM:-rm}
CUT=${CUT:-cut}
SW_VERS=${SW_VERS:-sw_vers}

set -u

trap cleanup_int HUP
trap cleanup_int INT
trap cleanup_int QUIT
trap cleanup_int ABRT
trap cleanup_int ALRM
trap cleanup_int TERM

# usage prints the usage message to stderr
usage() {
    $CAT 1>&2 <<EOF
The installer for kraftkit.
Build and use highly customized and ultra-lightweight unikernels.

Documentation:    https://kraftkit.sh/
Issues & support: https://github.com/unikraft/kraftkit/issues

USAGE:
    install [FLAGS] [OPTIONS]
FLAGS:
    -d, --debug             Enable debug output
    -y                      Disable confirmation prompt
    -h, --help              Prints help information
    -v, --version           Prints version information
EOF
}

# Helper variables
_NO_ANS="^n$|^N$|^no$|^No$|^nO$"
_YES_ANS="^y$|^Y$|^yes$|^Yes$|^yEs$|^yeS$|^YEs$|^yES$|^YES$"
_NO_ANS_DEFAULT="^n$|^N$|^no$|^No$|^nO$|^\n$|^$"
_YES_ANS_DEFAULT="^y$|^Y$|^yes$|^Yes$|^yEs$|^yeS$|^YEs$|^yES$|^YES$|^\n$|^$"
_RETVAL=""
_CLEANUP_ARCHIVE=""
_CLEANUP_BINARY=""
_CLEANUP_VERSION=""

# say prints a message to stdout prefixed with "kraftkit: "
# $1: message to print
# Returns:
say() {
    printf '%s\n' "$1"
}

say_debug() {
    if [ "$DEBUG" = "y" ]; then
        printf '%s\n' "$1"
    fi
}

# err prints a message to stderr prefixed with "kraftkit: " and exits
# $1: error to print
# Returns:
# Code: 1
err() {
    say "$1" >&2
    exit 1
}

# need_cmd checks if a command is available and exits if it is not.
# $1: command to check
# Returns:
need_cmd() {
    say_debug "Checking for command $1"
    if ! check_cmd "$1"; then
        err "need '$1' (command not found)"
    fi
}

# check_cmd checks if the path to a command exists.
# $1: command to check
# Returns:
# Code: 1 if the command exists, 0 otherwise
check_cmd() {
    command -v "$1" > /dev/null 2>&1
}

# assert_nz checks if a variable is not empty and exits if it is.
# $1: variable to check
# $2: error message to print
assert_nz() {
    if [ -z "$1" ]; then err "assert_nz $2"; fi
}

# ensure runs a command that should never fail. If the command fails execution
# will immediately terminate with an error showing the failing command.
# $@: command to run
# Returns:
ensure() {
    if ! "$@"; then err "command failed: $*"; fi
}

# cleanup_int is the interrupt handler for the installer.
# Returns:
# Code: 1
cleanup_int() {
    say ""
    cleanup
    err "user interrupt: exiting..."
}

# cleanup removes any temporary files created during installation.
# Returns:
# Code: 1 if the command exists, 0 otherwise
cleanup() {
    if [ -n "$_CLEANUP_ARCHIVE" ] || \
        [ -n "$_CLEANUP_BINARY" ] || \
        [ -n "$_CLEANUP_VERSION" ]; then
        do_cmd "$RM -f $_CLEANUP_BINARY $_CLEANUP_ARCHIVE $_CLEANUP_VERSION"
    fi
}

# get_user_response asks the user a question and returns the answer in '_RETVAL'
# $1: question
# $2: default answer
# Returns:
# _RETVAL: answer
get_user_response() {
    _gur_question="$1"
    _gur_default="$2"
    _gur_read_answer=""

    # Ask the question
    printf "%s" "$_gur_question"

    # Check if we have a tty and read from it if we do
    if [ "$NEED_TTY" = "y" ]; then
        read -r _gur_read_answer < /dev/tty
    fi

    # If we the answer is empty, or we don't have a tty, use the default
    if [ -z "$_gur_read_answer" ]; then
        _gur_read_answer="$_gur_default"
    fi

    # Return the answer
    _RETVAL="$_gur_read_answer"
}

# do_cmd runs a command and asks the user if they want to retry with root
# if it fails
# $@: command
# Returns:
# _cmd_retval: return value of the command
# Code: _cmd_retval: return value of the command on fail
do_cmd() {
    # Print the command out to stderr for users to see
    echo "+" "$@" > /dev/stderr

    # Run the command if the command exists
    if ! sh -c "$@"; then
        # Take the return value of the command
        _cmd_retval="$?"

        # Inform the user that the command failed
        say "command failed: $*"

        # Ask the user if they want to retry with root
        get_user_response "do you want to retry with root? [y/N]: " "n"
        _answer="$_RETVAL"

        # Retry with root if the user wants to, otherwise exit
        # with the return value of the command
        if printf "%s" "$_answer" | "$GREP" -q -E "$_NO_ANS"; then
            exit "$_cmd_retval"
        elif printf "%s" "$_answer" | "$GREP" -q -E "$_YES_ANS"; then

            # Check if we have a tty and run with it if we do,
            # otherwise exit with an error
            # This is because sudo requires a tty to input the password
            if [ "$NEED_TTY" = "y" ]; then
                # shellcheck disable=SC2002
                $SUDO sh -c "$@" < /dev/tty
            else
                err "fatal: cannot retry with root without a tty."
            fi
        else
            err "error: choose either yes or no."
        fi
    fi
}

# check_proc checks if /proc is mounted by looking for /proc/self/exe for Linux
# Returns:
check_proc() {
    if ! test -L /proc/self/exe ; then
        err "fatal: unable to find /proc/self/exe. Is /proc mounted?"\
        "Installation cannot proceed without /proc."
    fi
}


# check_help_for prints the help for a command with a specific set of options
# $1: architecture/command to check
# $@: options to check
# Returns:
check_help_for() {
    _chf_cmd=""
    _chf_arg=""
    _chf_arch="$1"
    shift
    _chf_cmd="$1"
    shift

    _chf_category=""
    _chf_cmd_help_check='For all options use the manual or "--help all".'
    if "$_chf_cmd" --help | "$GREP" -q "$_chf_cmd_help_check"; then
      _chf_category="all"
    else
      _chf_category=""
    fi

    case "$_chf_arch" in

        *darwin*)
        if check_cmd "$SW_VERS"; then
            _chf_ver_arg="-productVersion"
            case $("$SW_VERS" "$_chf_ver_arg") in
                10.*)
                    # If we're running on macOS, older than 10.13, then we
                    # always fail to find these options to force fallback
                    _chf_ck_arg="$("$SW_VERS" "$_chf_ver_arg" | "$CUT" -d. -f2)"
                    if [ "$_chf_ck_arg" -lt 13 ]; then
                        # Older than 10.13
                        say "warning: Detected macOS platform older than 10.13"
                        return 1
                    fi
                    ;;
                11.*)
                    # We assume Big Sur will be OK for now
                    ;;
                *)
                    # Unknown product version, warn and continue
                    say "warning: Detected unknown macOS major version:"\
                    "$("$SW_VERS" "$_chf_ver_arg")"
                    say "warning TLS capabilities detection may fail"
                    ;;
            esac
        fi
        ;;

    esac

    for _chf_arg in "$@"; do
        "$_chf_cmd" --help "$_chf_category" | "$GREP" -q -- "$_chf_arg"
        _chf_cmd_res="$?"
        if [ "$_chf_cmd_res" != "0" ]; then
            return 1
        fi
    done

    true # not strictly needed
}

# Check if curl supports the --retry flag, then pass it to the curl invocation.
# Returns:
# _RETVAL: empty string if not supported, "--retry 3" if supported
check_curl_for_retry_support() {
    _ccr_retry_supported=""
    # "unspecified" is for arch, allows for possibility old OS
    # using macports, homebrew, etc.
    if check_help_for "notspecified" "curl" "--retry"; then
    _ccr_retry_supported="--retry 3"
    fi

    _RETVAL="$_ccr_retry_supported"
}

# check_os_release checks if the given OS is in the ID_LIKE or ID field of
# /etc/os-release.
# Returns:
# Code: 0 if the OS is found, 1 otherwise
check_os_release() {
    _cor_os="$1"
    _ccr_ck1=""
    _cor_ck2=""

    # False positive - it does not interpret it as awk code because of the macro
    # shellcheck disable=SC2016
    "$AWK" -F= '/^ID_LIKE/{print $2}' /etc/os-release | "$GREP" -q "$_cor_os"
    _cor_ck1=$?

    # False positive - it does not interpret it as awk code because of the macro
    # shellcheck disable=SC2016
    "$AWK" -F= '/^ID/{print $2}' /etc/os-release | "$GREP" -q "$_cor_os"
    _cor_ck2=$?

    say_debug "ID_LIKE: $_cor_ck1, ID: $_cor_ck2"

    [ "$_cor_ck1" = "0" ] || [ "$_cor_ck2" = "0" ]
    return $?
}

# is_host_amd64_elf returns true if the current platform is amd64.
# Returns 0 if true, 1 if false.
is_host_amd64_elf() {
    need_cmd "$HEAD"
    need_cmd "$TAIL"
    # ELF e_machine detection without dependencies beyond coreutils.
    # Two-byte field at offset 0x12 indicates the CPU,
    # but we're interested in it being 0x3E to indicate amd64, or not that.
    _iha_current_exe_machine=""
    _iha_current_exe_machine=$("$HEAD" -c 19 /proc/self/exe | "$TAIL" -c 1)
    [ "$_iha_current_exe_machine" = "$(printf '\076')" ]
}

# get_bitness returns the bitness of the current platform.
# Echoes 32 or 64.
get_bitness() {
    need_cmd "$HEAD"
    # Architecture detection without dependencies beyond coreutils.
    # ELF files start out "\x7fELF", and the following byte is
    #   0x01 for 32-bit and
    #   0x02 for 64-bit.
    # The printf builtin on some shells like dash only supports octal
    # escape sequences, so we use those.
    _gbt_current_exe_head=""
    _gbt_current_exe_head=$("$HEAD" -c 5 /proc/self/exe )
    if [ "$_gbt_current_exe_head" = "$(printf '\177ELF\001')" ]; then
        echo 32
    elif [ "$_gbt_current_exe_head" = "$(printf '\177ELF\002')" ]; then
        echo 64
    else
        err "fatal: unknown platform bitness"
    fi
}

# get_endianness returns the endianness of the current platform.
# Echoes the cputype with the suffix appended.
get_endianness() {
    _ged_cputype="$1"
    _ged_suffix_eb="$2"
    _ged_suffix_el="$3"

    # detect endianness without od/hexdump, like get_bitness() does.
    need_cmd "$HEAD"
    need_cmd "$TAIL"

    _ged_current_exe_endianness=""
    _ged_current_exe_endianness="$("$HEAD" -c 6 /proc/self/exe | "$TAIL" -c 1)"
    if [ "$_ged_current_exe_endianness" = "$(printf '\001')" ]; then
        echo "${_ged_cputype}${_ged_suffix_el}"
    elif [ "$_ged_current_exe_endianness" = "$(printf '\002')" ]; then
        echo "${_ged_cputype}${_ged_suffix_eb}"
    else
        err "unknown platform endianness"
    fi
}

# get_linux_types returns the ostype and clibtype if the platform is based
# on Linux.
# $1: the ostype to check
# $2: the clibtype to check
# Returns:
# _OSTYPE: the ostype to use
# _CLIBTYPE: the clibtype to use
get_linux_types() {
    _glt_ostype="$1"
    _glt_clibtype="$2"

    if [ "$_glt_ostype" = Linux ]; then
        # Check if the OS is Android
        if [ "$($UNAME -o)" = Android ]; then
            _OSTYPE="Android"
        fi
        # Check if the OS is Alpine (based on musl)
        if $LDD --version 2>&1 | "$GREP" -q 'musl'; then
            _CLIBTYPE="musl"
        fi
    fi
}

# get_darwin_types returns the ostype and clibtype if the platform is based
# on Darwin.
# $1: the ostype to check
# $2: the cputype to check
# Returns:
# _OSTYPE: the ostype to use
# _CPUTYPE: the cputype to use
get_darwin_types() {
    _gdt_ostype="$1"
    _gdt_cputype="$2"

    if [ "$_gdt_ostype" = Darwin ] && [ "$_gdt_cputype" = i386 ]; then
        # Darwin `uname -m` lies so we need to do further checks
        if $SYSCTL hw.optional.x86_64 | "$GREP" -q ': 1'; then
            _CPUTYPE="x86_64"
        fi
    fi
}

# get_sunos_types returns the ostype and clibtype if the platform is based
# on SunOS.
# $1: the ostype to check
# $2: the cputype to check
# Returns:
# _OSTYPE: the ostype to use
# _CPUTYPE: the cputype to use
get_sunos_types() {
    _gst_ostype="$1"
    _gst_cputype="$2"

    if [ "$_gst_ostype" = SunOS ]; then
        # Both Solaris and illumos presently announce as "SunOS" in "uname -s"
        # so use "uname -o" to disambiguate.  We use the full path to the
        # system uname in case the user has coreutils uname first in PATH,
        # which has historically sometimes printed the wrong value here.
        if [ "$(/usr/bin/uname -o)" = illumos ]; then
            _OSTYPE="illumos"
        fi

        # illumos systems have multi-arch userlands, and "uname -m" reports the
        # machine hardware name; e.g., "i86pc" on both 32- and 64-bit x86
        # systems.  Check for the native (widest) instruction set on the
        # running kernel:
        if [ "$_gst_cputype" = i86pc ]; then
            _CPUTYPE="$(isainfo -n)"
        fi
    fi
}

# resolve_os_type returns the ostype and clibtype for the current platform.
# $1: the ostype to check
# $2: the clibtype to check
# Returns:
# _OSTYPE: the resolved ostype
# _BITNESS: the bitness of the current platform
resolve_os_type() {
    _rot_ostype="$1"
    _rot_clibtype="$2"

    case "$_rot_ostype" in

        Android)
            _OSTYPE=linux-android
            ;;

        Linux)
            check_proc
            _OSTYPE=unknown-linux-$_rot_clibtype
            _BITNESS=$(get_bitness)
            ;;

        FreeBSD)
            _OSTYPE=unknown-freebsd
            ;;

        NetBSD)
            _OSTYPE=unknown-netbsd
            ;;

        DragonFly)
            _OSTYPE=unknown-dragonfly
            ;;

        Darwin)
            _OSTYPE=apple-darwin
            ;;

        illumos)
            _OSTYPE=unknown-illumos
            ;;

        MINGW* | MSYS* | CYGWIN* | Windows_NT)
            _OSTYPE=pc-windows-gnu
            ;;

        *)
            err "error: unrecognized OS type: $_rot_ostype"
            ;;

    esac
}

# resolve_cpu_type returns the cputype for the current platform.
# $1: the cputype to check
# $2: the bitness of the current platform
# Returns:
# _CPUTYPE: the resolved cputype
# _OSTYPE: the resolved ostype
resolve_cpu_type() {
    _rct_cputype="$1"
    _rct_bitness="$2"

    case "$_rct_cputype" in

        i386 | i486 | i686 | i786 | x86)
            _CPUTYPE=i686
            ;;

        xscale | arm)
            _CPUTYPE=arm
            if [ "$_OSTYPE" = "linux-android" ]; then
                _OSTYPE=linux-androideabi
            fi
            ;;

        armv6l)
            _CPUTYPE=arm
            if [ "$_OSTYPE" = "linux-android" ]; then
                _OSTYPE=linux-androideabi
            else
                _OSTYPE="${_OSTYPE}eabihf"
            fi
            ;;

        armv7l | armv8l)
            _CPUTYPE=armv7
            if [ "$_OSTYPE" = "linux-android" ]; then
                _OSTYPE=linux-androideabi
            else
                _OSTYPE="${_OSTYPE}eabihf"
            fi
            ;;

        aarch64 | arm64)
            _CPUTYPE=aarch64
            ;;

        x86_64 | x86-64 | x64 | amd64)
            _CPUTYPE=x86_64
            ;;

        mips)
            _CPUTYPE=$(get_endianness mips '' el)
            ;;

        mips64)
            if [ "$_rct_bitness" -eq 64 ]; then
                # only n64 ABI is supported for now
                _OSTYPE="${_OSTYPE}abi64"
                _CPUTYPE=$(get_endianness mips64 '' el)
            fi
            ;;

        ppc)
            _CPUTYPE=powerpc
            ;;

        ppc64)
            _CPUTYPE=powerpc64
            ;;

        ppc64le)
            _CPUTYPE=powerpc64le
            ;;

        s390x)
            _CPUTYPE=s390x
            ;;
        riscv64)
            _CPUTYPE=riscv64gc
            ;;
        *)
            err "error: unknown CPU type: $_rct_cputype"

    esac
}

# resolve_cpu_unknown resolves the cputype for the current platform if it is
# detected as unknown and 32 bit.
# $1: the bitness of the current platform
# Returns:
# _CPUTYPE: the resolved cputype
# _OSTYPE: the resolved ostype
resolve_cpu_unknown() {
    _rcu_bitness="$1"
    _rcu_unknown="unknown-linux-gnu"

    if [ "${_OSTYPE}" = "$_rcu_unknown" ] && [ "${_rcu_bitness}" = "32" ]; then
        case $_CPUTYPE in
            x86_64)
                if [ -n "${INSTALL_CPUTYPE:-}" ]; then
                    _CPUTYPE="$INSTALL_CPUTYPE"
                else {
                    # 32-bit executable for amd64 = x32
                    if is_host_amd64_elf; then {
                         exit 1
                    }; else
                        _CPUTYPE=i686
                    fi
                }; fi
                ;;
            mips64)
                _CPUTYPE=$(get_endianness mips '' el)
                ;;
            powerpc64)
                _CPUTYPE=powerpc
                ;;
            aarch64)
                _CPUTYPE=armv7
                if [ "$_OSTYPE" = "linux-android" ]; then
                    _OSTYPE=linux-androideabi
                else
                    _OSTYPE="${_OSTYPE}eabihf"
                fi
                ;;
            riscv64gc)
                err "error: riscv64 with 32-bit userland unsupported"
                ;;
        esac
    fi
}

# resolve_cpu_unknown_arm resolves the cputype for the current platform if it is
# detected as unknown and arm.
# $1: the ostype to check
# $2: the cputype to check
# Returns:
# _OSTYPE: the resolved cputype
resolve_cpu_unknown_arm() {
    _rca_ostype="$1"
    _rca_cputype="$2"
    _rca_unk_lin="unknown-linux-gnueabihf"

    if [ "$_rca_ostype" = "$_rca_unk_lin" ] && [ "$_rca_cputype" = armv7 ]; then
        if ensure "$GREP" '^Features' /proc/cpuinfo | "$GREP" -q -v neon; then
            # At least one processor does not have NEON.
            _CPUTYPE=arm
        fi
    fi
}

# get_architecture returns the architecture of the current platform.
# Returns:
# _RETVAL: the os-cpu of the current platform.
get_architecture() {
    # Run basic checks to get the os and architecture
    _OSTYPE="$($UNAME -s)"
    _CPUTYPE="$($UNAME -m)"
    _CLIBTYPE="gnu"
    _BITNESS=""
    _ARCH=""

    # Check if the OS is Linux based
    get_linux_types "$_OSTYPE" "$_CLIBTYPE"

    # Check if the OS is Mac based
    get_darwin_types "$_OSTYPE" "$_CPUTYPE"

    # Check if the OS is Solaris or illumos based
    get_sunos_types "$_OSTYPE" "$_CPUTYPE"

    say_debug "Gotten OS: $_OSTYPE, CPU: $_CPUTYPE, LIBC: $_CLIBTYPE"

    # Check the OS variable and construct the first part of the return string
    resolve_os_type "$_OSTYPE" "$_CLIBTYPE"

    # Check the CPU variable and construct the second part of the return string
    resolve_cpu_type "$_CPUTYPE" "$_BITNESS"

    # Check the CPU/OS variables if 64-bit linux with 32-bit userland
    resolve_cpu_unknown "$_BITNESS"

    # Check the CPU/OS variables if unknown arm
    resolve_cpu_unknown_arm "$_OSTYPE" "$_CPUTYPE"

    _ARCH="${_CPUTYPE}-${_OSTYPE}"

    _RETVAL="$_ARCH"
}

# Return cipher suite string specified by user, otherwise return strong
# TLS 1.2-1.3 cipher suites if support by local tools is detected. Detection
# currently supports these curl backends: GnuTLS and OpenSSL (possibly also
# LibreSSL and BoringSSL).
# Returns:
# _RETVAL: cipher suite string. Can be empty.
get_ciphersuites_for_curl() {
    if [ -n "${INSTALL_TLS_CIPHERSUITES-}" ]; then
        # user specified custom cipher suites
        # assume they know what they're doing
        say_debug "Using user specified cipher suites"
        _RETVAL="$INSTALL_TLS_CIPHERSUITES"
        return
    fi

    _gcf_openssl_syntax="no"
    _gcf_gnutls_syntax="no"
    _gcf_backend_supported="yes"
    if "$CURL" -V | "$GREP" -q ' OpenSSL/'; then
        _gcf_openssl_syntax="yes"
    elif "$CURL" -V | "$GREP" -iq ' LibreSSL/'; then
        _gcf_openssl_syntax="yes"
    elif "$CURL" -V | "$GREP" -iq ' BoringSSL/'; then
        _gcf_openssl_syntax="yes"
    elif "$CURL" -V | "$GREP" -iq ' GnuTLS/'; then
        _gcf_gnutls_syntax="yes"
    else
        _gcf_backend_supported="no"
    fi

    _gcf_args_supported="no"
    if [ "$_gcf_backend_supported" = "yes" ]; then
        if check_help_for "notspecified" "$CURL" "--tlsv1.2" "--ciphers" \
            "--proto"; then
            _gcf_args_supported="yes"
        fi
    fi

    _gcf_cs=""
    if [ "$_gcf_args_supported" = "yes" ]; then
        if [ "$_gcf_openssl_syntax" = "yes" ]; then
            say_debug "Using OpenSSL syntax for ciphersuites"
            _gcf_cs=$(get_strong_ciphersuites_for "openssl")
        elif [ "$_gcf_gnutls_syntax" = "yes" ]; then
            say_debug "Using GnuTLS syntax for ciphersuites"
            _gcf_cs=$(get_strong_ciphersuites_for "gnutls")
        fi
    fi

    _RETVAL="$_gcf_cs"
}

# Return cipher suite string specified by user, otherwise return strong
# TLS 1.2-1.3 cipher suites if support by local tools is detected. Detection
# currently supports these wget backends: GnuTLS and OpenSSL (possibly also
# LibreSSL and BoringSSL).
# Returns:
# _RETVAL: cipher suite string. Can be empty.
get_ciphersuites_for_wget() {
    if [ -n "${INSTALL_TLS_CIPHERSUITES-}" ]; then
        # User specified custom cipher suites, assume they know what
        # they're doing
        say_debug "Using user specified cipher suites"
        _RETVAL="$INSTALL_TLS_CIPHERSUITES"
        return
    fi

    _gcw_cs=""
    if "$WGET" -V | "$GREP" -q '\-DHAVE_LIBSSL'; then
        # "unspecified" is for arch, allows for possible old OSes
        if check_help_for "notspecified" "$WGET" "TLSv1_2" "--ciphers" \
            "--https-only" "--secure-protocol"; then
            say_debug "Using strong cipher suites for wget from OpenSSL"
            _gcw_cs=$(get_strong_ciphersuites_for "openssl")
        fi
    elif "$WGET" -V | "$GREP" -q '\-DHAVE_LIBGNUTLS'; then
        # "unspecified" is for arch, allows for possible old OSes
        if check_help_for "notspecified" "$WGET" "TLSv1_2" "--ciphers" \
            "--https-only" "--secure-protocol"; then
            say_debug "Using strong cipher suites for wget from GnuTLS"
            _gcw_cs=$(get_strong_ciphersuites_for "gnutls")
        fi
    fi

    _RETVAL="$_gcw_cs"
}

# Strong TLS 1.2-1.3 cipher suites in OpenSSL or GnuTLS syntax. TLS 1.2
# excludes non-ECDHE and non-AEAD cipher suites. DHE is excluded due to bad
# DH params often found on servers (see RFC 7919). Sequence matches or is
# similar to Firefox 68 ESR with weak cipher suites disabled via about:config.
# $1: "openssl" or "gnutls"
# Returns:
# echo: cipher suite string
get_strong_ciphersuites_for() {
    _gsc_cs="$1"

    if [ "$_gsc_cs" = "openssl" ]; then
        # OpenSSL is forgiving of unknown values, no problems with TLS 1.3
        # values on versions that don't support it yet.
        printf "%s%s%s%s%s\n"                                           \
        "TLS_AES_128_GCM_SHA256:TLS_CHACHA20_POLY1305_SHA256:"          \
        "TLS_AES_256_GCM_SHA384:ECDHE-ECDSA-AES128-GCM-SHA256:"         \
        "ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-CHACHA20-POLY1305:"    \
        "ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES256-GCM-SHA384:"    \
        "ECDHE-RSA-AES256-GCM-SHA384"
    elif [ "$_gsc_cs" = "gnutls" ]; then
        # GnuTLS isn't forgiving of unknown values, so this may require a
        # GnuTLS version that supports TLS 1.3 even if wget doesn't.
        # Begin with SECURE128 (and higher) then remove/add to build cipher
        # suites. Produces same 9 cipher suites as OpenSSL but in slightly
        # different order.
        printf "%s%s%s%s\n"                                         \
        "SECURE128:-VERS-SSL3.0:-VERS-TLS1.0:-VERS-TLS1.1:"         \
        "-VERS-DTLS-ALL:-CIPHER-ALL:-MAC-ALL:-KX-ALL:+AEAD:"        \
        "+ECDHE-ECDSA:+ECDHE-RSA:+AES-128-GCM:+CHACHA20-POLY1305:"  \
        "+AES-256-GCM"
    fi
}

# download_using_curl downloads the given file using curl
# $1: the url to download
# $2: the archive to download to
# $3: the architecture of the download
# Returns:
# _RETVAL: return code of curl
# Code: 0 on success, 1 on error
download_using_curl() {
    _duc_url="$1"
    _duc_archive="$2"
    _duc_arch="$3"
    _duc_retry=""
    _duc_ciphersuites=""
    _duc_err=""

    check_curl_for_retry_support
    _duc_retry="$_RETVAL"

    if [ "$INSTALL_TLS" = "y" ]; then
        get_ciphersuites_for_curl
        _duc_ciphersuites="$_RETVAL"
        if [ -n "$_duc_ciphersuites" ]; then
            say_debug "Enforcing strong cipher suites for TLS"

            # shellcheck disable=SC2086
            _duc_err=$("$CURL" $_duc_retry --proto '=https' --tlsv1.2       \
                --ciphers "$_duc_ciphersuites" --silent --show-error --fail \
                --location "$_duc_url" --output "$_duc_archive" 2>&1)
            _RETVAL=$?
        else
            say "warning: Not enforcing strong cipher suites for TLS"
            if ! check_help_for "$_duc_arch" "$CURL" --proto --tlsv1.2; then
                say "warning: Not enforcing TLS v1.2; less secure"

                # shellcheck disable=SC2086
                _duc_err=$("$CURL" $_duc_retry --silent --show-error \
                    --fail --location "$_duc_url" --output "$_duc_archive" 2>&1)
                _RETVAL=$?
            else

                # shellcheck disable=SC2086
                _duc_err=$("$CURL" $_duc_retry --proto '=https' --tlsv1.2   \
                    --silent --show-error --fail --location "$_duc_url"     \
                    --output "$_duc_archive" 2>&1)
                _RETVAL=$?
            fi
        fi
    else
        say "warning: Not enforcing cipher suites for TLS; less secure"

        # shellcheck disable=SC2086
        _duc_err=$("$CURL" $_duc_retry --silent \
            --show-error --fail --location "$_duc_url" \
            --output "$_duc_archive" 2>&1)
        _RETVAL=$?
    fi

    if [ -n "$_duc_err" ]; then
        echo "$_duc_err" >&2
        if echo "$_duc_err" | $GREP -q 404$; then
            _duc_msg=""
            _duc_msg=$(printf "error: %s%s"                         \
                "installer for platform '$_duc_arch' not found, "   \
                "this may be unsupported"                           \
            )
            err "$_duc_msg"
        fi
    fi
}

# download_using_wget downloads the given file using wget
# $1: the url to download
# $2: the archive to download to
# $3: the architecture of the download
# Returns:
# _RETVAL: return code of wget
# Code: 0 on success, 1 on error
download_using_wget() {
    _duw_url="$1"
    _duw_archive="$2"
    _duw_arch="$3"
    _duw_err=""
    _duw_ciphersuites=""
    _duw_chk_busybox="$("$WGET" -V 2>&1|"$HEAD" -2|"$TAIL" -1|"$CUT" -f1 -d" ")"

    if [ "$INSTALL_TLS" = "y" ]; then
        if [ "$_duw_chk_busybox" = "BusyBox" ]; then
            say "warning: using the BusyBox version of $WGET."\
            "Not enforcing strong cipher suites for TLS or TLS v1.2"
            _duw_err=$("$WGET" "$_duw_url" -O "$_duw_archive" 2>&1)
            _RETVAL=$?
        else
            get_ciphersuites_for_wget
            _duw_ciphersuites="$_RETVAL"
            if [ -n "$_duw_ciphersuites" ]; then
                _duw_err=$("$WGET" --https-only                 \
                    --secure-protocol=TLSv1_2                   \
                    --ciphers "$_duw_ciphersuites" "$_duw_url"  \
                    -O "$_duw_archive" 2>&1)
                _RETVAL=$?
            else
                say "warning: Not enforcing strong suites for TLS; less secure"
                _help_chk=$(check_help_for "$_duw_arch" "$WGET" \
                            --https-only --secure-protocol)
                if ! "$_help_chk"; then
                    say "warning: Not enforcing TLS v1.2, this is less secure"
                    _duw_err=$("$WGET" "$_duw_url" -O "$_duw_archive" 2>&1)
                    _RETVAL=$?
                else
                    _duw_err=$("$WGET" --https-only             \
                        --secure-protocol=TLSv1_2 "$_duw_url"   \
                        -O "$_duw_archive" 2>&1)
                    _RETVAL=$?
                fi
            fi
        fi
    else
        say "warning: Not enforcing  cipher suites for TLS; less secure"
        _duw_err=$("$WGET" "$_duw_url" -O "$_duw_archive" 2>&1)
        _RETVAL=$?
    fi


    if [ -n "$_duw_err" ]; then
        echo "$_duw_err" >&2
        if echo "$_duw_err" | $GREP -q ' 404 Not Found$'; then
            _duc_msg=""
            _duc_msg=$(printf "error: %s%s"                         \
                "installer for platform '$_duw_arch' not found, "   \
                "this may be unsupported"                           \
            )
            err "$_duc_msg"
        fi
    fi
}

# downloader wraps curl or wget. Try curl first, if not installed,
# use wget instead.
# $1: the url to download
# $2: the archive to download to
# $3: the architecture of the download
# Returns:
# _RETVAL: return code of curl or wget
# Code: 0 on success, 1 on error
downloader() {
    _dow_url="$1"
    _dow_archive="$2"
    _dow_arch="$3"
    _dow_dld=""

    if check_cmd "$CURL"; then
        _dow_dld="$CURL"
    elif check_cmd "$WGET"; then
        _dow_dld="$WGET"
    else
        # To be used in error message of need_cmd
        _dow_dld="$CURL or $WGET"
    fi

    if [ "$_dow_url" = --check ]; then
        need_cmd "$_dow_dld"
    elif [ "$_dow_dld" = "$CURL" ]; then
        say_debug "Using $CURL to download"
        download_using_curl "$_dow_url" "$_dow_archive" "$_dow_arch"
        return $_RETVAL
    elif [ "$_dow_dld" = "$WGET" ]; then
        say_debug "Using $WGET to download"
        download_using_wget "$_dow_url" "$_dow_archive" "$_dow_arch"
        return $_RETVAL
    else
        # Should not reach this
        err "fatal: unknown downloader"
    fi
}

# install_linux_gnu installs the kraftkit package for standard linux
# distributions that use glibc.
# Returns:
# Code: 0 on success, 1 on error
install_linux_gnu() {
    need_cmd "$AWK"
    need_cmd "$GREP"

    if check_os_release "rhel"; then
        need_cmd "$YUM"
        _ilg_rpm_path=$(printf "%s%s%s%s%s"         \
            "[kraftkit]\n"                          \
            "name=Kraftkit Repo\n"                  \
            "baseurl=https://rpm.pkg.kraftkit.sh\n" \
            "enabled=1\n"                           \
            "gpgcheck=0\n"                          \
        )

        do_cmd "printf $_ilg_rpm_path | tee /etc/yum.repos.d/kraftkit.repo"
        do_cmd "$YUM update"
        do_cmd "$YUM install kraftkit"
    elif check_os_release "debian"; then
        need_cmd "$APT"
        _ilg_deb_path="deb [trusted=yes] https://deb.pkg.kraftkit.sh /"

        _ilg_deb_cmd=$(printf "%s%s"                    \
            "echo ${_ilg_deb_path} | "                  \
            "tee /etc/apt/sources.list.d/kraftkit.list" \
        )
        do_cmd "$_ilg_deb_cmd"
        do_cmd "$APT --allow-unauthenticated update"
        do_cmd "$APT install -y kraftkit"
    elif check_os_release "arch"; then
        need_cmd "$GIT"
        need_cmd "$MAKEPKG"
        need_cmd "$RM"

        do_cmd "$GIT clone https://aur.archlinux.org/kraftkit-bin.git /tmp/kraftkit-bin"
        do_cmd "$MAKEPKG -si /tmp/kraftkit-bin"
        $RM -rf /tmp/kraftkit-bin
    else
        _ilg_msg=$(printf "error: %s%s%s"                               \
            "Unsupported Linux distribution. "                          \
            "Try downloading the tar.gz file from "                     \
            "https://github/unikraft/kraftkit or switch to manual mode" \
        )
        err "$_ilg_msg"
    fi
}

# install_linux_musl installs the kraftkit package for non-standard linux
# distributions that use musl (musl).
# Returns:
# Code: 0 on success, 1 on error
install_linux_musl() {
    need_cmd "$AWK"
    need_cmd "$GREP"
    if check_os_release "alpine"; then
        need_cmd "$APK"
        _ilm_cmd=$(printf "%s%s"                    \
            "$APK add --no-cache --repository "     \
            "https://apk.pkg.kraftkit.sh kraftkit"  \
        )
        do_cmd "$_ilm_cmd"
    else
        _ilm_msg=$(printf "error: %s%s%s"                               \
            "Unsupported Linux distribution. "                          \
            "Try downloading the tar.gz file from "                     \
            "https://github/unikraft/kraftkit or switch to manual mode" \
        )
        err "$_ilm_msg"
    fi
}

# install_darwin installs the kraftkit package for MacOS distributions.
# Currently not implemented.
# $1: the architecture of the download
# Returns:
# Code: 1
install_darwin() {
    _ind_arch="$1"
    _ind_url="https://github.com/unikraft/kraftkit/issues/266"
    _ind_ext=".dmg"

    err "error: MacOS architecture unsupported: $_ind_arch."\
    "You can contribute at $_ind_url"
}

# install_windows installs the kraftkit package for windows distributions.
# Currently not implemented.
# $1: the architecture of the download
# Returns:
# Code: 1
install_windows() {
    _inw_arch="$1"
    _inw_url="https://github.com/unikraft/kraftkit/issues/267"
    _inw_ext=".msi"

    err "error: Windows architecture unsupported: $_inw_arch."\
    "You can contribute at $_inw_url"
}

# install_linux_manual installs the kraftkit package for other Linux
# distributions using the GitHub binaries.
# $1: the architecture of the download
# Returns:
# Code: 1
install_linux_manual() {
    _ill_arch="$1"

    _ill_version_url="$INSTALL_SERVER/latest.txt"
    _ill_version_file="latest.txt"
    downloader "$_ill_version_url" "$_ill_version_file" "$_ill_arch"
    say_debug "Got kraftkit version: $("$CAT" $_ill_version_file)"

    _ill_url=$(printf "%s%s%s%s"                \
        "https://github.com/unikraft/kraftkit"  \
        "/releases/latest/download/kraftkit_"   \
        "$("$CAT" $_ill_version_file)"          \
        "_linux_amd64.tar.gz"
    )
    _CLEANUP_VERSION="$_ill_version_file"

    get_user_response "change the install prefix? [$PREFIX] [y/N]: " "n"
    _ill_answer="$_RETVAL"

    if printf "%s" "$_ill_answer" | "$GREP" -q -E "$_NO_ANS_DEFAULT"; then
        :
    elif printf "%s" "$_ill_answer" | "$GREP" -q -E "$_YES_ANS"; then
        get_user_response "what should the prefix be? [$PREFIX]: " "$PREFIX"
        PREFIX="$_RETVAL"
    else
        err "fatal: choose either yes or no."
    fi

    _ill_binary="kraft"
    _ill_archive="kraftkit.tar.gz"
    downloader "$_ill_url" "$_ill_archive" "$_ill_arch"
    _CLEANUP_ARCHIVE="$_ill_archive"

    do_cmd "$TAR -xzf $_ill_archive"
    _CLEANUP_BINARY="$_ill_binary"

    do_cmd "$INSTALL $_ill_binary $PREFIX"

    cleanup
    _CLEANUP_ARCHIVE=""
    _CLEANUP_BINARY=""
    _CLEANUP_VERSION=""
}

# install_kraftkit installs the kraftkit package for the current architecture.
# $1: the architecture
# $2: whether to install in auto mode
# Returns:
# Code: 0 on success, 1 on error
install_kraftkit() {
    _ikk_arch="$1"
    _ikk_auto_install="$2"

    if [ -z "$_ikk_auto_install" ]; then
        say_debug "Installing kraftkit using package manager for $_ikk_arch"
        case $_ikk_arch in
            *"linux-gnu"*)
                install_linux_gnu
                ;;
            *"linux-musl"*)
                install_linux_musl
                ;;
            *"darwin"*)
                install_darwin "$_ikk_arch"
                ;;
            *"windows"*)
                install_windows "$_ikk_arch"
                ;;
            *)
                err "error: unsupported architecture: $_ikk_arch"
                ;; 
        esac
    else
        need_cmd "$TAR"
        need_cmd "$INSTALL"
        install_linux_manual "$_ikk_arch"
    fi
}

# arg_parse parses the arguments passed to the script.
# $@: the arguments to parse
# Returns:
# NEED_TTY: whether /dev/tty is needed
# Code: 0 on success, 1 on error
arg_parse() {
    for _agp_arg in "$@"; do
        case "$_agp_arg" in
            --help)
                usage
                exit 0
                ;;
            --debug)
                DEBUG=y
                ;;
            -d)
                DEBUG=y
                ;;
            *)
                OPTIND=1
                if [ "${_agp_arg%%--*}" = "" ]; then
                    # Long option (other than --help);
                    # don't attempt to interpret it.
                    continue
                fi
                while getopts :hyd _agp_sub_arg "$_agp_arg"; do
                    case "$_agp_sub_arg" in
                        h)
                            usage
                            exit 0
                            ;;
                        y)
                            # user wants to skip the prompt --
                            # we don't need /dev/tty
                            NEED_TTY=n
                            ;;
                        d)
                            # user wants debugging output
                            DEBUG=y
                            ;;
                        *)
                            ;;
                        esac
                done
                ;;
        esac
    done
}

# check_autoinstall checks if the user wants to automatically install kraftkit
# Returns:
# _RETVAL: whether to install automatically: n or empty
check_autoinstall() {
    _cai_answer=""
    get_user_response "install kraftkit using package manager? [Y/n]: " "y"
    _cai_answer="$_RETVAL"

    _cai_auto_install=""
    if printf "%s" "$_cai_answer" | "$GREP" -q -E "$_NO_ANS"; then
        _cai_auto_install="n"
    elif printf "%s" "$_cai_answer" | "$GREP" -q -E "$_YES_ANS_DEFAULT"; then
        say "installing kraftkit via package manager..."
    else
        err "fatal: choose either yes or no."
    fi

    _RETVAL="$_cai_auto_install"
}

# main is the entrypoint of the script.
# The steps are:
# 1. Check if we have all the commands we need
# 2. Parse the arguments passed to the script
# 3. Detect the architecture and OS of the current machine
# 4. Check if the user wants to install kraftkit automatically or not
# 5. Install kraftkit for the given architecture step by step
# 6. Exit with the appropriate code
# $@: the arguments to parse (see usage)
# Returns:
# Code: 0 on success if kraftkit is installed, something else otherwise
main() {
    # Check if we have all the commands we need
    downloader --check "" ""
    need_cmd "$UNAME"
    need_cmd "$RM"

    # Check if we have to use /dev/tty to prompt the user
    NEED_TTY=y
    arg_parse "$@"
    say_debug "Parsed arguments: NEED_TTY: $NEED_TTY"

    # Detect the architecture and OS of the current machine
    get_architecture || return 1
    _main_arch="$_RETVAL"
    assert_nz "$_main_arch" "arch"
    say_debug "Detected architecture: $_main_arch"

    # Check if the user wants to install kraftkit automatically
    check_autoinstall
    _main_auto_install="$_RETVAL"
    say_debug "Auto install: $_main_auto_install"

    # Install kraftkit for the given architecture
    install_kraftkit "$_main_arch" "$_main_auto_install"
    say "kraftkit was installed successfully to $PREFIX/kraft"

    # Check if kraft is installed and working
    kraft -h

    return $?
}


main "$@" || exit 1

# File taken and modified from: https://github.com/rust-lang/rustup
# Original credits go to the Rust project and its contributors.