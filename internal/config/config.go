package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/arkurl/quotui/internal/quote"
)

// AllColumns 是全部可展示的列，同时决定默认列顺序。
var AllColumns = []string{"code", "name", "price", "chg", "pct", "amount", "turnover"}

var sortFields = map[string]bool{"order": true, "pct": true, "amount": true, "code": true}

// Config 是 quotui 的全部本地配置。
type Config struct {
	RefreshSeconds int            `toml:"refresh_seconds"`
	ColorScheme    string         `toml:"color_scheme"`
	SortBy         string         `toml:"sort_by"`
	SortDesc       bool           `toml:"sort_desc"`
	Cursor         int            `toml:"cursor"`
	Columns        []string       `toml:"columns"`
	Holidays       []string       `toml:"holidays"`
	Watchlist      []quote.Symbol `toml:"watchlist"`
}

// DefaultPath 返回系统配置目录下的配置文件路径。
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "quotui", "config.toml"), nil
}

// Default 返回内置默认配置。
func Default() *Config {
	return &Config{
		RefreshSeconds: 3,
		ColorScheme:    "red_up",
		SortBy:         "order",
		SortDesc:       true,
		Columns:        append([]string(nil), AllColumns...),
		Watchlist: []quote.Symbol{
			{Code: "600519", Market: "sh", Name: "贵州茅台"},
			{Code: "000001", Market: "sz", Name: "平安银行"},
			{Code: "300750", Market: "sz", Name: "宁德时代"},
			{Code: "600036", Market: "sh", Name: "招商银行"},
			{Code: "000858", Market: "sz", Name: "五粮液"},
			{Code: "601318", Market: "sh", Name: "中国平安"},
		},
	}
}

// Load 读取配置；文件不存在时写入默认配置。
func Load(path string) (*Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if saveErr := Save(path, cfg); saveErr != nil {
				return nil, saveErr
			}
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	cfg.normalize()
	return cfg, nil
}

// Save 原子写入配置。
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := toml.NewEncoder(f)
	err = enc.Encode(cfg)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// cleanHolidays 校验并去重自定义休市日（YYYY-MM-DD）。
func cleanHolidays(dates []string) []string {
	seen := make(map[string]bool, len(dates))
	out := make([]string, 0, len(dates))
	for _, d := range dates {
		d = strings.TrimSpace(d)
		if _, err := time.Parse("2006-01-02", d); err != nil {
			continue
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// Learn 用行情结果补全自选股缺失的名称与市场编号，返回是否有改动。
// 市场编号很重要：美股的交易所前缀（105/106/107）无法从代码推断，
// 从备用源学到后写回，主源下次才能正确定位。
func (c *Config) Learn(quotes []quote.Quote) bool {
	if len(quotes) == 0 {
		return false
	}
	index := make(map[string]int, len(c.Watchlist))
	for i, s := range c.Watchlist {
		index[s.Key()] = i
	}

	dirty := false
	for _, q := range quotes {
		i, ok := index[q.Symbol.Key()]
		if !ok {
			continue
		}
		w := &c.Watchlist[i]
		if w.Name == "" && q.Name != "" {
			w.Name = q.Name
			dirty = true
		}
		if w.SecID == "" && q.Symbol.SecID != "" {
			w.SecID = q.Symbol.SecID
			dirty = true
		}
	}
	return dirty
}

// cleanColumns 去掉未知列与重复列，全部非法时回落到默认集合。
func cleanColumns(cols []string) []string {
	known := make(map[string]bool, len(AllColumns))
	for _, k := range AllColumns {
		known[k] = true
	}
	out := make([]string, 0, len(cols))
	seen := make(map[string]bool, len(cols))
	for _, k := range cols {
		if !known[k] || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	if len(out) == 0 {
		return append([]string(nil), AllColumns...)
	}
	return out
}

func (c *Config) normalize() {
	// 0 表示只手动刷新，不自动轮询。
	if c.RefreshSeconds < 0 {
		c.RefreshSeconds = 0
	}
	if c.RefreshSeconds > 3600 {
		c.RefreshSeconds = 3600
	}
	if c.ColorScheme != "green_up" {
		c.ColorScheme = "red_up"
	}
	if !sortFields[c.SortBy] {
		c.SortBy = "order"
	}
	c.Columns = cleanColumns(c.Columns)
	c.Holidays = cleanHolidays(c.Holidays)
	for i := range c.Watchlist {
		if c.Watchlist[i].Market == "" {
			c.Watchlist[i].Market = quote.InferMarket(c.Watchlist[i].Code)
		}
	}
}
