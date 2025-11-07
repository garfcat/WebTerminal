#!/usr/bin/env bash

set -euo pipefail

PROGRAM_NAME="webterminal"
BIN_PATH="/usr/local/bin/webterminal"
ADDR="0.0.0.0:8089"
AUTH="false"
USERNAME="admin"
PASSWORD="password"
SHELL_CMD="sh"
ALLOWED_ORIGINS=""
RUN_USER="root"
WORKDIR="$(pwd)"
LOG_DIR="/var/log/webterminal"

usage() {
  cat <<EOF
Usage: sudo $0 [options]

Options:
  --program-name NAME       Supervisor program name (default: webterminal)
  --bin PATH                Path to webterminal binary (default: /usr/local/bin/webterminal)
  --addr HOST:PORT          Listen address (default: 0.0.0.0:8089)
  --auth true|false         Enable auth (default: false)
  --username USER           Basic auth username (default: admin)
  --password PASS           Basic auth password (default: password)
  --shell SHELL             Shell executable (default: sh)
  --allowed-origins LIST    Comma-separated Origin list; empty = same-origin only
  --user USER               System user to run as (default: root)
  --workdir DIR             Working directory (default: current dir)
  --log-dir DIR             Log directory (default: /var/log/webterminal)

Example:
  sudo $0 \
    --bin /usr/local/bin/webterminal \
    --addr 0.0.0.0:8089 \
    --auth true \
    --username admin \
    --password secret \
    --shell bash \
    --allowed-origins https://example.com \
    --user webterm \
    --workdir /var/lib/webterminal
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --program-name) PROGRAM_NAME="$2"; shift 2;;
      --bin) BIN_PATH="$2"; shift 2;;
      --addr) ADDR="$2"; shift 2;;
      --auth) AUTH="$2"; shift 2;;
      --username) USERNAME="$2"; shift 2;;
      --password) PASSWORD="$2"; shift 2;;
      --shell) SHELL_CMD="$2"; shift 2;;
      --allowed-origins) ALLOWED_ORIGINS="$2"; shift 2;;
      --user) RUN_USER="$2"; shift 2;;
      --workdir) WORKDIR="$2"; shift 2;;
      --log-dir) LOG_DIR="$2"; shift 2;;
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

write_conf() {
  mkdir -p "$LOG_DIR"
  local conf_path="/etc/supervisor/conf.d/${PROGRAM_NAME}.conf"
  local origins_arg=""
  if [[ -n "$ALLOWED_ORIGINS" ]]; then
    origins_arg=" --allowed-origins $ALLOWED_ORIGINS"
  fi

  cat > "$conf_path" <<CONF
[program:${PROGRAM_NAME}]
directory=$WORKDIR
command=$BIN_PATH --addr $ADDR --auth $AUTH --username $USERNAME --password $PASSWORD --shell $SHELL_CMD${origins_arg}
autostart=true
autorestart=true
startretries=3
user=$RUN_USER
stdout_logfile=$LOG_DIR/${PROGRAM_NAME}.out.log
stderr_logfile=$LOG_DIR/${PROGRAM_NAME}.err.log
stdout_logfile_maxbytes=10MB
stderr_logfile_maxbytes=10MB
stdout_logfile_backups=5
stderr_logfile_backups=5
stopsignal=TERM
stopwaitsecs=5
CONF

  echo "Wrote conf: $conf_path"
}

reload_update() {
  supervisorctl reread || true
  supervisorctl update || true
  supervisorctl status "$PROGRAM_NAME" || true
}

main() {
  parse_args "$@"
  require_root
  write_conf
  reload_update
}

main "$@"


