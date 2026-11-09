package monitor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	http               *http.Client
	endpoint           string
	key                string
	interval           time.Duration
	last               time.Time
	pageSize, maxPages int
}

func NewClient(c Config, key string) *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: "https://api.etherscan.io/v2/api", key: key, interval: time.Duration(c.RequestIntervalMS) * time.Millisecond, pageSize: c.PageSize, maxPages: c.MaxPages}
}

func pause(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Client) request(ctx context.Context, params url.Values) (json.RawMessage, error) {
	q := make(url.Values, len(params)+1)
	for k, v := range params {
		q[k] = append([]string(nil), v...)
	}
	q.Set("apikey", c.key)
	for attempt := 0; attempt < 4; attempt++ {
		if err := pause(ctx, time.Until(c.last.Add(c.interval))); err != nil {
			return nil, err
		}
		c.last = time.Now()
		req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("无法创建 API 请求")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt == 3 {
				return nil, fmt.Errorf("API 网络请求失败（已重试；URL 已隐藏以保护密钥）")
			}
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
			resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK {
				var envelope struct {
					Status  string          `json:"status"`
					Message string          `json:"message"`
					Result  json.RawMessage `json:"result"`
				}
				if json.Unmarshal(body, &envelope) != nil {
					return nil, fmt.Errorf("API 返回无效 JSON")
				}
				if envelope.Status == "1" && len(envelope.Result) > 0 && string(envelope.Result) != "null" {
					return envelope.Result, nil
				}
				if envelope.Status == "0" && envelope.Message == "No transactions found" && string(envelope.Result) == "[]" {
					return json.RawMessage("[]"), nil
				}
				// API 错误原文可能包含 URL 或密钥，不写入日志。
				message := strings.ToLower(string(envelope.Result))
				if strings.Contains(message, "query timeout") || strings.Contains(message, "result window is too large") {
					return nil, errRangeTooLarge
				}
				if !strings.Contains(message, "rate limit") {
					return nil, fmt.Errorf("API 拒绝 %s 请求；请检查密钥、链套餐权限及查询范围", params.Get("action"))
				}
			} else if resp.StatusCode != 429 && resp.StatusCode < 500 && readErr == nil {
				return nil, fmt.Errorf("API HTTP %d", resp.StatusCode)
			}
		}
		if attempt < 3 {
			if err := pause(ctx, time.Duration(1<<attempt)*time.Second); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("API 重试次数耗尽")
}

func (c *Client) blockAt(ctx context.Context, chain string, at time.Time) (string, error) {
	raw, err := c.request(ctx, url.Values{"chainid": {chain}, "module": {"block"}, "action": {"getblocknobytime"}, "timestamp": {strconv.FormatInt(at.Unix(), 10)}, "closest": {"before"}})
	if err != nil {
		return "", err
	}
	var block string
	if json.Unmarshal(raw, &block) != nil {
		return "", fmt.Errorf("API 区块格式错误")
	}
	if _, err := strconv.ParseUint(block, 10, 64); err != nil {
		return "", fmt.Errorf("API 区块编号错误")
	}
	return block, nil
}

type transaction struct {
	Hash          string `json:"hash"`
	Timestamp     string `json:"timeStamp"`
	From          string `json:"from"`
	To            string `json:"to"`
	Value         string `json:"value"`
	Contract      string `json:"contractAddress"`
	Symbol        string `json:"tokenSymbol"`
	Decimals      string `json:"tokenDecimal"`
	IsError       string `json:"isError"`
	ReceiptStatus string `json:"txreceipt_status"`
}

func (c *Client) transactions(ctx context.Context, chain, address, action, first, last string) ([]transaction, error) {
	lo, err := strconv.ParseUint(first, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("开始区块无效")
	}
	hi, err := strconv.ParseUint(last, 10, 64)
	if err != nil || lo > hi {
		return nil, fmt.Errorf("结束区块无效")
	}
	return c.transactionsRange(ctx, chain, address, action, lo, hi)
}

var errRangeTooLarge = errors.New("查询范围超过分页容量")

func (c *Client) transactionsRange(ctx context.Context, chain, address, action string, lo, hi uint64) ([]transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := c.transactionsPage(ctx, chain, address, action, strconv.FormatUint(lo, 10), strconv.FormatUint(hi, 10))
	if !errors.Is(err, errRangeTooLarge) {
		return rows, err
	}
	if lo == hi {
		return nil, fmt.Errorf("单个区块 %d 仍超出 max_pages 或 API 查询容量，无法保证完整性", lo)
	}
	// 放弃父范围的部分结果；两个子范围不重叠，避免重复计数。
	mid := lo + (hi-lo)/2
	left, err := c.transactionsRange(ctx, chain, address, action, lo, mid)
	if err != nil {
		return nil, err
	}
	right, err := c.transactionsRange(ctx, chain, address, action, mid+1, hi)
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

func (c *Client) transactionsPage(ctx context.Context, chain, address, action, first, last string) ([]transaction, error) {
	var result []transaction
	seenPages := map[[32]byte]bool{}
	for page := 1; page <= c.maxPages; page++ {
		// Etherscan 历史查询窗口最多 10000 条；满窗后改为拆分区块。
		if page*c.pageSize > 10000 {
			return nil, errRangeTooLarge
		}
		raw, err := c.request(ctx, url.Values{"chainid": {chain}, "module": {"account"}, "action": {action}, "address": {address}, "startblock": {first}, "endblock": {last}, "page": {strconv.Itoa(page)}, "offset": {strconv.Itoa(c.pageSize)}, "sort": {"asc"}})
		if err != nil {
			return nil, err
		}
		var rows []transaction
		if json.Unmarshal(raw, &rows) != nil || string(raw) == "null" {
			return nil, fmt.Errorf("API 转账列表格式错误")
		}
		if len(rows) > c.pageSize {
			return nil, fmt.Errorf("API 返回超出分页大小")
		}
		digest := sha256.Sum256(raw)
		if len(rows) > 0 && seenPages[digest] {
			return nil, fmt.Errorf("API 重复返回分页，停止以避免重复统计")
		}
		seenPages[digest] = true
		result = append(result, rows...)
		if len(rows) < c.pageSize {
			return result, nil
		}
	}
	return nil, errRangeTooLarge
}
