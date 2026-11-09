package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Transfer struct {
	ChainID   string `json:"chain_id"`
	Wallet    string `json:"wallet"`
	Hash      string `json:"hash"`
	Timestamp int64  `json:"timestamp"`
	From      string `json:"from"`
	To        string `json:"to"`
	Asset     string `json:"asset"`
	Symbol    string `json:"symbol"`
	Decimals  int    `json:"decimals"`
	RawValue  string `json:"raw_value"`
	Direction string `json:"direction"`
}
type Flow struct {
	ChainID  string `json:"chain_id"`
	Wallet   string `json:"wallet"`
	Label    string `json:"label"`
	Asset    string `json:"asset"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
	Incoming string `json:"incoming"`
	Outgoing string `json:"outgoing"`
	Net      string `json:"net"`
	Count    int    `json:"transfer_count"`
}
type Report struct {
	Version     int        `json:"version"`
	Date        string     `json:"date"`
	Timezone    string     `json:"timezone"`
	Fingerprint string     `json:"config_fingerprint"`
	GeneratedAt time.Time  `json:"generated_at"`
	Chains      []Chain    `json:"chains"`
	Transfers   []Transfer `json:"transfers"`
	Flows       []Flow     `json:"flows"`
}

func convert(row transaction, chain Chain, wallet Wallet, action string, start, end time.Time) (*Transfer, error) {
	ts, err := strconv.ParseInt(row.Timestamp, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("转账时间无效")
	}
	if ts < start.Unix() || ts >= end.Unix() {
		return nil, nil
	}
	if action == "txlist" {
		if row.IsError == "1" || row.ReceiptStatus == "0" {
			return nil, nil
		}
		if row.IsError != "0" {
			return nil, fmt.Errorf("原生币交易缺少执行状态")
		}
	}
	value, ok := new(big.Int).SetString(row.Value, 10)
	if !ok || value.Sign() < 0 {
		return nil, fmt.Errorf("转账金额无效")
	}
	if value.Sign() == 0 {
		return nil, nil
	}
	from, to, addr := strings.ToLower(row.From), strings.ToLower(row.To), strings.ToLower(wallet.Address)
	if row.Hash == "" || !addressPattern.MatchString(from) {
		return nil, fmt.Errorf("转账标识或发送地址无效")
	}
	// 合约创建的原生币 value 转入新合约。
	if to == "" && action == "txlist" {
		to = strings.ToLower(row.Contract)
	}
	if !addressPattern.MatchString(to) {
		return nil, fmt.Errorf("转账接收地址无效")
	}
	direction := "out"
	if from == addr && to == addr {
		direction = "self"
	} else if to == addr {
		direction = "in"
	} else if from != addr {
		return nil, fmt.Errorf("API 返回了无关钱包的转账")
	}
	asset, symbol, decimals := "native", chain.NativeSymbol, 18
	if action == "tokentx" {
		asset, symbol = strings.ToLower(row.Contract), row.Symbol
		if !addressPattern.MatchString(asset) {
			return nil, fmt.Errorf("代币合约地址无效")
		}
		decimals, err = strconv.Atoi(row.Decimals)
		if err != nil || decimals < 0 || decimals > 255 {
			return nil, fmt.Errorf("代币精度无效")
		}
	}
	return &Transfer{chain.ID, addr, row.Hash, ts, from, to, asset, symbol, decimals, value.String(), direction}, nil
}

func decimal(n *big.Int, places int) string {
	s := new(big.Int).Abs(n).String()
	if places > 0 {
		if len(s) <= places {
			s = strings.Repeat("0", places-len(s)+1) + s
		}
		s = s[:len(s)-places] + "." + s[len(s)-places:]
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if n.Sign() < 0 {
		s = "-" + s
	}
	return s
}

func aggregate(transfers []Transfer, chains []Chain) ([]Flow, error) {
	type bucket struct {
		flow    Flow
		in, out *big.Int
	}
	labels := map[string]string{}
	for _, c := range chains {
		for _, w := range c.Wallets {
			labels[c.ID+":"+strings.ToLower(w.Address)] = w.Label
		}
	}
	buckets := map[string]*bucket{}
	for _, t := range transfers {
		if t.Direction == "self" {
			continue
		}
		key := t.ChainID + ":" + t.Wallet + ":" + t.Asset
		b := buckets[key]
		if b == nil {
			b = &bucket{flow: Flow{ChainID: t.ChainID, Wallet: t.Wallet, Label: labels[t.ChainID+":"+t.Wallet], Asset: t.Asset, Symbol: t.Symbol, Decimals: t.Decimals}, in: new(big.Int), out: new(big.Int)}
			buckets[key] = b
		}
		if b.flow.Decimals != t.Decimals {
			return nil, fmt.Errorf("同一合约的代币精度不一致")
		}
		n, ok := new(big.Int).SetString(t.RawValue, 10)
		if !ok {
			return nil, fmt.Errorf("金额无效")
		}
		if t.Direction == "in" {
			b.in.Add(b.in, n)
		} else {
			b.out.Add(b.out, n)
		}
		b.flow.Count++
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	flows := make([]Flow, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		b.flow.Incoming = decimal(b.in, b.flow.Decimals)
		b.flow.Outgoing = decimal(b.out, b.flow.Decimals)
		b.flow.Net = decimal(new(big.Int).Sub(b.in, b.out), b.flow.Decimals)
		flows = append(flows, b.flow)
	}
	return flows, nil
}

func Collect(ctx context.Context, c Config, client *Client, date string) (Report, error) {
	start, end, err := c.Window(date)
	if err != nil {
		return Report{}, err
	}
	if end.After(time.Now()) {
		return Report{}, fmt.Errorf("仅允许采集已结束的自然日")
	}
	r := Report{Version: 1, Date: date, Timezone: c.Timezone, Fingerprint: c.Fingerprint(), GeneratedAt: time.Now().UTC(), Chains: c.Chains, Transfers: []Transfer{}}
	for _, chain := range c.Chains {
		first, err := client.blockAt(ctx, chain.ID, start)
		if err != nil {
			return Report{}, fmt.Errorf("%s 开始区块: %w", chain.Name, err)
		}
		last, err := client.blockAt(ctx, chain.ID, end)
		if err != nil {
			return Report{}, fmt.Errorf("%s 结束区块: %w", chain.Name, err)
		}
		for _, wallet := range chain.Wallets {
			for _, action := range []string{"txlist", "tokentx"} {
				rows, err := client.transactions(ctx, chain.ID, wallet.Address, action, first, last)
				if err != nil {
					return Report{}, fmt.Errorf("%s/%s/%s: %w", chain.Name, wallet.Address, action, err)
				}
				for _, row := range rows {
					t, err := convert(row, chain, wallet, action, start, end)
					if err != nil {
						return Report{}, err
					}
					if t != nil {
						r.Transfers = append(r.Transfers, *t)
					}
				}
			}
		}
	}
	r.Flows, err = aggregate(r.Transfers, c.Chains)
	return r, err
}

func reportPath(c Config, date string) (string, error) {
	if _, _, err := c.Window(date); err != nil {
		return "", err
	}
	return filepath.Join(c.DataDir, date+".json"), nil
}

func Save(c Config, r Report) error {
	path, err := reportPath(c, r.Date)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(c.DataDir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(c.DataDir, ".report-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(r); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func Read(c Config, date string) (Report, error) {
	path, err := reportPath(c, date)
	if err != nil {
		return Report{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer f.Close()
	var r Report
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&r)
	if err == nil {
		if trailing := decoder.Decode(new(any)); trailing != io.EOF {
			err = fmt.Errorf("日报包含无效尾部数据，请重新运行 run")
		}
	}
	if err == nil && (r.Version != 1 || r.Date != date || r.Fingerprint != c.Fingerprint()) {
		err = fmt.Errorf("日报与当前配置不匹配，请重新运行 run")
	}
	return r, err
}

func Markdown(w io.Writer, r Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# 聪明钱日报 %s\n\n时区：%s；监控 %d 条链，%d 条钱包转账记录。\n\n", r.Date, r.Timezone, len(r.Chains), len(r.Transfers))
	b.WriteString("金额为代币数量；净流入 = 流入 − 流出。原生币仅含普通交易 value，不含内部转账及 Gas。\n\n")
	b.WriteString("| 链 ID | 钱包标签 / 地址 | 资产 / 合约 | 流入 | 流出 | 净流入 | 笔数 |\n| --- | --- | --- | ---: | ---: | ---: | ---: |\n")
	clean := strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ", "<", "&lt;", ">", "&gt;")
	for _, f := range r.Flows {
		fmt.Fprintf(&b, "| %s | %s / %s | %s / %s | %s | %s | %s | %d |\n", f.ChainID, clean.Replace(f.Label), f.Wallet, clean.Replace(f.Symbol), f.Asset, f.Incoming, f.Outgoing, f.Net, f.Count)
	}
	if len(r.Flows) == 0 {
		b.WriteString("\n该日无非零、非自身转账。\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
