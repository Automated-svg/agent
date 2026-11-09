//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// AcquireLock 持有内核文件锁，进程退出（包括崩溃）时自动释放。
// 锁文件始终保留，删除它会使其他进程锁住不同 inode。
func AcquireLock(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".collector.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("无法锁定数据目录（可能已有 run/daemon 进程）：%w", err)
	}
	return func() { _ = f.Close() }, nil
}
