//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package monitor

import (
	"os"
	"os/exec"
	"testing"
)

func TestLockHelper(t *testing.T) {
	if os.Getenv("SMART_MONEY_LOCK_HELPER") != "1" {
		return
	}
	release, err := AcquireLock(os.Getenv("SMART_MONEY_LOCK_DIR"))
	if err != nil {
		os.Exit(23)
	}
	defer release()
}

func TestCrossProcessLockAndRelease(t *testing.T) {
	dir := t.TempDir()
	release, err := AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	child := func() error {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelper$")
		cmd.Env = append(os.Environ(), "SMART_MONEY_LOCK_HELPER=1", "SMART_MONEY_LOCK_DIR="+dir)
		return cmd.Run()
	}
	err = child()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
		t.Fatalf("second process acquired lock: %v", err)
	}
	release()
	if err = child(); err != nil {
		t.Fatalf("lock not released: %v", err)
	}
	// 子进程退出后，父进程可再次获取同一锁文件。
	releaseAgain, err := AcquireLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	releaseAgain()
}
