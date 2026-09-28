// Package config는 설정을 읽는다.
//
// 환경 변수가 먼저고, 없으면 설정 파일(~/.config/notify-agent/env)의 값을 쓴다.
// 설정 파일은 KEY=값 줄의 모음이다. 훅 키가 들어가므로 권한 600으로 둔다.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	// RelayURL은 relay-service 주소. 같은 맥이면 http://127.0.0.1:4100.
	RelayURL string
	// HookKey는 relay-service의 RELAY_HOOK_KEY.
	HookKey string
	// Apps는 가져올 앱의 번들 id. 비어 있으면 아무것도 보내지 않는다 — 개인
	// 메시지가 서버에 쌓이므로, 무엇을 보낼지는 사람이 정해야 한다. "*"는 전부.
	Apps []string
	// Poll은 알림 기록을 들여다보는 간격.
	Poll time.Duration
	// StatePath는 어디까지 보냈는지 적는 파일.
	StatePath string
	// DBPath는 알림 기록 파일. 비우면 macOS 기본 자리.
	DBPath string
}

// DefaultEnvFile은 설정 파일 자리다.
func DefaultEnvFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "notify-agent", "env")
}

// Load는 설정 파일과 환경 변수를 읽는다. envFile이 없어도 된다.
func Load(envFile string) (Config, error) {
	file, err := readEnvFile(envFile)
	if err != nil {
		return Config{}, err
	}
	get := func(key, def string) string {
		if v, ok := os.LookupEnv(key); ok {
			return strings.TrimSpace(v)
		}
		if v, ok := file[key]; ok {
			return v
		}
		return def
	}

	home, _ := os.UserHomeDir()
	c := Config{
		RelayURL:  get("NOTIFY_RELAY_URL", "http://127.0.0.1:4100"),
		HookKey:   get("NOTIFY_HOOK_KEY", ""),
		StatePath: get("NOTIFY_STATE", filepath.Join(home, "Library", "Application Support", "notify-agent", "state.json")),
		DBPath:    get("NOTIFY_DB", ""),
	}
	for _, a := range strings.Split(get("NOTIFY_APPS", ""), ",") {
		if a = strings.TrimSpace(a); a != "" {
			c.Apps = append(c.Apps, a)
		}
	}
	poll, err := time.ParseDuration(get("NOTIFY_POLL", "5s"))
	if err != nil || poll < time.Second {
		return Config{}, fmt.Errorf("NOTIFY_POLL이 잘못됐습니다(1s 이상, 예: 5s): %q", get("NOTIFY_POLL", ""))
	}
	c.Poll = poll
	return c, nil
}

// Allows는 이 앱의 알림을 보내도 되는지.
func (c Config) Allows(app string) bool {
	for _, a := range c.Apps {
		if a == "*" || a == app {
			return true
		}
	}
	return false
}

func readEnvFile(path string) (map[string]string, error) {
	out := map[string]string{}
	if path == "" {
		return out, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out, sc.Err()
}
