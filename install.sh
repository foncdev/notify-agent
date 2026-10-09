#!/bin/sh
#
# notify-agent 설치 스크립트. (terminal-agent·claudeAgent의 install.sh와 같은 규칙)
#
#   curl -fsSL https://raw.githubusercontent.com/foncdev/notify-agent/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/foncdev/notify-agent/main/install.sh | sh -s -- --uninstall
#
# GitHub Release에서 이 맥에 맞는 묶음을 받아, 안에 든 scripts/install.sh로 설치한다
# (~/.local/bin/notify-agent, 로그인할 때마다 켜지는 LaunchAgent, ~/.config/notify-agent/env).
# 맥 전용이다 — 알림 센터 DB를 읽는다.
#
# 환경변수로 바꿀 수 있는 것:
#   VERSION   받을 버전 (기본: 최신 릴리스)

set -eu

REPO="foncdev/notify-agent"
BIN="notify-agent"

# --- 출력 ---

# 터미널이면 색을 쓰고, 파이프면 쓰지 않는다.
if [ -t 1 ]; then
	c_dim='\033[2m'; c_red='\033[31m'; c_grn='\033[32m'; c_off='\033[0m'
else
	c_dim=''; c_red=''; c_grn=''; c_off=''
fi

say()  { printf '%b\n' "$*"; }
info() { printf '%b\n' "${c_dim}  $*${c_off}"; }
ok()   { printf '%b\n' "${c_grn}✓${c_off} $*"; }
die()  { printf '%b\n' "${c_red}✗${c_off} $*" >&2; exit 1; }

# --- 플랫폼 확인 ---

detect_os() {
	case "$(uname -s)" in
		Darwin) echo darwin ;;
		*) die "notify-agent는 맥 전용입니다(알림 센터를 읽습니다): $(uname -s)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		arm64|aarch64) echo arm64 ;;
		x86_64|amd64)  echo amd64 ;;
		*) die "지원하지 않는 아키텍처입니다: $(uname -m)" ;;
	esac
}

# --- 필요한 도구 ---

need() {
	command -v "$1" >/dev/null 2>&1 || die "$1 이 필요합니다."
}

# 받기. curl이 없으면 wget을 쓴다.
fetch() {
	url="$1"; out="$2"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$out"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$out" "$url"
	else
		die "curl 또는 wget이 필요합니다."
	fi
}

fetch_stdout() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1"
	else
		wget -qO- "$1"
	fi
}

# --- 최신 버전 찾기 ---

latest_version() {
	# GitHub API에서 최신 릴리스 태그를 뽑는다.
	# jq 없이 sed만으로 처리해 의존성을 늘리지 않는다.
	fetch_stdout "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
		head -1
}

# --- 본체 ---

main() {
	need tar

	os=$(detect_os)
	arch=$(detect_arch)

	version="${VERSION:-}"
	if [ -z "$version" ]; then
		info "최신 버전 확인 중…"
		version=$(latest_version)
		[ -n "$version" ] || die "최신 버전을 찾지 못했습니다.
  아직 릴리스가 없거나 네트워크 문제일 수 있습니다.
  https://github.com/$REPO/releases 를 확인하세요."
	fi

	asset="${BIN}_${os}_${arch}.tar.gz"
	url="https://github.com/$REPO/releases/download/$version/$asset"

	say "notify-agent $version ($os/$arch)"

	tmp=$(mktemp -d)
	# 중간에 실패해도 임시 파일을 남기지 않는다.
	trap 'rm -rf "$tmp"' EXIT INT TERM

	info "내려받는 중…"
	fetch "$url" "$tmp/$asset" || die "받지 못했습니다: $url
  그 버전에 이 플랫폼 파일이 없을 수 있습니다."

	# 체크섬이 있으면 확인한다. 없어도 설치는 진행한다.
	if fetch "https://github.com/$REPO/releases/download/$version/checksums.txt" \
		"$tmp/checksums.txt" 2>/dev/null; then
		if command -v shasum >/dev/null 2>&1; then
			sum=$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)
		elif command -v sha256sum >/dev/null 2>&1; then
			sum=$(sha256sum "$tmp/$asset" | cut -d' ' -f1)
		else
			sum=""
		fi
		if [ -n "$sum" ]; then
			want=$(grep "$asset" "$tmp/checksums.txt" 2>/dev/null | cut -d' ' -f1 | head -1)
			if [ -n "$want" ] && [ "$sum" != "$want" ]; then
				die "체크섬이 맞지 않습니다. 받다 만 파일일 수 있습니다."
			fi
			[ -n "$want" ] && info "체크섬 확인됨"
		fi
	fi

	tar -xzf "$tmp/$asset" -C "$tmp" || die "압축을 풀지 못했습니다."
	[ -f "$tmp/$BIN" ] && [ -f "$tmp/scripts/install.sh" ] || die "압축 안에 $BIN 이나 scripts/install.sh 가 없습니다."
	chmod +x "$tmp/$BIN"

	# 묶음 안의 설치 스크립트가 실행 파일을 ~/.local/bin에 두고 LaunchAgent로 켠다.
	sh "$tmp/scripts/install.sh" "$@" || die "설치에 실패했습니다."
	[ "${1:-}" = "--uninstall" ] && return 0

	ok "설치됨: $HOME/.local/bin/$BIN ($version)"
	say ""
	say "다음 단계(한 번만):"
	say "  1. 시스템 설정 > 개인정보 보호 및 보안 > 전체 디스크 접근에 ~/.local/bin/$BIN 추가"
	say "  2. ~/.config/notify-agent/env 의 NOTIFY_APPS 정하기  ${c_dim}($BIN apps)${c_off}"
	say "  3. relay-service .env 에 위에 보인 RELAY_HOOK_KEY 를 넣고 다시 켜기"
	say ""
	say "${c_dim}자세한 설명: https://github.com/$REPO#설치${c_off}"
}

main "$@"
