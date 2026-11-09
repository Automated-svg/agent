package monitor

import (
	"context"
	"fmt"
	"log"
	"time"
)

// 每日截止时间之前，昨天的日报尚未到采集时间。
func DueDates(c Config, now time.Time) []string {
	loc, _ := time.LoadLocation(c.Timezone)
	now = now.In(loc)
	clock, _ := time.Parse("15:04", c.DailyAt)
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), clock.Hour(), clock.Minute(), 0, 0, loc)
	latest := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -1)
	if now.Before(cutoff) {
		latest = latest.AddDate(0, 0, -1)
	}
	dates := make([]string, 0, c.CatchupDays)
	for i := c.CatchupDays - 1; i >= 0; i-- {
		dates = append(dates, latest.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return dates
}

func Run(ctx context.Context, c Config, client *Client, date string) error {
	r, err := Collect(ctx, c, client, date)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = Save(c, r); err != nil {
		return err
	}
	log.Printf("日报已保存：%s，%d 条转账，%d 条资金流", date, len(r.Transfers), len(r.Flows))
	return nil
}

func Daemon(ctx context.Context, c Config, client *Client) error {
	release, err := AcquireLock(c.DataDir)
	if err != nil {
		return err
	}
	defer release()
	log.Printf("每日 %s (%s) 采集前一天数据；补采窗口 %d 天", c.DailyAt, c.Timezone, c.CatchupDays)
	for {
		failed, err := processDue(ctx, c, time.Now(), func(date string) error { return Run(ctx, c, client, date) })
		if err != nil {
			return err
		}
		delay := time.Minute
		if failed {
			delay = 5 * time.Minute
		}
		if err := pause(ctx, delay); err != nil {
			return fmt.Errorf("调度结束: %w", err)
		}
	}
}

// 单日失败不阻塞其他日期；所有日期尝试后再等待重试。
func processDue(ctx context.Context, c Config, now time.Time, run func(string) error) (bool, error) {
	failed := false
	for _, date := range DueDates(c, now) {
		if err := ctx.Err(); err != nil {
			return failed, err
		}
		if _, err := Read(c, date); err == nil {
			continue
		}
		if err := run(date); err != nil {
			if ctx.Err() != nil {
				return failed, ctx.Err()
			}
			failed = true
			log.Printf("%s 采集失败，本轮其他日期完成后 5 分钟重试：%v", date, err)
		}
	}
	return failed, nil
}
