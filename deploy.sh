#!/usr/bin/env bash
# House on the Hill - WebAssembly deployment via wasmserve.
#
# Architecture (to dodge network restrictions):
#   browser ---> :PORT (proxy)
#       /wasm_exec.js -> served locally from GOROOT
#       *             -> reverse-proxied to wasmserve on :INTERNAL_PORT
#
# Usage:
#   ./deploy.sh            # foreground: install wasmserve, run proxy + wasmserve
#   ./deploy.sh start      # background (nohup) for both processes
#   ./deploy.sh stop       # stop background processes
#   ./deploy.sh restart    # stop then start
#   ./deploy.sh status     # report background status
#
# Environment:
#   PORT=7426              # external port (browser-facing)
#   INTERNAL_PORT=17426    # wasmserve internal port

set -euo pipefail

cd "$(cd "$(dirname "$0")" && pwd)"

PORT="${PORT:-7426}"
INTERNAL_PORT="${INTERNAL_PORT:-17426}"
PKG_PATH="./cmd"
PROXY_PKG="./cmd/proxy"
WASMSERVE_VERSION="latest"
WS_PID_FILE="server.wasmserve.pid"
PROXY_PID_FILE="server.proxy.pid"
LOG_FILE="server.log"

log() { printf "\033[36m[deploy]\033[0m %s\n" "$*"; }
err() { printf "\033[31m[deploy]\033[0m %s\n" "$*" >&2; }

ensure_wasmserve() {
    local gobin
    gobin="$(go env GOBIN)"
    if [ -z "$gobin" ]; then
        gobin="$(go env GOPATH)/bin"
    fi
    export PATH="$gobin:$PATH"
    if command -v wasmserve >/dev/null 2>&1; then
        return 0
    fi
    log "Installing wasmserve@${WASMSERVE_VERSION} into ${gobin}..."
    GOFLAGS="-mod=mod" go install "github.com/hajimehoshi/wasmserve@${WASMSERVE_VERSION}"
    if ! command -v wasmserve >/dev/null 2>&1; then
        err "wasmserve install failed"
        exit 1
    fi
}

locate_wasm_exec() {
    local goroot
    goroot="$(go env GOROOT)"
    local candidates=(
        "$goroot/lib/wasm/wasm_exec.js"
        "$goroot/misc/wasm/wasm_exec.js"
    )
    local mod
    mod="$(go env GOMODCACHE)"
    if [ -n "$mod" ] && [ -d "$mod/golang.org" ]; then
        while IFS= read -r p; do
            candidates+=("$p")
        done < <(find "$mod/golang.org" -maxdepth 5 -name 'wasm_exec.js' -path '*/lib/wasm/*' 2>/dev/null || true)
    fi
    for c in "${candidates[@]}"; do
        if [ -f "$c" ]; then
            printf "%s" "$c"
            return 0
        fi
    done
    return 1
}

start_processes_fg() {
    ensure_wasmserve
    local wasm_exec=""
    if wasm_exec=$(locate_wasm_exec); then
        log "Local wasm_exec.js: $wasm_exec"
    else
        log "[warn] no local wasm_exec.js; proxy will fall through to wasmserve (network needed)"
    fi
    log "Starting wasmserve on :${INTERNAL_PORT} ..."
    GOOS=js GOARCH=wasm wasmserve -http ":${INTERNAL_PORT}" "${PKG_PATH}" >>"$LOG_FILE" 2>&1 &
    local ws_pid=$!
    echo "$ws_pid" > "$WS_PID_FILE"
    sleep 1
    if ! kill -0 "$ws_pid" 2>/dev/null; then
        err "wasmserve failed to start, see $LOG_FILE"
        rm -f "$WS_PID_FILE"
        tail -n 20 "$LOG_FILE" >&2 || true
        exit 1
    fi
    trap 'log "Stopping..."; kill $ws_pid 2>/dev/null || true; rm -f '"$WS_PID_FILE $PROXY_PID_FILE"'; exit 0' INT TERM
    log "Starting proxy on :${PORT} (Ctrl+C to stop)..."
    exec go run "$PROXY_PKG" \
        -listen ":${PORT}" \
        -upstream "http://127.0.0.1:${INTERNAL_PORT}" \
        -wasm-exec "$wasm_exec"
}

start_processes_bg() {
    ensure_wasmserve
    if [ -f "$WS_PID_FILE" ] && kill -0 "$(cat "$WS_PID_FILE")" 2>/dev/null; then
        log "wasmserve already running (pid $(cat "$WS_PID_FILE"))"
        return 0
    fi
    : >"$LOG_FILE"
    local wasm_exec=""
    if wasm_exec=$(locate_wasm_exec); then
        log "Local wasm_exec.js: $wasm_exec"
    else
        log "[warn] no local wasm_exec.js; proxy will fall through to wasmserve (network needed)"
    fi
    log "Launching wasmserve in background on :${INTERNAL_PORT}..."
    nohup env GOOS=js GOARCH=wasm wasmserve -http ":${INTERNAL_PORT}" "${PKG_PATH}" >>"$LOG_FILE" 2>&1 &
    echo $! > "$WS_PID_FILE"
    sleep 1
    if ! kill -0 "$(cat "$WS_PID_FILE")" 2>/dev/null; then
        err "wasmserve failed; tail of $LOG_FILE:"
        tail -n 20 "$LOG_FILE" >&2 || true
        rm -f "$WS_PID_FILE"
        exit 1
    fi
    log "Launching proxy in background on :${PORT}..."
    nohup go run "$PROXY_PKG" \
        -listen ":${PORT}" \
        -upstream "http://127.0.0.1:${INTERNAL_PORT}" \
        -wasm-exec "$wasm_exec" >>"$LOG_FILE" 2>&1 &
    echo $! > "$PROXY_PID_FILE"
    sleep 1
    if ! kill -0 "$(cat "$PROXY_PID_FILE")" 2>/dev/null; then
        err "proxy failed; tail of $LOG_FILE:"
        tail -n 20 "$LOG_FILE" >&2 || true
        kill "$(cat "$WS_PID_FILE")" 2>/dev/null || true
        rm -f "$WS_PID_FILE" "$PROXY_PID_FILE"
        exit 1
    fi
    log "wasmserve pid=$(cat "$WS_PID_FILE")  proxy pid=$(cat "$PROXY_PID_FILE")"
    log "Open http://<server-ip>:${PORT}/ in your browser. Logs: $LOG_FILE"
}

stop_processes() {
    log "Forcing stop of any process using ports ${PORT} and ${INTERNAL_PORT}..."
    
    # 使用 fuser 强行杀死占用端口的进程
    fuser -k "${PORT}/tcp" 2>/dev/null || true
    fuser -k "${INTERNAL_PORT}/tcp" 2>/dev/null || true
    
    # 清理 PID 文件
    rm -f "$PROXY_PID_FILE" "$WS_PID_FILE"
    
    log "Cleanup complete."
}

status_processes() {
    for f in "$WS_PID_FILE" "$PROXY_PID_FILE"; do
        if [ -f "$f" ] && kill -0 "$(cat "$f")" 2>/dev/null; then
            log "$f -> pid $(cat "$f") (running)"
        else
            log "$f -> not running"
        fi
    done
    log "External port: ${PORT}    Internal port: ${INTERNAL_PORT}"
}

case "${1:-serve}" in
    serve|"")        start_processes_fg ;;
    start)           start_processes_bg ;;
    stop)            stop_processes ;;
    restart)         stop_processes || true; start_processes_bg ;;
    status)          status_processes ;;
    *)
        err "Unknown command: $1"
        err "Usage: $0 [serve|start|stop|restart|status]"
        exit 2
        ;;
esac
