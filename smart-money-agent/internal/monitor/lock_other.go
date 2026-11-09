//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !dragonfly

package monitor

import "fmt"

func AcquireLock(string) (func(), error) {
	return nil, fmt.Errorf("当前平台暂不支持采集进程锁，请在 Linux、macOS 或 BSD 上运行采集")
}
