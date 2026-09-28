// Package state는 어디까지 보냈는지(마지막 알림 순번)를 파일에 적는다.
//
// 다시 켜도 같은 알림을 두 번 보내지 않고, 꺼져 있던 사이 온 알림은 보낸다.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type State struct {
	LastID int64 `json:"lastId"`
}

// Load는 적어 둔 것을 읽는다. 없으면 ok가 false다.
func Load(path string) (s State, ok bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, false, err
	}
	return s, true, nil
}

// Save는 원자적으로 적는다. 적다 꺼져도 앞의 값이 남는다.
func Save(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
