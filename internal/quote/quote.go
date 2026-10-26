package quote

import (
	"fmt"
	"strings"
	"time"
)

// Symbol 是自选股条目的规范化表示。
// SecID 对应东方财富的 secid（市场.代码），用于直接请求行情；
// 缺省时会按 Market + Code 推断。
type Symbol struct {
	Code   string `toml:"code" json:"code"`
	Market string `toml:"market" json:"market"`
	SecID  string `toml:"secid,omitempty" json:"secid,omitempty"`
	Name   string `toml:"name,omitempty" json:"name,omitempty"`
}

// ID 返回唯一的 secid，形如 "1.600519"、"116.00700"、"105.AAPL"。
func (s Symbol) ID() string {
	if s.SecID != "" {
		return s.SecID
	}
	c := s.Code
	switch s.Market {
	case "sh":
		return "1." + c
	case "sz", "bj":
		return "0." + c
	case "hk":
		return "116." + padCode(c, 5)
	case "us":
		// 美股的交易所编号（105/106/107）无法从代码推断，
		// 正常流程由搜索接口带回；这里退化为纳斯达克。
		return "105." + strings.ToUpper(c)
	}
	if m := InferMarket(c); m != "" {
		return Symbol{Code: c, Market: m}.ID()
	}
	return "0." + c
}

// Key 是跨数据源稳定的唯一键：市场 + 代码（美股大小写不敏感）。
// 用它做本地索引，而不是 secid —— 不同数据源的市场编号并不一致。
func (s Symbol) Key() string {
	return s.Market + "/" + strings.ToLower(s.Code)
}

// Quote 是一支股票的实时快照。
type Quote struct {
	Symbol    Symbol
	Name      string
	Price     float64
	PrevClose float64
	Open      float64
	High      float64
	Low       float64
	Change    float64
	ChangePct float64
	Volume    float64 // 手
	Amount    float64 // 元
	Turnover  float64 // 换手率 %
	PE        float64
	PB        float64
	VolRatio  float64
	MarketCap float64
	Decimals  int
	Suspended bool
}

// Point 是分时序列上的一个采样点。
type Point struct {
	At    time.Time
	Price float64
	Avg   float64 // 均价，部分数据源不提供时为零值
}

// Intraday 是某只标的当日的分时序列。
type Intraday struct {
	Symbol    Symbol
	Name      string
	PrevClose float64
	Decimals  int
	Date      string
	Points    []Point
}

// K 线周期。
const (
	PeriodDay   = "day"
	PeriodWeek  = "week"
	PeriodMonth = "month"
	Period60Min = "60m"
)

// PeriodLabel 返回周期的中文名。
func PeriodLabel(p string) string {
	switch p {
	case PeriodWeek:
		return "周K"
	case PeriodMonth:
		return "月K"
	case Period60Min:
		return "60分"
	}
	return "日K"
}

// Candle 是一根 K 线。
type Candle struct {
	At     time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	Pct    float64
}

// Kline 是一段 K 线序列（按时间升序）。
type Kline struct {
	Symbol   Symbol
	Period   string
	Decimals int
	Candles  []Candle
}

// InferMarket 按代码形态猜测市场，仅用于手工输入。
func InferMarket(code string) string {
	if !isDigits(code) {
		return "us"
	}
	switch {
	case len(code) == 6 && (code[0] == '6' || code[0] == '9'):
		return "sh"
	case len(code) == 6 && (code[0] == '0' || code[0] == '3'):
		return "sz"
	case len(code) == 6 && (code[0] == '4' || code[0] == '8'):
		return "bj"
	case len(code) <= 5:
		return "hk"
	}
	return "us"
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func padCode(c string, n int) string {
	if !isDigits(c) || len(c) >= n {
		return c
	}
	return strings.Repeat("0", n-len(c)) + c
}

// LimitPct 返回涨跌停幅度（百分比）。无法判断（如港美股）时返回 0。
func LimitPct(code, name string) float64 {
	if strings.Contains(strings.ToUpper(name), "ST") {
		return 5
	}
	if len(code) != 6 || !isDigits(code) {
		return 0
	}
	switch {
	case strings.HasPrefix(code, "688"), strings.HasPrefix(code, "689"):
		return 20 // 科创板
	case strings.HasPrefix(code, "300"), strings.HasPrefix(code, "301"):
		return 20 // 创业板
	case code[0] == '4' || code[0] == '8':
		return 30 // 北交所
	}
	return 10
}

// FmtPrice 按接口给出的小数位格式化价格。
func FmtPrice(v float64, dec int) string {
	if dec < 0 || dec > 4 {
		dec = 2
	}
	return fmt.Sprintf("%.*f", dec, v)
}

func FmtChange(v float64, dec int) string {
	if dec < 0 || dec > 4 {
		dec = 2
	}
	return fmt.Sprintf("%+.*f", dec, v)
}

func FmtPct(v float64) string {
	return fmt.Sprintf("%+.2f%%", v)
}

func FmtAmount(v float64) string {
	switch {
	case v >= 1e8:
		return fmt.Sprintf("%.2f亿", v/1e8)
	case v >= 1e4:
		return fmt.Sprintf("%.2f万", v/1e4)
	case v > 0:
		return fmt.Sprintf("%.0f", v)
	}
	return "--"
}

func FmtPctPlain(v float64) string {
	if v <= 0 {
		return "--"
	}
	return fmt.Sprintf("%.2f%%", v)
}
