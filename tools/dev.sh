#!/usr/bin/env bash
# Flutter-style runner for the agent-notify daemon.
#
#   tools/dev.sh          # dev mode: build working tree, run, hot-restart on `r`
#   tools/dev.sh prod     # prod mode: relaunch /Applications/agent-notify.app
#
# Keys:
#   r  rebuild + restart daemon (dev) / relaunch app (prod)
#   s  show dashboard window
#   t  tail daemon log
#   d  detach: leave daemon running, exit script
#   q  quit daemon and exit

set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MODE="${1:-dev}"
BIN="/tmp/agent-notify-dev"
LOG="/tmp/agent-notify-dev.log"
APP="/Applications/agent-notify.app"
PORT=49517

say() { printf '\033[1;33m[dev.sh]\033[0m %s\n' "$*"; }

kill_daemons() {
  # dev binary, prod app bundle, or a stray `go run` — anything holding the port
  pkill -f "$BIN daemon" 2>/dev/null
  pkill -f "$APP/Contents/MacOS/agent-notify" 2>/dev/null
  # wait until the port is actually free (daemon retries its bind for ~5s anyway)
  for _ in $(seq 1 20); do
    lsof -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1 || return 0
    sleep 0.25
  done
  say "warning: port $PORT still held"
}

wait_up() {
  for _ in $(seq 1 40); do
    if curl -s -o /dev/null "http://127.0.0.1:$PORT/ping"; then return 0; fi
    sleep 0.25
  done
  return 1
}

show() { curl -s -X POST "http://127.0.0.1:$PORT/show" -o /dev/null; }

start_dev() {
  say "building..."
  # build to a temp path first: a broken build must not kill the running daemon
  if ! (cd "$ROOT" && go build -o "$BIN.new" . 2>&1 | grep -v '^ld: warning'); then
    say "BUILD FAILED — running daemon left untouched"
    return 1
  fi
  mv -f "$BIN.new" "$BIN"
  kill_daemons
  AGENT_NOTIFY_DEV=1 "$BIN" daemon >"$LOG" 2>&1 &
  disown
  if wait_up; then
    say "dev daemon up (log: $LOG)"
  else
    say "daemon did not come up — check: tail $LOG"
    return 1
  fi
}

start_prod() {
  open "$APP"
  if wait_up; then
    say "prod app up"
  else
    say "prod app did not come up"
    return 1
  fi
}

restart() {
  # dev builds first and only swaps the daemon on success (start_dev kills)
  if [ "$MODE" = "prod" ]; then kill_daemons; start_prod; else start_dev; fi
}

case "$MODE" in
  dev|prod) ;;
  *) echo "usage: tools/dev.sh [dev|prod]" >&2; exit 2 ;;
esac

C_HEAD='\033[1;36m'; C_KEY='\033[1;33m'; C_DIM='\033[2m'; C_OFF='\033[0m'
printf '\n'
printf "${C_HEAD}  agent-notify runner${C_OFF}  ${C_DIM}mode:${C_OFF} %s\n" "$MODE"
printf '\n'
printf "  ${C_KEY}r${C_OFF}  rebuild + restart daemon\n"
printf "  ${C_KEY}s${C_OFF}  show dashboard window\n"
printf "  ${C_KEY}t${C_OFF}  tail daemon log\n"
printf "  ${C_KEY}d${C_OFF}  detach (leave daemon running)\n"
printf "  ${C_KEY}q${C_OFF}  quit daemon and exit\n"
printf '\n'
# first launch opens the window; later `r` restarts stay quiet (press s)
restart && show

while true; do
  IFS= read -rsn1 key || break
  case "$key" in
    r) say "restarting..."; restart ;;
    s) show ;;
    t) [ "$MODE" = "dev" ] && tail -20 "$LOG" || say "prod logs: Console.app / launchd" ;;
    d) say "detached — daemon keeps running"; exit 0 ;;
    q)
      say "quitting daemon..."
      kill_daemons
      exit 0
      ;;
  esac
done
