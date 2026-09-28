package main

import (
	"database/sql"
	"testing"

	"howett.net/plist"
	_ "modernc.org/sqlite"
)

func ncdbFixture(t *testing.T, path string) string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(q string, args ...any) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE app (app_id INTEGER PRIMARY KEY, identifier VARCHAR)`)
	exec(`CREATE TABLE record (rec_id INTEGER PRIMARY KEY, app_id INTEGER, data BLOB)`)
	exec(`INSERT INTO app VALUES (1, 'com.kakao.KakaoTalkMac'), (2, 'com.tinyspeck.slackmacgap')`)
	for i, r := range []struct {
		app, title, body string
		appID             int
	}{
		{"com.kakao.KakaoTalkMac", "엄마", "밥 먹었니", 1},
		{"com.tinyspeck.slackmacgap", "#dev", "배포 끝", 2},
		{"com.kakao.KakaoTalkMac", "팀장", "회의 5분 전", 1},
	} {
		data, err := plist.Marshal(map[string]any{"app": r.app, "req": map[string]any{"titl": r.title, "body": r.body}}, plist.BinaryFormat)
		if err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO record (rec_id, app_id, data) VALUES (?, ?, ?)`, i+1, r.appID, data)
	}
	return path
}
