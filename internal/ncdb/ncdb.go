// Package ncdb는 macOS 알림 센터의 기록을 읽는다.
//
// 알림은 usernoted가 SQLite 파일 하나에 쌓는다. 애플이 공개한 형식이 아니라
// macOS 판마다 자리·모양이 바뀔 수 있다. macOS 15부터는
// ~/Library/Group Containers/group.com.apple.usernoted/db2/db 에 있고,
// 읽으려면 이 프로그램에 '전체 디스크 접근' 권한이 있어야 한다.
//
// usernoted가 쓰는 중인 파일(WAL 모드)을 직접 열면 잠금이 얽힐 수 있어,
// 매번 db·db-wal·db-shm을 임시 폴더에 복사해 그 복사본을 읽는다.
package ncdb

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"howett.net/plist"
	_ "modernc.org/sqlite"
)

// Notification은 알림 하나다.
type Notification struct {
	// ID는 기록의 순번(rec_id)이다. 늘기만 하므로 어디까지 읽었는지 적는 데 쓴다.
	ID       int64
	App      string // 번들 id. 예: com.kakao.KakaoTalkMac
	Title    string
	Subtitle string
	Body     string
	At       time.Time
}

// DefaultPath는 macOS 15 이후의 알림 기록 자리다.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Group Containers", "group.com.apple.usernoted", "db2", "db")
}

// ErrNoAccess는 전체 디스크 접근 권한이 없어 읽지 못했을 때다.
var ErrNoAccess = errors.New("알림 기록을 읽을 수 없습니다. 시스템 설정 > 개인정보 보호 및 보안 > 전체 디스크 접근에서 notify-agent를 허용하세요")

// Reader는 알림 기록을 읽는다.
type Reader struct {
	Path string
}

// After는 id보다 뒤에 쌓인 알림을 순서대로 돌려준다.
func (r Reader) After(id int64) ([]Notification, error) {
	db, cleanup, err := r.open()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	rows, err := db.Query(`
		SELECT r.rec_id, COALESCE(a.identifier, ''), r.data
		FROM record r LEFT JOIN app a ON a.app_id = r.app_id
		WHERE r.rec_id > ?
		ORDER BY r.rec_id`, id)
	if err != nil {
		return nil, fmt.Errorf("알림 기록의 모양이 예상과 다릅니다(macOS가 바뀌었을 수 있다): %w", err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var (
			n    Notification
			app  string
			data []byte
		)
		if err := rows.Scan(&n.ID, &app, &data); err != nil {
			return nil, err
		}
		if err := decode(data, &n); err != nil {
			// 한 건이 깨졌다고 나머지를 버리지 않는다. 순번만 남겨 건너뛴다.
			n.Title = ""
		}
		if n.App == "" {
			n.App = app
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// LastID는 지금 쌓인 마지막 순번이다. 처음 켤 때 여기서부터 읽어 지난 알림이
// 한꺼번에 쏟아지지 않게 한다.
func (r Reader) LastID() (int64, error) {
	db, cleanup, err := r.open()
	if err != nil {
		return 0, err
	}
	defer cleanup()
	var id sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(rec_id) FROM record`).Scan(&id); err != nil {
		return 0, fmt.Errorf("알림 기록의 모양이 예상과 다릅니다: %w", err)
	}
	return id.Int64, nil
}

// AppCount는 앱마다 쌓인 알림 수다. 어떤 앱을 가져올지 고를 때 본다.
type AppCount struct {
	App   string
	Count int
}

// Apps는 알림을 보낸 앱과 그 수를 많은 순으로 돌려준다.
func (r Reader) Apps() ([]AppCount, error) {
	db, cleanup, err := r.open()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	rows, err := db.Query(`
		SELECT COALESCE(a.identifier, ''), COUNT(*)
		FROM record r LEFT JOIN app a ON a.app_id = r.app_id
		GROUP BY a.identifier ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, fmt.Errorf("알림 기록의 모양이 예상과 다릅니다: %w", err)
	}
	defer rows.Close()
	var out []AppCount
	for rows.Next() {
		var c AppCount
		if err := rows.Scan(&c.App, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// open은 알림 기록을 임시 폴더에 복사해 연다. 다 쓰면 cleanup을 부른다.
func (r Reader) open() (*sql.DB, func(), error) {
	path := r.Path
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, nil, ErrNoAccess
		}
		return nil, nil, fmt.Errorf("알림 기록이 없습니다(%s): %w", path, err)
	}

	dir, err := os.MkdirTemp("", "notify-agent-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	// db가 먼저, 그다음 WAL. 복사하는 사이 새 알림이 들어와도 다음 번에 읽힌다.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := copyFile(path+suffix, filepath.Join(dir, "db"+suffix)); err != nil {
			if suffix != "" && errors.Is(err, os.ErrNotExist) {
				continue // WAL이 없을 수도 있다.
			}
			cleanup()
			if errors.Is(err, os.ErrPermission) {
				return nil, nil, ErrNoAccess
			}
			return nil, nil, err
		}
	}

	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "db"))
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return db, func() {
		_ = db.Close()
		cleanup()
	}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// record는 기록 한 건의 data(바이너리 plist)다. 쓰는 것만 읽는다.
type record struct {
	App  string  `plist:"app"`
	Date float64 `plist:"date"`
	Req  struct {
		Title    string `plist:"titl"`
		Subtitle string `plist:"subt"`
		Body     string `plist:"body"`
	} `plist:"req"`
}

// macOS 기준 시각(2001-01-01 UTC)부터 센 초를 time으로 바꾼다.
var appleEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

func decode(data []byte, n *Notification) error {
	var rec record
	if _, err := plist.Unmarshal(data, &rec); err != nil {
		return err
	}
	n.App = rec.App
	n.Title = rec.Req.Title
	n.Subtitle = rec.Req.Subtitle
	n.Body = rec.Req.Body
	if rec.Date > 0 {
		n.At = appleEpoch.Add(time.Duration(rec.Date * float64(time.Second)))
	}
	return nil
}
