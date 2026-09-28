package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnvFileAndEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	_ = os.WriteFile(path, []byte("# 주석\nNOTIFY_HOOK_KEY='file-key'\nNOTIFY_APPS=com.a, com.b\nNOTIFY_POLL=10s\n"), 0o600)
	t.Setenv("NOTIFY_RELAY_URL", "http://10.0.1.50:4100")

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.HookKey != "file-key" || c.RelayURL != "http://10.0.1.50:4100" || c.Poll != 10*time.Second {
		t.Fatalf("%+v", c)
	}
	if !c.Allows("com.b") || c.Allows("com.c") {
		t.Fatalf("앱 목록: %v", c.Apps)
	}
	if !(Config{Apps: []string{"*"}}).Allows("anything") {
		t.Fatal("*는 전부")
	}
	if (Config{}).Allows("com.a") {
		t.Fatal("비어 있으면 아무것도 보내지 않는다")
	}
}

func TestBadPollIsRejected(t *testing.T) {
	t.Setenv("NOTIFY_POLL", "100ms")
	if _, err := Load(""); err == nil {
		t.Fatal("1초보다 짧은 간격은 받지 않는다")
	}
}
