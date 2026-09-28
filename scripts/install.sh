#!/bin/sh
# notify-agent를 맥에 설치하고 로그인할 때마다 켜지게 한다.
#
#   ./scripts/install.sh              설치(또는 새 판으로 바꾸기)
#   ./scripts/install.sh --uninstall  지우기
#
# 설치 뒤 한 번은 사람이 해야 한다:
#   1) 시스템 설정 > 개인정보 보호 및 보안 > 전체 디스크 접근에 ~/.local/bin/notify-agent 추가
#   2) ~/.config/notify-agent/env 의 NOTIFY_APPS 정하기(notify-agent apps)
#   3) relay-service .env 에 RELAY_HOOK_KEY(아래에서 만든 키)를 넣고 다시 켜기
set -eu

LABEL=com.foncsoft.notify-agent
BIN_DIR="$HOME/.local/bin"
BIN="$BIN_DIR/notify-agent"
CONF_DIR="$HOME/.config/notify-agent"
ENV_FILE="$CONF_DIR/env"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
LOG="$HOME/Library/Logs/notify-agent.log"

if [ "${1:-}" = "--uninstall" ]; then
  launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
  rm -f "$PLIST" "$BIN"
  echo "지웠습니다. 설정($CONF_DIR)과 로그($LOG)는 남겨 둡니다."
  exit 0
fi

cd "$(dirname "$0")/.."
[ -x ./notify-agent ] || { echo "먼저 make build 하세요." >&2; exit 1; }

mkdir -p "$BIN_DIR" "$CONF_DIR" "$(dirname "$PLIST")"
# 바이너리를 바꾸면 전체 디스크 접근을 다시 줘야 할 수 있다(서명이 바뀐다).
cp ./notify-agent "$BIN"

if [ ! -f "$ENV_FILE" ]; then
  KEY=$(openssl rand -hex 24)
  sed "s/^NOTIFY_HOOK_KEY=.*/NOTIFY_HOOK_KEY=$KEY/" env.example > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  echo "설정 파일을 만들었습니다: $ENV_FILE"
  echo "relay-service .env 에 아래 줄을 넣고 relay-service를 다시 켜세요:"
  echo "  RELAY_HOOK_KEY=$KEY"
fi

cat > "$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key><array><string>$BIN</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <!-- 권한이 없어 바로 끝나면 10초 뒤에 다시 켠다. 권한을 주면 저절로 돈다. -->
  <key>ThrottleInterval</key><integer>10</integer>
  <key>StandardOutPath</key><string>$LOG</string>
  <key>StandardErrorPath</key><string>$LOG</string>
</dict>
</plist>
PLIST

launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
launchctl bootstrap "gui/$(id -u)" "$PLIST"
echo "켰습니다. 로그: $LOG"
echo "전체 디스크 접근에 $BIN 을 추가해야 알림을 읽습니다."
