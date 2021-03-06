#!/bin/bash

# This script downloads the latest Aptomi release from GitHub for your platform,
# installs server and client locally and creates default configs for them
# The list of releases is retrieved from: https://github.com/Aptomi/aptomi/releases

APTOMI_INSTALL_DIR="/usr/local/bin"
APTOMI_SERVER_CONFIG_DIR="/etc/aptomi"
APTOMI_CLIENT_CONFIG_DIR="$HOME/.aptomi"
APTOMI_INSTALL_CACHE="$HOME/.aptomi-install-cache"
APTOMI_DB_DIR="/var/lib/aptomi"
REPO_NAME="Aptomi/aptomi"
SCRIPT_NAME=`basename "$0"`
UPLOAD_EXAMPLE=0
CLIENT_ONLY=0
SERVER_PID=""
APTOMI_INSTALLED_SERVER_VERSION=""
APTOMI_INSTALLED_CLIENT_VERSION=""

COLOR_GRAY='\033[0;37m'
COLOR_BLUE='\033[0;34m'
COLOR_RED='\033[0;31m'
COLOR_GREEN='\033[0;92m'
COLOR_YELLOW='\033[0;93m'
COLOR_RESET='\033[0m'

DEBUG=${DEBUG:-no}
if [ "yes" == "$DEBUG" ]; then
    set -x
fi

trap "script_done" INT TERM EXIT

set -eou pipefail

function script_done() {
    local CODE=$?

    if [ ! -z "${SERVER_PID}" ]; then
        log_sub "Stopping aptomi server (PID: ${SERVER_PID})"
        kill ${SERVER_PID} >/dev/null 2>&1
        while ps -p ${SERVER_PID} >/dev/null; do sleep 1; done
    fi

    if [ ! $CODE -eq 0 ]; then
        log_err "Script failed (set DEBUG=yes environment variable and run again)"
    fi

    exit $CODE
}

function check_installed() {
    if ! [ -x "$(command -v $1)" ]; then
        log_err "$1 is not installed" >&2
        exit 1
    fi
}

function log() {
    echo -e "$COLOR_BLUE[$(date +"%F %T")] $SCRIPT_NAME $COLOR_RED|$COLOR_RESET" $@$COLOR_GRAY
}

function log_sub() {
    echo -e "$COLOR_BLUE[$(date +"%F %T")] $SCRIPT_NAME $COLOR_RED|$COLOR_RESET - " $@$COLOR_GRAY
}

function log_warn() {
    echo -e "$COLOR_BLUE[$(date +"%F %T")] $SCRIPT_NAME $COLOR_RED|$COLOR_RESET$COLOR_YELLOW - " $@$COLOR_GRAY
}

function log_err() {
    echo -e "$COLOR_BLUE[$(date +"%F %T")] $SCRIPT_NAME $COLOR_RED| ERROR:" $@$COLOR_GRAY
}

function get_arch() {
    local ARCH=$(uname -m)
    case $ARCH in
        armv5*) ARCH="armv5";;
        armv6*) ARCH="armv6";;
        armv7*) ARCH="armv7";;
        aarch64) ARCH="arm64";;
        x86) ARCH="386";;
        x86_64) ARCH="amd64";;
        i686) ARCH="386";;
        i386) ARCH="386";;
    esac
    echo $ARCH
}

function get_os() {
    local OS=$(echo `uname`|tr '[:upper:]' '[:lower:]')
    case "$OS" in
        mingw*) OS='windows';;
    esac
    echo $OS
}

function verify_supported_platform() {
    local ARCH=$1
    local OS=$2

    if [ -z "${ARCH}" ] || [ -z "${OS}" ]; then
        log_err "Unable to detect platform: architecture=$ARCH, os=$OS"
        exit 1
    fi
    log_sub "Architecture=$COLOR_GREEN$ARCH$COLOR_RESET, os=$COLOR_GREEN$OS$COLOR_RESET"

    local supported="darwin_amd64\nlinux_386\nlinux_amd64"
    if ! echo "${supported}" | grep -q "${OS}_${ARCH}"; then
        log_err "No binaries available for ${OS}_${ARCH}"
        log_err "To build from source, go to https://github.com/$REPO_NAME#building-from-source"
        exit 1
    fi
}

function get_latest_release() {
    curl --silent "https://api.github.com/repos/$REPO_NAME/releases/latest" | # Get latest release from GitHub API
    grep '"tag_name":' |                                              # Filter out tag_name line
    sed -E 's/.*"v([^"]+)".*/\1/'                                      # Parse out JSON value
}

function download_and_install_release() {
    local ARCH=$1
    local OS=$2
    local VERSION=$3

    if [ -z "${VERSION}" ]; then
        log_err "Unable to get the latest release from GitHub (https://api.github.com/repos/$REPO_NAME/releases/latest)"
        exit 1
    fi
    log_sub "Latest Version: $COLOR_GREEN$VERSION$COLOR_RESET"

    local FILENAMEBINARY="aptomi_${VERSION}_${OS}_${ARCH}.tar.gz"

    local FILENAMECHECKSUMS="aptomi_${VERSION}_checksums.txt"
    local URL_BINARY="https://github.com/$REPO_NAME/releases/download/v$VERSION/$FILENAMEBINARY"
    local URL_CHECKSUMS="https://github.com/$REPO_NAME/releases/download/v$VERSION/$FILENAMECHECKSUMS"

    local FILE_BINARY="$APTOMI_INSTALL_CACHE/$FILENAMEBINARY"
    local FILE_CHECKSUMS="$APTOMI_INSTALL_CACHE/$FILENAMECHECKSUMS"

    mkdir -p $APTOMI_INSTALL_CACHE
    if [ ! -f $FILE_BINARY ]; then
        log_sub "Downloading: $URL_BINARY"
        curl -SsL "$URL_BINARY" -o "$FILE_BINARY"
    else
        log_sub "Already downloaded. Using from cache: $APTOMI_INSTALL_CACHE"
    fi

    # Never cache checksum, it'll allow us to verify cached binary
    log_sub "Downloading: $URL_CHECKSUMS"
    curl -SsL "$URL_CHECKSUMS" -o "$FILE_CHECKSUMS"

    local sum=$(openssl sha1 -sha256 ${FILE_BINARY} | awk '{print $2 xxx}')
    local expected_line=$(cat ${FILE_CHECKSUMS} | grep ${FILENAMEBINARY})
    if [ "$sum  ${FILENAMEBINARY}" != "$expected_line" ]; then
        log_err "Failed to download '${FILENAMEBINARY}' or SHA sum does not match. Aborting install"
        exit 1
    fi

    install_binaries_from_archive $FILE_BINARY $FILENAMEBINARY $VERSION
}

function run_as_root() {
    CMD="$*"

    if [ $EUID -ne 0 ]; then
        CMD="sudo $CMD"
    fi

    $CMD
}

function install_binaries_from_archive() {
    local FILE_BINARY=$1
    local FILENAMEBINARY=$2
    local VERSION=$3
    local TMP_DIR="$(mktemp -dt aptomi-install-unpacked-XXXXXX)"

    # Unpack the archive
    log "Installing/Updating Aptomi"
    log_sub "Unpacking $FILENAMEBINARY"
    tar xf "$FILE_BINARY" -C "$TMP_DIR"

    # Cut .tar.gz to get the name of the directory inside the archive
    local DIRNAME="${FILENAMEBINARY%.*}"
    DIRNAME="${DIRNAME%.*}"
    UNPACKED_PATH="$TMP_DIR/$DIRNAME"

    # Install server (only if we are not in CLIENT_ONLY mode)
    if [ $CLIENT_ONLY -eq 0 ]; then
        if [ ! -f $UNPACKED_PATH/aptomi ]; then
            log_err "Binary 'aptomi' not found inside the release"
        fi

        if [ -z $APTOMI_INSTALLED_SERVER_VERSION ]; then
            log_sub "Installing Aptomi server: $COLOR_GREEN${APTOMI_INSTALL_DIR}/aptomi"
            run_as_root cp "$UNPACKED_PATH/aptomi" "$APTOMI_INSTALL_DIR"
        elif [ $APTOMI_INSTALLED_SERVER_VERSION != $VERSION ]; then
            log_sub "Updating Aptomi server: $COLOR_GREEN${APTOMI_INSTALL_DIR}/aptomi$COLOR_RESET ($COLOR_GREEN$APTOMI_INSTALLED_SERVER_VERSION$COLOR_RESET -> $COLOR_GREEN$VERSION$COLOR_RESET)"
            run_as_root cp "$UNPACKED_PATH/aptomi" "$APTOMI_INSTALL_DIR"
        else
            log_sub "Aptomi server is already at the required version. Skipping install"
        fi
    fi

    if [ ! -f $UNPACKED_PATH/aptomictl ]; then
        log_err "Binary 'aptomictl' not found inside the release"
    fi

    # Install client
    if [ -z $APTOMI_INSTALLED_CLIENT_VERSION ]; then
        log_sub "Installing Aptomi client: $COLOR_GREEN${APTOMI_INSTALL_DIR}/aptomictl$COLOR_RESET"
        run_as_root cp "$UNPACKED_PATH/aptomictl" "$APTOMI_INSTALL_DIR"
    elif [ $APTOMI_INSTALLED_CLIENT_VERSION != $VERSION ]; then
        log_sub "Updating Aptomi client: $COLOR_GREEN${APTOMI_INSTALL_DIR}/aptomi$COLOR_RESET ($COLOR_GREEN$APTOMI_INSTALLED_CLIENT_VERSION$COLOR_RESET -> $COLOR_GREEN$VERSION$COLOR_RESET)"
        run_as_root cp "$UNPACKED_PATH/aptomictl" "$APTOMI_INSTALL_DIR"
    else
        log_sub "Aptomi client is already at the required version. Skipping install"
    fi
}

function create_server_config() {
    # Skip server installation if we are in CLIENT_ONLY mode
    if [ $CLIENT_ONLY -eq 1 ]; then
        return 0
    fi

    local TMP_DIR="$(mktemp -dt aptomi-install-server-config-XXXXXX)"

    if [ -f ${APTOMI_SERVER_CONFIG_DIR}/config.yaml ]; then
        log_warn "Config for Aptomi server already exists. Keeping ${APTOMI_SERVER_CONFIG_DIR}/config.yaml"
    else
        log_sub "Creating config for Aptomi server: $COLOR_GREEN${APTOMI_SERVER_CONFIG_DIR}/config.yaml$COLOR_RESET"

        # If we are in example mode, disable enforcer
        local NOOP="false"
        if [ $UPLOAD_EXAMPLE -eq 1 ]; then
            log_sub "Disabling all state updates (as we are running in example mode)"
            NOOP="true"
        fi

        cat >${TMP_DIR}/config.yaml <<EOL
debug: true

api:
  host: 0.0.0.0
  port: 27866

db:
  connection: ${APTOMI_DB_DIR}/db.bolt

enforcer:
  noop: ${NOOP}
  interval: 60s

updater:
  noop: ${NOOP}
  interval: 60s

users:
  file:
    - ${APTOMI_SERVER_CONFIG_DIR}/users_builtin.yaml
    - ${APTOMI_SERVER_CONFIG_DIR}/users_example.yaml
  ldap-disabled:
    - host: localhost
      port: 10389
      basedn: "o=aptomiOrg"
      filter: "(&(objectClass=organizationalPerson))"
      filterbyname: "(&(objectClass=organizationalPerson)(cn=%s))"
      labeltoattributes:
        name: cn
        description: description
        global_ops: isglobalops
        is_operator: isoperator
        mail: mail
        team: team
        org: o
        short-description: role
        deactivated: deactivated
EOL
        run_as_root mkdir -p ${APTOMI_SERVER_CONFIG_DIR}
        run_as_root cp ${TMP_DIR}/config.yaml ${APTOMI_SERVER_CONFIG_DIR}/config.yaml
    fi

    if [ -f ${APTOMI_SERVER_CONFIG_DIR}/users_builtin.yaml ]; then
        log_warn "Built-in admin users for Aptomi server already exist. Keeping ${APTOMI_SERVER_CONFIG_DIR}/users_builtin.yaml"
    else
        log_sub "Creating built-in admin users for Aptomi server: $COLOR_GREEN${APTOMI_SERVER_CONFIG_DIR}/users_builtin.yaml$COLOR_RESET"
        cat >${TMP_DIR}/users_builtin.yaml <<EOL
- name: admin
  passwordhash: "\$2a\$10\$2eh0YI/gzj2UdxN8j52NseQW54BsZ5cUGhFstblR1D8UOGMUCwuMm"
  domainadmin: true
EOL
        run_as_root mkdir -p ${APTOMI_SERVER_CONFIG_DIR}
        run_as_root cp ${TMP_DIR}/users_builtin.yaml ${APTOMI_SERVER_CONFIG_DIR}/users_builtin.yaml
    fi

    if [ -f ${APTOMI_SERVER_CONFIG_DIR}/users_example.yaml ]; then
        log_warn "Example users for Aptomi server already exist. Keeping ${APTOMI_SERVER_CONFIG_DIR}/users_example.yaml"
    else
        log_sub "Creating example users for Aptomi server: $COLOR_GREEN${APTOMI_SERVER_CONFIG_DIR}/users_example.yaml$COLOR_RESET"
        run_as_root mkdir -p ${APTOMI_SERVER_CONFIG_DIR}

        if [ -d "${UNPACKED_PATH}/examples/twitter-analytics/_external" ]; then
            run_as_root cp ${UNPACKED_PATH}/examples/twitter-analytics/_external/users.yaml ${APTOMI_SERVER_CONFIG_DIR}/users_example.yaml
        else
            run_as_root cp ${UNPACKED_PATH}/examples/common/users.yaml ${APTOMI_SERVER_CONFIG_DIR}/users_example.yaml
        fi
    fi

    if [ -d ${APTOMI_DB_DIR} ]; then
        log_warn "Aptomi server database directory already exists. Keeping data under ${APTOMI_DB_DIR}"
    else
        log_sub "Creating Aptomi server database directory: $COLOR_GREEN${APTOMI_DB_DIR}$COLOR_RESET"
        run_as_root mkdir -p ${APTOMI_DB_DIR}
    fi

    run_as_root chown ${USER:=$(/usr/bin/id -run)} ${APTOMI_DB_DIR}
}

function create_client_config() {
    local TMP_DIR="$(mktemp -dt aptomi-install-client-config-XXXXXX)"

    if [ -f ${APTOMI_CLIENT_CONFIG_DIR}/config.yaml ]; then
        log_warn "Config for Aptomi client already exists. Keeping ${APTOMI_CLIENT_CONFIG_DIR}/config.yaml"
    else
        log_sub "Creating config for Aptomi client: $COLOR_GREEN${APTOMI_CLIENT_CONFIG_DIR}/config.yaml$COLOR_RESET"
        cat >${TMP_DIR}/config.yaml <<EOL
debug: true

api:
  host: 127.0.0.1
  port: 27866
EOL
        mkdir -p ${APTOMI_CLIENT_CONFIG_DIR}
        cp ${TMP_DIR}/config.yaml ${APTOMI_CLIENT_CONFIG_DIR}/config.yaml
    fi
}

function copy_examples() {
    log_sub "Copying examples into $COLOR_GREEN${APTOMI_CLIENT_CONFIG_DIR}/examples$COLOR_RESET"

    mkdir -p ${APTOMI_CLIENT_CONFIG_DIR}
    cp -R ${UNPACKED_PATH}/examples ${APTOMI_CLIENT_CONFIG_DIR}/
}

function test_aptomi_server_in_path() {
    # Verify that Aptomi server is in path
    local APTOMI=`which aptomi`
    if [ "$APTOMI" == "$APTOMI_INSTALL_DIR/aptomi" ]; then
        log_sub "Aptomi server: ${COLOR_GREEN}OK${COLOR_RESET} (which aptomi -> $APTOMI)"
    else
        log_warn "Aptomi server: 'which aptomi' returned '$APTOMI', but expected '$APTOMI_INSTALL_DIR/aptomi'"
        exit 1
    fi
}

function test_aptomi_client_in_path() {
    # Verify that Aptomi client is in path
    local APTOMICTL=`which aptomictl`
    if [ "$APTOMICTL" == "$APTOMI_INSTALL_DIR/aptomictl" ]; then
        log_sub "Aptomi client: ${COLOR_GREEN}OK${COLOR_RESET} (which aptomictl -> $APTOMICTL)"
    else
        log_warn "Aptomi client: 'which aptomictl' returned '$APTOMICTL', but expected '$APTOMI_INSTALL_DIR/aptomictl'"
        exit 1
    fi
}

function test_aptomi_server_version_success() {
    # Run 'aptomi version' and remove leading whitespaces
    local SERVER_VERSION_OUTPUT=$(get_aptomi_server_version)
    if [ "$SERVER_VERSION_OUTPUT" != "$VERSION" ]; then
        log_err "Failed to verify 'aptomi version': got '$SERVER_VERSION_OUTPUT' instead of '$VERSION'"
        exit 1
    else
        log_sub "Running 'aptomi version': ${COLOR_GREEN}${SERVER_VERSION_OUTPUT}${COLOR_RESET}"
    fi
}

function is_aptomi_server_running() {
    local SERVER_RUNNING_PRIOR=`ps | grep aptomi | grep server`
    if [ ! -z "${SERVER_RUNNING_PRIOR}" ]; then
        echo 1
    else
        echo 0
    fi
}

function get_aptomi_server_version() {
    if [ -f ${APTOMI_INSTALL_DIR}/aptomi ]; then
        ${APTOMI_INSTALL_DIR}/aptomi version --short 2>/dev/null | grep 'Server Version:' | sed -E 's/[a-zA-Z ]*: (.*)/\1/'
    fi
}

function get_aptomi_client_version() {
    if [ -f ${APTOMI_INSTALL_DIR}/aptomictl ]; then
        ${APTOMI_INSTALL_DIR}/aptomictl version --client --short 2>/dev/null | grep 'Client Version:' | sed -E 's/[a-zA-Z ]*: (.*)/\1/'
    fi
}

function check_aptomi_install_status() {
    APTOMI_INSTALLED_SERVER_VERSION=$(get_aptomi_server_version)
    if [ -z "${APTOMI_INSTALLED_SERVER_VERSION}" ]; then
        log_sub "Aptomi Server: ${COLOR_YELLOW}not installed${COLOR_RESET}"
    else
        log_sub "Aptomi Server: ${COLOR_GREEN}${APTOMI_INSTALLED_SERVER_VERSION}${COLOR_RESET} in ${APTOMI_INSTALL_DIR}/aptomi"
    fi

    APTOMI_INSTALLED_CLIENT_VERSION=$(get_aptomi_client_version)
    if [ -z "${APTOMI_INSTALLED_CLIENT_VERSION}" ]; then
        log_sub "Aptomi Client: ${COLOR_YELLOW}not installed${COLOR_RESET}"
    else
        log_sub "Aptomi Client: ${COLOR_GREEN}${APTOMI_INSTALLED_CLIENT_VERSION}${COLOR_RESET} in ${APTOMI_INSTALL_DIR}/aptomictl"
    fi
}

function start_aptomi_server() {
    local RUNNING=$(is_aptomi_server_running)
    if [ $RUNNING -eq 1 ]; then
        log_err "Aptomi server already running. Can't run another instance for testing (may want to use 'killall aptomi')"
        exit 1
    fi

    # Start Aptomi server
    ${APTOMI_INSTALL_DIR}/aptomi server &>/dev/null &
    SERVER_PID=$!
    log_sub "Starting 'aptomi server' for testing (PID: ${SERVER_PID})"
    sleep 2
    local SERVER_RUNNING=`ps | grep aptomi | grep "${SERVER_PID}"`
    if [ -z "${SERVER_RUNNING}" ]; then
        log_err "Aptomi server failed to start"
        exit 1
    fi
}

function test_aptomi_client_version_success() {
    # Run client to show the version
    local CLIENT_VERSION_OUTPUT=$(get_aptomi_client_version)
    if [ "$CLIENT_VERSION_OUTPUT" != "$VERSION" ]; then
        log_err "Failed to verify 'aptomictl version': got '$CLIENT_VERSION_OUTPUT' instead of '$VERSION'"
        exit 1
    else
        log_sub "Running 'aptomictl version': ${COLOR_GREEN}${CLIENT_VERSION_OUTPUT}${COLOR_RESET}"
    fi
}

function test_aptomi_client_show_policy_success() {
    # Run client to show the policy
    local CLIENT_POLICY_SHOW_OUTPUT
    ${APTOMI_INSTALL_DIR}/aptomictl --config ${APTOMI_CLIENT_CONFIG_DIR} login -u admin -p admin 2>/dev/null
    CLIENT_POLICY_SHOW_OUTPUT=$(${APTOMI_INSTALL_DIR}/aptomictl --config ${APTOMI_CLIENT_CONFIG_DIR} policy show 2>/dev/null)
    if [ $? -eq 0 ]; then
        log_sub "Running 'aptomictl policy show': ${COLOR_GREEN}OK${COLOR_RESET}"
    else
        log_err "Failed to execute 'aptomictl policy show'"
        log_err $CLIENT_POLICY_SHOW_OUTPUT
        exit 1
    fi
}

function test_aptomi() {
    # Test that installed aptomi binaries are in PATH
    if [ $CLIENT_ONLY -eq 0 ]; then
        test_aptomi_server_in_path
        test_aptomi_server_version_success
        start_aptomi_server
    fi

    test_aptomi_client_in_path
    test_aptomi_client_version_success

    if [ $CLIENT_ONLY -eq 0 ]; then
        test_aptomi_client_show_policy_success

        # Upload example, if needed
        if [ $UPLOAD_EXAMPLE -eq 1 ]; then
            upload_example
        fi
    fi
}

function example_run_line() {
    local USERNAME=$1

    # Run login command
    local CMD_LOGIN="aptomictl --config ${APTOMI_CLIENT_CONFIG_DIR} login -u $USERNAME -p $USERNAME"
    log_sub "${CMD_LOGIN}"
    ${APTOMI_INSTALL_DIR}/$CMD_LOGIN 1>/dev/null 2>&1

    # Run actual command
    local CMD_RUN="aptomictl --config ${APTOMI_CLIENT_CONFIG_DIR} policy apply --wait -f ${APTOMI_CLIENT_CONFIG_DIR}/examples/$2"
    log_sub "${CMD_RUN}"
    ${APTOMI_INSTALL_DIR}/$CMD_RUN
}

function upload_example() {
    log_sub "Uploading example"
    example_run_line "sam" "twitter-analytics/policy/rules"
    example_run_line "sam" "twitter-analytics/policy/clusters/clusters.yaml.template"
    example_run_line "frank" "twitter-analytics/policy/analytics_pipeline"
    example_run_line "john" "twitter-analytics/policy/twitter_stats"
    example_run_line "john" "twitter-analytics/policy/john-prod-ts.yaml"
    example_run_line "alice" "twitter-analytics/policy/alice-dev-ts.yaml"
    example_run_line "bob" "twitter-analytics/policy/bob-dev-ts.yaml"
    example_run_line "carol" "twitter-analytics/policy/carol-dev-ts.yaml"
}

function help() {
    echo "This script installs Aptomi. Accepted CLI arguments are:"
    echo -e "\t--help: prints this help"
    echo -e "\t--with-example: imports example after installing and disables enforcer"
}

# Parsing input arguments (if any)
export INPUT_ARGUMENTS="$@"
while [[ $# -gt 0 ]]; do
  case $1 in
    '--client-only')
        CLIENT_ONLY=1
        ;;
    '--with-example')
        UPLOAD_EXAMPLE=1
        ;;
    '--help')
        help
        exit 0
        ;;
  esac
  shift
done

# Initial checks
log "Starting Aptomi install"
check_installed 'curl'
check_installed 'grep'
check_installed 'sed'
check_installed 'awk'
check_installed 'openssl'
check_installed 'tar'
check_installed 'cp'
check_installed 'mkdir'
check_installed 'cat'
check_installed 'ps'

# Detect platform and verify that it's supported
log "Checking environment"
ARCH=$(get_arch)
OS=$(get_os)
verify_supported_platform $ARCH $OS
check_aptomi_install_status

# Download the latest release from GitHub and install it
log "Checking the latest release on GitHub"
VERSION=$(get_latest_release)
download_and_install_release $ARCH $OS $VERSION

# Set up server and client locally on the same host
create_server_config
create_client_config
copy_examples

# Test Aptomi
log "Testing Aptomi"
test_aptomi

# Done
log "Done"
