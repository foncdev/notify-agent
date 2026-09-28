package state

import (
	"path/filepath"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "state.json")
	if _, ok, err := Load(path); ok || err != nil {
		t.Fatalf("처음에는 없다: %v %v", ok, err)
	}
	if err := Save(path, State{LastID: 42}); err != nil {
		t.Fatal(err)
	}
	s, ok, err := Load(path)
	if !ok || err != nil || s.LastID != 42 {
		t.Fatalf("%+v %v %v", s, ok, err)
	}
}
