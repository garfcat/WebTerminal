#!/usr/bin/env bash

set -euo pipefail

SERVICE_NAME="webterminal"
BIN_PATH="/usr/local/bin/webterminal"
ADDR="0.0.0.0:8089"
AUTH="false"
USERNAME="admin"
PASSWORD="password"
SHELL_CMD="sh"
ALLOWED_ORIGINS=""
RUN_USER="root"
WORKDIR="$(pwd)"
ENV_FILE=""

usage() {
  cat <<EOF
Usage: sudo $0 [options]

Options:
  --service-name NAME       Service name (default: webterminal)
  --bin PATH                Path to webterminal binary (default: /usr/local/bin/webterminal)
  --addr HOST:PORT          Listen address (default: 0.0.0.0:8089)
  --auth true|false         Enable auth (default: false)
  --username USER           Basic auth username (default: admin)
  --password PASS           Basic auth password (default: password)
  --shell SHELL             Shell executable (default: sh)
  --allowed-origins LIST    Comma-separated Origin list; empty = same-origin only
  --user USER               System user to run as (default: root)
  --workdir DIR             Working directory (default: current dir)
  --env-file PATH           Optional EnvironmentFile for systemd (default: none)

Example:
  sudo $0 \
    --bin /usr/local/bin/webterminal \
    --addr 0.0.0.0:8089 \
    --auth true \
    --username admin \
    --password secret \
    --shell bash \
    --allowed-origins https://example.com,https://admin.example.com \
    --user webterm \
    --workdir /var/lib/webterminal
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --service-name) SERVICE_NAME="$2"; shift 2;;
      --bin) BIN_PATH="$2"; shift 2;;
      --addr) ADDR="$2"; shift 2;;
      --auth) AUTH="$2"; shift 2;;
      --username) USERNAME="$2"; shift 2;;
      --password) PASSWORD="$2"; shift 2;;
      --shell) SHELL_CMD="$2"; shift 2;;
      --allowed-origins) ALLOWED_ORIGINS="$2"; shift 2;;
      --user) RUN_USER="$2"; shift 2;;
      --workdir) WORKDIR="$2"; shift 2;;
      --env-file) ENV_FILE="$2"; shift 2;;
      -h|--help) usage; exit 0;;
      *) echo "Unknown option: $1"; usage; exit 1;;
    esac
  done
}

require_root() {
  if [[ $(id -u) -ne 0 ]]; then
    echo "This script must be run as root (use sudo)." >&2
    exit 1
  fi
}

write_unit() {
  local unit_path="/etc/systemd/system/${SERVICE_NAME}.service"
  local env_line=""
  if [[ -n "$ENV_FILE" ]]; then
    env_line="EnvironmentFile=$ENV_FILE"
  fi

  local origins_arg=""
  if [[ -n "$ALLOWED_ORIGINS" ]]; then
    origins_arg=" --allowed-origins $ALLOWED_ORIGINS"
  fi

  cat > "$unit_path" <<UNIT
[Unit]
Description=WebTerminal Service
After=network.target

[Service]
Type=simple
User=$RUN_USER
WorkingDirectory=$WORKDIR
${env_line}
ExecStart=$BIN_PATH --addr $ADDR --auth $AUTH --username $USERNAME --password $PASSWORD --shell $SHELL_CMD${origins_arg}
Restart=on-failure
RestartSec=3s

[Install]
WantedBy=multi-user.target
UNIT

  echo "Wrote unit: $unit_path"
}

reload_enable_start() {
  systemctl daemon-reload
  systemctl enable --now "$SERVICE_NAME"
  systemctl status "$SERVICE_NAME" --no-pager || true
}

main() {
  parse_args "$@"
  require_root
  write_unit
  reload_enable_start
}

main "$@"


