package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"smart-money-agent/internal/monitor"
)

func TestCheckAndHelp(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"agent", "--help"}
	if err := execute(); err != nil {
		t.Fatal(err)
	}
	var c monitor.Config
	if err := json.Unmarshal([]byte(monitor.ExampleConfig), &c); err != nil {
		t.Fatal(err)
	}
	c.Chains = c.Chains[:1]
	c.Chains[0].Wallets = []monitor.Wallet{{Address: "0x1111111111111111111111111111111111111111"}}
	c.APIKeyEnv = "SMART_MONEY_CLI_TEST_KEY"
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	b, _ := json.Marshal(c)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"agent", "check", "-config", path}
	t.Setenv(c.APIKeyEnv, "")
	if err := execute(); err == nil || !strings.Contains(err.Error(), c.APIKeyEnv) {
		t.Fatalf("missing key check: %v", err)
	}
	t.Setenv(c.APIKeyEnv, "fake-key-no-network")
	if err := execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "data")); !os.IsNotExist(err) {
		t.Fatal("check should not create data directory")
	}
	os.Args = append(os.Args, "-date", "2026-01-01")
	if err := execute(); err == nil {
		t.Fatal("check accepted unsupported date")
	}
}
