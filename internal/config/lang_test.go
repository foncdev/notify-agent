package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/foncdev/notify-agent/internal/lang"
)

// 설정 파일의 RELAY_LANG을 따른다. 환경 변수가 있으면 그것이 먼저다.
func TestLoadAppliesLanguageFromFile(t *testing.T) {
	defer lang.Set("")()
	t.Setenv("RELAY_LANG", "")
	os.Unsetenv("RELAY_LANG")

	path := filepath.Join(t.TempDir(), "env")
	_ = os.WriteFile(path, []byte("RELAY_LANG=en\nNOTIFY_POLL=1ms\n"), 0o600)

	_, err := Load(path)
	if lang.Current() != lang.EN {
		t.Fatal("설정 파일의 RELAY_LANG=en을 따르지 않았다")
	}
	if err == nil || strings.IndexFunc(err.Error(), func(r rune) bool { return unicode.Is(unicode.Hangul, r) }) >= 0 {
		t.Fatalf("영어 오류가 아니다: %v", err)
	}

	t.Setenv("RELAY_LANG", "ko")
	_, _ = Load(path)
	if lang.Current() != lang.KO {
		t.Fatal("환경 변수가 설정 파일보다 먼저여야 한다")
	}
}
