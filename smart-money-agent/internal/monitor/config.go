package monitor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Wallet struct {
	Label   string `json:"label"`
	Address string `json:"address"`
}
type Chain struct {
	Name         string   `json:"name"`
	ID           string   `json:"chain_id"`
	NativeSymbol string   `json:"native_symbol"`
	Wallets      []Wallet `json:"wallets"`
}
type Config struct {
	Timezone          string  `json:"timezone"`
	DailyAt           string  `json:"daily_at"`
	DataDir           string  `json:"data_dir"`
	APIKeyEnv         string  `json:"api_key_env"`
	RequestIntervalMS int     `json:"request_interval_ms"`
	PageSize          int     `json:"page_size"`
	MaxPages          int     `json:"max_pages"`
	CatchupDays       int     `json:"catchup_days"`
	Chains            []Chain `json:"chains"`
}

var addressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func LoadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	var c Config
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return c, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return c, fmt.Errorf("配置必须是单个 JSON 对象")
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return err
	}
	if _, err := time.Parse("15:04", c.DailyAt); err != nil {
		return fmt.Errorf("daily_at 必须是 HH:MM: %w", err)
	}
	if c.DataDir == "" || c.APIKeyEnv == "" {
		return fmt.Errorf("data_dir 和 api_key_env 不能为空")
	}
	if c.RequestIntervalMS < 200 || c.PageSize < 1 || c.PageSize > 1000 || c.MaxPages < 1 || c.MaxPages > 1000 || c.CatchupDays < 1 || c.CatchupDays > 365 {
		return fmt.Errorf("配置范围错误：间隔 >=200ms，page_size 1..1000，max_pages 1..1000，catchup_days 1..365")
	}
	if len(c.Chains) == 0 {
		return fmt.Errorf("至少配置一条链")
	}
	seen := map[string]bool{}
	for _, chain := range c.Chains {
		id, err := strconv.ParseUint(chain.ID, 10, 64)
		if err != nil || id == 0 || strconv.FormatUint(id, 10) != chain.ID || chain.Name == "" || chain.NativeSymbol == "" || seen[chain.ID] {
			return fmt.Errorf("链配置无效或 chain_id 重复: %q", chain.ID)
		}
		seen[chain.ID] = true
		if len(chain.Wallets) == 0 {
			return fmt.Errorf("链 %s 尚未配置聪明钱地址", chain.Name)
		}
		addresses := map[string]bool{}
		for _, w := range chain.Wallets {
			a := strings.ToLower(w.Address)
			if !addressPattern.MatchString(a) || addresses[a] || a == "0x0000000000000000000000000000000000000000" {
				return fmt.Errorf("链 %s 地址无效或重复: %s", chain.Name, w.Address)
			}
			addresses[a] = true
		}
	}
	return nil
}

// 采集范围变化后，旧日报不作为当前配置已完成的依据。
func (c Config) Fingerprint() string {
	b, _ := json.Marshal(struct {
		Version  int
		Timezone string
		Chains   []Chain
	}{1, c.Timezone, c.Chains})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (c Config) Window(date string) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, start.AddDate(0, 0, 1), nil
}

const ExampleConfig = `{
  "timezone": "Asia/Shanghai",
  "daily_at": "08:00",
  "data_dir": "data",
  "api_key_env": "ETHERSCAN_API_KEY",
  "request_interval_ms": 550,
  "page_size": 1000,
  "max_pages": 10,
  "catchup_days": 7,
  "chains": [
    {"name": "ethereum", "chain_id": "1", "native_symbol": "ETH", "wallets": []},
    {"name": "base", "chain_id": "8453", "native_symbol": "ETH", "wallets": []},
    {"name": "arbitrum", "chain_id": "42161", "native_symbol": "ETH", "wallets": []},
    {"name": "optimism", "chain_id": "10", "native_symbol": "ETH", "wallets": []},
    {"name": "polygon", "chain_id": "137", "native_symbol": "POL", "wallets": []},
    {"name": "bnb", "chain_id": "56", "native_symbol": "BNB", "wallets": []}
  ]
}
`
