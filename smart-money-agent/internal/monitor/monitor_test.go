package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const walletAddress = "0x1111111111111111111111111111111111111111"
const otherAddress = "0x2222222222222222222222222222222222222222"
const tokenAddress = "0x3333333333333333333333333333333333333333"

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{Timezone: "Asia/Shanghai", DailyAt: "08:00", DataDir: t.TempDir(), APIKeyEnv: "TEST_KEY", RequestIntervalMS: 200, PageSize: 100, MaxPages: 3, CatchupDays: 2, Chains: []Chain{{Name: "ethereum", ID: "1", NativeSymbol: "ETH", Wallets: []Wallet{{Label: "test", Address: walletAddress}}}}}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mockClient(c Config, handler func(*http.Request) string) *Client {
	client := NewClient(c, "secret-test-key")
	client.interval = 0
	client.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(handler(r))), Header: make(http.Header)}, nil
	})}
	return client
}
func success(v any) string {
	b, _ := json.Marshal(v)
	return `{"status":"1","message":"OK","result":` + string(b) + `}`
}

func TestExactFlowAndSelfTransfer(t *testing.T) {
	c := testConfig(t)
	transfers := []Transfer{
		{ChainID: "1", Wallet: walletAddress, Asset: tokenAddress, Symbol: "TOKEN", Decimals: 18, RawValue: "123456789012345678901234567890", Direction: "in"},
		{ChainID: "1", Wallet: walletAddress, Asset: tokenAddress, Symbol: "TOKEN", Decimals: 18, RawValue: "1", Direction: "out"},
		{ChainID: "1", Wallet: walletAddress, Asset: tokenAddress, Symbol: "TOKEN", Decimals: 18, RawValue: "999", Direction: "self"},
	}
	flows, err := aggregate(transfers, c.Chains)
	if err != nil || len(flows) != 1 {
		t.Fatalf("flows=%v err=%v", flows, err)
	}
	if flows[0].Net != "123456789012.345678901234567889" || flows[0].Count != 2 {
		t.Fatalf("incorrect flow: %+v", flows[0])
	}
	if got := decimal(big.NewInt(-1), 18); got != "-0.000000000000000001" {
		t.Fatal(got)
	}
	if got := decimal(big.NewInt(100), 0); got != "100" {
		t.Fatal(got)
	}
	transfers[1].Decimals = 6
	if _, err := aggregate(transfers, c.Chains); err == nil {
		t.Fatal("expected inconsistent decimals error")
	}
}

func TestConvertBoundariesAndFailure(t *testing.T) {
	c := testConfig(t)
	start, end, _ := c.Window("2026-01-01")
	base := transaction{Hash: "tx", Timestamp: fmt.Sprint(start.Unix()), From: otherAddress, To: walletAddress, Value: "42", IsError: "0"}
	for _, tc := range []struct {
		name   string
		ts     int64
		failed bool
		want   bool
	}{
		{"before", start.Unix() - 1, false, false}, {"start", start.Unix(), false, true}, {"last", end.Unix() - 1, false, true}, {"end", end.Unix(), false, false}, {"failed", start.Unix(), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			row.Timestamp = fmt.Sprint(tc.ts)
			if tc.failed {
				row.IsError = "1"
			}
			got, err := convert(row, c.Chains[0], c.Chains[0].Wallets[0], "txlist", start, end)
			if err != nil || (got != nil) != tc.want {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}

func TestPagination(t *testing.T) {
	c := testConfig(t)
	c.PageSize = 1
	client := mockClient(c, func(r *http.Request) string {
		if r.URL.Query().Get("page") == "1" {
			return success([]transaction{{Hash: "one"}})
		}
		return `{"status":"0","message":"No transactions found","result":[]}`
	})
	rows, err := client.transactions(context.Background(), "1", walletAddress, "tokentx", "0", "99")
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	client = mockClient(c, func(*http.Request) string { return success([]transaction{{Hash: "one"}}) })
	if _, err = client.transactions(context.Background(), "1", walletAddress, "tokentx", "0", "99"); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("expected repeated page error, got %v", err)
	}
	c.MaxPages = 1
	client = mockClient(c, func(*http.Request) string { return success([]transaction{{Hash: "one"}}) })
	if _, err = client.transactions(context.Background(), "1", walletAddress, "tokentx", "0", "99"); err == nil || !strings.Contains(err.Error(), "max_pages") {
		t.Fatalf("expected truncation error, got %v", err)
	}
}

func TestCollectSaveReadAndFailedRunPreservesReport(t *testing.T) {
	c := testConfig(t)
	date := "2026-01-01"
	start, _, _ := c.Window(date)
	client := mockClient(c, func(r *http.Request) string {
		switch r.URL.Query().Get("action") {
		case "getblocknobytime":
			return success("123")
		case "txlist":
			return success([]transaction{{Hash: "native", Timestamp: fmt.Sprint(start.Unix()), From: walletAddress, To: otherAddress, Value: "1000000000000000000", IsError: "0"}})
		default:
			// 同一交易中两条完全相同的 Transfer 也应保留，不能按 hash 去重。
			row := transaction{Hash: "token", Timestamp: fmt.Sprint(start.Unix()), From: otherAddress, To: walletAddress, Contract: tokenAddress, Symbol: "USDC", Decimals: "6", Value: "1234567"}
			return success([]transaction{row, row})
		}
	})
	if err := Run(context.Background(), c, client, date); err != nil {
		t.Fatal(err)
	}
	r, err := Read(c, date)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Transfers) != 3 || len(r.Flows) != 2 {
		t.Fatalf("incorrect report: %+v", r)
	}
	for _, f := range r.Flows {
		if f.Asset == tokenAddress && f.Net != "2.469134" {
			t.Fatal(f)
		}
	}
	path := filepath.Join(c.DataDir, date+".json")
	before, _ := os.ReadFile(path)
	client = mockClient(c, func(*http.Request) string {
		return `{"status":"0","message":"NOTOK","result":"secret-test-key invalid"}`
	})
	err = Run(context.Background(), c, client, date)
	if err == nil || strings.Contains(err.Error(), "secret-test-key") {
		t.Fatalf("error or key redaction failed: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed run overwrote valid report")
	}
	c.Chains[0].Wallets[0].Address = otherAddress
	if _, err := Read(c, date); err == nil {
		t.Fatal("configuration change should invalidate report")
	}
	if _, err := Read(c, "../escape"); err == nil {
		t.Fatal("expected invalid date rejection")
	}
}

func TestScheduleAndDST(t *testing.T) {
	c := testConfig(t)
	for _, tc := range []struct{ now, want string }{{"2026-01-03T07:59:00+08:00", "2026-01-01"}, {"2026-01-03T08:00:00+08:00", "2026-01-02"}} {
		now, _ := time.Parse(time.RFC3339, tc.now)
		dates := DueDates(c, now)
		if len(dates) != 2 || dates[1] != tc.want {
			t.Fatal(dates)
		}
	}
	c.Timezone = "America/New_York"
	start, end, err := c.Window("2026-03-08")
	if err != nil || end.Sub(start) != 23*time.Hour {
		t.Fatalf("DST window %v %v", end.Sub(start), err)
	}
}

func TestValidateAndCancellation(t *testing.T) {
	c := testConfig(t)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Chains[0].Wallets = append(c.Chains[0].Wallets, c.Chains[0].Wallets[0])
	if err := c.Validate(); err == nil {
		t.Fatal("duplicate wallet accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pause(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestRangeSplitDiscardsParentRows(t *testing.T) {
	for _, reason := range []string{"full", "timeout", "window"} {
		t.Run(reason, func(t *testing.T) {
			c := testConfig(t)
			c.PageSize = 2
			c.MaxPages = 1
			var ranges []string
			client := mockClient(c, func(r *http.Request) string {
				q := r.URL.Query()
				span := q.Get("startblock") + "-" + q.Get("endblock")
				ranges = append(ranges, span)
				switch span {
				case "0-3":
					if reason == "timeout" {
						return `{"status":"0","result":"Query Timeout occured"}`
					}
					if reason == "window" {
						return `{"status":"0","result":"Result window is too large"}`
					}
					return success([]transaction{{Hash: "parent1"}, {Hash: "parent2"}})
				case "0-1":
					return success([]transaction{{Hash: "left"}})
				case "2-3":
					return success([]transaction{{Hash: "right"}})
				default:
					t.Fatalf("unexpected range %s", span)
					return ""
				}
			})
			rows, err := client.transactions(context.Background(), "1", walletAddress, "tokentx", "0", "3")
			if err != nil || len(rows) != 2 || rows[0].Hash != "left" || rows[1].Hash != "right" {
				t.Fatalf("rows=%v err=%v", rows, err)
			}
			if strings.Join(ranges, ",") != "0-3,0-1,2-3" {
				t.Fatal(ranges)
			}
		})
	}
}

func TestSplitChildFailureReturnsNoPartialRows(t *testing.T) {
	c := testConfig(t)
	c.PageSize = 2
	c.MaxPages = 1
	client := mockClient(c, func(r *http.Request) string {
		q := r.URL.Query()
		if q.Get("startblock") == q.Get("endblock") {
			if q.Get("startblock") == "0" {
				return success([]transaction{{Hash: "left"}})
			}
			return `{"status":"0","result":"Invalid API Key"}`
		}
		return success([]transaction{{Hash: "one"}, {Hash: "two"}})
	})
	rows, err := client.transactions(context.Background(), "1", walletAddress, "tokentx", "0", "1")
	if err == nil || len(rows) != 0 {
		t.Fatalf("partial result escaped: %v %v", rows, err)
	}
}

func TestProcessDueContinuesAfterFailure(t *testing.T) {
	c := testConfig(t)
	now, _ := time.Parse(time.RFC3339, "2026-01-03T08:00:00+08:00")
	var calls []string
	run := func(date string) error {
		calls = append(calls, date)
		if date == "2026-01-01" {
			return errors.New("temporary failure")
		}
		return Save(c, Report{Version: 1, Date: date, Fingerprint: c.Fingerprint()})
	}
	failed, err := processDue(context.Background(), c, now, run)
	if !failed || err != nil || strings.Join(calls, ",") != "2026-01-01,2026-01-02" {
		t.Fatalf("calls=%v failed=%v err=%v", calls, failed, err)
	}
	calls = nil
	_, err = processDue(context.Background(), c, now, run)
	if err != nil || strings.Join(calls, ",") != "2026-01-01" {
		t.Fatalf("completed day should be skipped: %v %v", calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = nil
	_, err = processDue(ctx, c, now, run)
	if !errors.Is(err, context.Canceled) || len(calls) != 0 {
		t.Fatalf("cancellation failed: %v %v", calls, err)
	}
}

func TestReadRejectsTrailingGarbage(t *testing.T) {
	c := testConfig(t)
	date := "2026-01-01"
	if err := Save(c, Report{Version: 1, Date: date, Fingerprint: c.Fingerprint()}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(c.DataDir, date+".json"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("garbage")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Read(c, date); err == nil {
		t.Fatal("accepted corrupted report")
	}
}

func TestCancelledRequestDoesNotReachTransport(t *testing.T) {
	c := testConfig(t)
	client := mockClient(c, func(*http.Request) string { t.Fatal("request sent after cancellation"); return "" })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.transactions(ctx, "1", walletAddress, "tokentx", "0", "3")
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
