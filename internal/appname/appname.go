// Package appname은 번들 id를 사람이 읽는 앱 이름으로 바꾼다.
//
// Spotlight(mdfind)로 앱을 찾아 Info.plist의 표시 이름을 읽는다. 맥에 없는
// 앱(아이폰 미러링으로 온 알림 등)은 번들 id의 끝 조각을 쓴다.
package appname

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"howett.net/plist"
)

var (
	mu    sync.Mutex
	cache = map[string]string{}
)

// Of는 번들 id의 앱 이름이다. 한 번 찾은 것은 기억한다.
func Of(bundleID string) string {
	mu.Lock()
	defer mu.Unlock()
	if name, ok := cache[bundleID]; ok {
		return name
	}
	name := lookup(bundleID)
	cache[bundleID] = name
	return name
}

func lookup(bundleID string) string {
	fallback := bundleID
	if i := strings.LastIndex(bundleID, "."); i >= 0 && i < len(bundleID)-1 {
		fallback = bundleID[i+1:]
	}
	if bundleID == "" {
		return "알림"
	}
	out, err := exec.Command("mdfind", "kMDItemCFBundleIdentifier == '"+strings.ReplaceAll(bundleID, "'", "")+"'").Output()
	if err != nil {
		return fallback
	}
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(path, "Contents", "Info.plist"))
		if err != nil {
			continue
		}
		var info struct {
			Display string `plist:"CFBundleDisplayName"`
			Name    string `plist:"CFBundleName"`
		}
		if _, err := plist.Unmarshal(data, &info); err != nil {
			continue
		}
		if info.Display != "" {
			return info.Display
		}
		if info.Name != "" {
			return info.Name
		}
	}
	return fallback
}
