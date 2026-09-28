package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/foncdev/notify-agent/internal/config"
	"github.com/foncdev/notify-agent/internal/ncdb"
)

func TestFormatPutsAppFirst(t *testing.T) {
	m, ok := Format(ncdb.Notification{Title: "엄마", Subtitle: "", Body: "밥 먹었니"}, "카카오톡")
	if !ok || m.Title != "[카카오톡] 엄마" || m.Body != "밥 먹었니" {
		t.Fatalf("%+v", m)
	}
	m, ok = Format(ncdb.Notification{Body: "제목 없는 알림"}, "메일")
	if !ok || m.Title != "메일" || m.Body != "제목 없는 알림" {
		t.Fatalf("제목이 없으면 앱 이름이 제목: %+v", m)
	}
	if _, ok := Format(ncdb.Notification{}, "x"); ok {
		t.Fatal("글이 없는 알림은 보내지 않는다")
	}
}

// fakeRelay는 훅으로 온 알림을 모은다. fail이면 500을 준다.
type fakeRelay struct {
	mu    sync.Mutex
	got   []map[string]string
	keys  []string
	paths []string
	fail  bool
}

func (f *fakeRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		w.WriteHeader(500)
		return
	}
	var m map[string]string
	_ = json.NewDecoder(r.Body).Decode(&m)
	f.got = append(f.got, m)
	f.keys = append(f.keys, r.Header.Get("X-Hook-Key"))
	f.paths = append(f.paths, r.URL.Path)
	w.WriteHeader(201)
}

func TestForwardSendsOnlyAllowedAppsAndStopsOnFailure(t *testing.T) {
	path := writeFixture(t)
	fake := &fakeRelay{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := config.Config{RelayURL: srv.URL, HookKey: "hook-key", Apps: []string{"com.kakao.KakaoTalkMac"}, Poll: time.Second}
	reader := ncdb.Reader{Path: path}

	last, err := forward(cfg, reader, 0)
	if err != nil {
		t.Fatal(err)
	}
	if last != 3 {
		t.Fatalf("끝까지 처리해야 한다: %d", last)
	}
	if len(fake.got) != 2 {
		t.Fatalf("카카오톡 두 건만 보내야 한다: %+v", fake.got)
	}
	if fake.keys[0] != "hook-key" || fake.paths[0] != "/hooks/notify/mac" {
		t.Fatalf("훅 키·경로: %v %v", fake.keys, fake.paths)
	}

	// relay-service가 받지 않으면 거기서 멈춰 다음에 다시 보낸다.
	fake.fail = true
	fake.got = nil
	last, err = forward(cfg, reader, 0)
	if err == nil || last != 0 {
		t.Fatalf("실패하면 앞 순번에 멈춰야 한다: %d %v", last, err)
	}
}

func TestNothingAllowedSendsNothing(t *testing.T) {
	path := writeFixture(t)
	fake := &fakeRelay{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	last, err := forward(config.Config{RelayURL: srv.URL, HookKey: "k"}, ncdb.Reader{Path: path}, 0)
	if err != nil || last != 3 || len(fake.got) != 0 {
		t.Fatalf("앱을 정하지 않으면 아무것도 보내지 않는다: %d %v %v", last, err, fake.got)
	}
}

// writeFixture는 ncdb 테스트와 같은 모양의 알림 기록을 만든다(카카오톡 2, 슬랙 1).
func writeFixture(t *testing.T) string {
	t.Helper()
	return ncdbFixture(t, filepath.Join(t.TempDir(), "db"))
}
