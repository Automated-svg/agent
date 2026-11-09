package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"smart-money-agent/internal/monitor"
)

func main() {
	if err := execute(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func execute() error {
	const usage = "用法: smart-money-agent <init|check|run|daemon|report> [-config config.json] [-date YYYY-MM-DD]"
	if len(os.Args) < 2 {
		return fmt.Errorf("%s", usage)
	}
	command := os.Args[1]
	if command == "--help" || command == "-h" || command == "help" {
		fmt.Println(usage)
		return nil
	}
	if command != "init" && command != "check" && command != "run" && command != "daemon" && command != "report" {
		return fmt.Errorf("未知命令 %q", command)
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	configPath := fs.String("config", "config.json", "配置文件路径")
	date := fs.String("date", "", "日报日期，默认配置时区的昨天（run/report）")
	if err := fs.Parse(os.Args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("不支持额外位置参数")
	}
	if *date != "" && command != "run" && command != "report" {
		return fmt.Errorf("%s 不接受 -date", command)
	}
	if command == "init" {
		f, err := os.OpenFile(*configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.WriteString(monitor.ExampleConfig)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Println("配置已创建。请填写各链 wallets 或删除不用的链，并设置 ETHERSCAN_API_KEY。")
		return nil
	}
	c, err := monitor.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(c.DataDir) {
		abs, err := filepath.Abs(*configPath)
		if err != nil {
			return err
		}
		c.DataDir = filepath.Join(filepath.Dir(abs), c.DataDir)
	}
	if *date == "" {
		loc, _ := time.LoadLocation(c.Timezone)
		*date = time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	}
	if command == "report" {
		r, err := monitor.Read(c, *date)
		if err != nil {
			return err
		}
		return monitor.Markdown(os.Stdout, r)
	}
	key := strings.TrimSpace(os.Getenv(c.APIKeyEnv))
	if key == "" {
		return fmt.Errorf("请设置环境变量 %s", c.APIKeyEnv)
	}
	if command == "check" {
		wallets := 0
		for _, chain := range c.Chains {
			wallets += len(chain.Wallets)
		}
		fmt.Printf("本地检查通过：%d 条链、%d 个钱包配置；每日 %s (%s)；数据目录 %s。密钥环境变量已设置，未验证 API 权限。\n", len(c.Chains), wallets, c.DailyAt, c.Timezone, c.DataDir)
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := monitor.NewClient(c, key)
	if command == "run" {
		release, err := monitor.AcquireLock(c.DataDir)
		if err != nil {
			return err
		}
		defer release()
		return monitor.Run(ctx, c, client, *date)
	}
	return monitor.Daemon(ctx, c, client)
}
