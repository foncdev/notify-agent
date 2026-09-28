// Package relay는 알림을 relay-service의 알림 훅으로 보낸다.
//
// 훅(POST /hooks/notify/<출처>)은 전용 키(X-Hook-Key)로만 열리고 알림 추가
// 하나밖에 못 한다. 그래서 이 프로그램은 마스터 키를 모른다.
package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/foncdev/notify-agent/internal/lang"
)

// Client는 relay-service 알림 훅에 보낸다.
type Client struct {
	BaseURL string // 예: http://127.0.0.1:4100
	HookKey string
	Source  string // 알림 끝에 남는 출처. 기본 mac
	HTTP    *http.Client
}

// Message는 relay-service 알림 하나다.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

// Send는 알림 하나를 보낸다.
func (c Client) Send(m Message) error {
	source := c.Source
	if source == "" {
		source = "mac"
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/hooks/notify/" + source
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hook-Key", c.HookKey)

	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf(lang.L("relay-service에 보내지 못했습니다: %w", "Couldn't send to relay-service: %w"), err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf(lang.L("relay-service가 받지 않았습니다(%d): %s", "relay-service rejected it (%d): %s"), res.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
