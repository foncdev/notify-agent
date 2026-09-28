package main

import (
	"strings"
	"testing"
	"unicode"

	"github.com/foncdev/notify-agent/internal/appname"
	"github.com/foncdev/notify-agent/internal/lang"
	"github.com/foncdev/notify-agent/internal/ncdb"
)

func hasHangul(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.Is(unicode.Hangul, r) }) >= 0
}

// RELAY_LANG=en이면 안경·폰에 뜨는 시험 알림과 CLI 오류가 영어다.
func TestEnglishMessages(t *testing.T) {
	defer lang.Set(lang.EN)()
	m := TestMessage()
	if m.Title != "[notify-agent] Test notification" {
		t.Errorf("제목: %q", m.Title)
	}
	for _, s := range []string{m.Title, m.Body, ncdb.ErrNoAccess.Error(), appname.Of("")} {
		if s == "" || hasHangul(s) {
			t.Errorf("영어가 아니다: %q", s)
		}
	}
}

func TestKoreanIsDefault(t *testing.T) {
	defer lang.Set(lang.KO)()
	if m := TestMessage(); m.Title != "[notify-agent] 시험 알림" {
		t.Errorf("기본 한국어가 아니다: %q", m.Title)
	}
}
