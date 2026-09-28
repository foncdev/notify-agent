package ncdb

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"howett.net/plist"
)

// fixture는 macOS 알림 기록과 같은 모양의 SQLite를 만든다(app·record 표, data는 바이너리 plist).
func fixture(t *testing.T, recs ...map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mustExec(t, db, `CREATE TABLE app (app_id INTEGER PRIMARY KEY, identifier VARCHAR)`)
	mustExec(t, db, `CREATE TABLE record (rec_id INTEGER PRIMARY KEY, app_id INTEGER, uuid BLOB, data BLOB, delivered_date REAL)`)
	mustExec(t, db, `INSERT INTO app VALUES (1, 'com.kakao.KakaoTalkMac'), (2, 'com.tinyspeck.slackmacgap')`)
	for i, r := range recs {
		data, err := plist.Marshal(r, plist.BinaryFormat)
		if err != nil {
			t.Fatal(err)
		}
		app := 1
		if r["app"] == "com.tinyspeck.slackmacgap" {
			app = 2
		}
		mustExec(t, db, `INSERT INTO record (rec_id, app_id, data) VALUES (?, ?, ?)`, i+1, app, data)
	}
	return path
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}

func rec(app, title, body string) map[string]any {
	return map[string]any{
		"app":  app,
		"date": 800000000.0,
		"req":  map[string]any{"titl": title, "body": body},
	}
}

func TestAfterReadsNewOnesInOrder(t *testing.T) {
	path := fixture(t,
		rec("com.kakao.KakaoTalkMac", "엄마", "밥 먹었니"),
		rec("com.tinyspeck.slackmacgap", "#dev", "배포 끝"),
		rec("com.kakao.KakaoTalkMac", "팀장", "회의 5분 전"),
	)
	r := Reader{Path: path}

	got, err := r.After(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("1 뒤의 두 건이어야 한다: %+v", got)
	}
	if got[1].App != "com.kakao.KakaoTalkMac" || got[1].Title != "팀장" || got[1].Body != "회의 5분 전" {
		t.Fatalf("내용을 잘못 읽었다: %+v", got[1])
	}
	want := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC).Add(800000000 * time.Second)
	if !got[0].At.Equal(want) {
		t.Fatalf("시각: %v", got[0].At)
	}

	last, err := r.LastID()
	if err != nil || last != 3 {
		t.Fatalf("마지막 순번 %d, %v", last, err)
	}

	apps, err := r.Apps()
	if err != nil || len(apps) != 2 || apps[0].App != "com.kakao.KakaoTalkMac" || apps[0].Count != 2 {
		t.Fatalf("앱별 수: %+v %v", apps, err)
	}
}

func TestMissingFileSaysSo(t *testing.T) {
	_, err := Reader{Path: filepath.Join(t.TempDir(), "none")}.After(0)
	if err == nil {
		t.Fatal("없는 파일인데 오류가 없다")
	}
}

func TestBrokenRecordIsSkippedNotFatal(t *testing.T) {
	path := fixture(t, rec("com.kakao.KakaoTalkMac", "a", "b"))
	db, _ := sql.Open("sqlite", "file:"+path)
	mustExec(t, db, `INSERT INTO record (rec_id, app_id, data) VALUES (2, 1, x'00ff')`)
	db.Close()

	got, err := Reader{Path: path}.After(0)
	if err != nil || len(got) != 2 || got[1].Title != "" {
		t.Fatalf("깨진 한 건은 글 없이 순번만 남아야 한다: %+v %v", got, err)
	}
}
