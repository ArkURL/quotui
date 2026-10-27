package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arkurl/quotui/internal/config"
	"github.com/arkurl/quotui/internal/market"
	"github.com/arkurl/quotui/internal/provider"
	"github.com/arkurl/quotui/internal/provider/eastmoney"
	"github.com/arkurl/quotui/internal/provider/failover"
	"github.com/arkurl/quotui/internal/provider/tencent"
	"github.com/arkurl/quotui/internal/quote"
	"github.com/arkurl/quotui/internal/ui"
)

// version 由发布流程通过 -ldflags 注入；未注入时回退到模块版本，
// 这样 go install ...@v0.1.0 装出来的二进制也能报出真实版本。
var version = "dev"

func buildVersion() string {
	if version != "dev" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	v := bi.Main.Version
	// 只认正式的 tag 版本：伪版本（含 -0.）和本地改动（含 +dirty）都退回 dev。
	if v == "" || v == "(devel)" || strings.Contains(v, "+") || strings.Contains(v, "-0.") {
		return version
	}
	return strings.TrimPrefix(v, "v")
}

func main() {
	once := flag.Bool("once", false, "抓取一次自选股行情，以纯文本输出后退出")
	cfgPath := flag.String("config", "", "配置文件路径")
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	jsonOut := flag.Bool("json", false, "以 JSON 输出一次行情后退出（隐含 -once）")
	flag.Parse()

	if *showVersion {
		fmt.Println("quotui " + buildVersion())
		return
	}

	path := *cfgPath
	if path == "" {
		p, err := config.DefaultPath()
		if err != nil {
			fatal(err)
		}
		path = p
	}

	cfg, err := config.Load(path)
	if err != nil {
		fatal(err)
	}

	// 东财字段最全，腾讯作为备用源；任一可用即可。
	// 自定义休市日并入内置日历，用于判断是否休市。
	market.AddClosures(cfg.Holidays)
	ui.Version = buildVersion()

	prov := failover.New(eastmoney.New(), tencent.New())
	if *jsonOut {
		if err := runJSON(cfg, path, prov); err != nil {
			fatal(err)
		}
		return
	}
	if *once {
		if err := dumpOnce(cfg, path, prov); err != nil {
			fatal(err)
		}
		return
	}

	if _, err := tea.NewProgram(ui.New(cfg, path, prov), tea.WithAltScreen()).Run(); err != nil {
		fatal(err)
	}
}

// snapshot 是 -json 输出的顶层结构。
type snapshot struct {
	Updated string        `json:"updated"`
	Source  string        `json:"source"`
	Count   int           `json:"count"`
	Quotes  []snapshotRow `json:"quotes"`
	Missing []string      `json:"missing,omitempty"`
}

// snapshotRow 是一支标的的快照。price 用指针是为了停牌时能输出 null，
// 而不是和「真的跌到 0」混淆。
type snapshotRow struct {
	Code      string   `json:"code"`
	Market    string   `json:"market"`
	Name      string   `json:"name"`
	Price     *float64 `json:"price"`
	PrevClose float64  `json:"prev_close"`
	Open      float64  `json:"open"`
	High      float64  `json:"high"`
	Low       float64  `json:"low"`
	Change    float64  `json:"change"`
	ChangePct float64  `json:"change_pct"`
	Volume    float64  `json:"volume"`
	Amount    float64  `json:"amount"`
	Turnover  float64  `json:"turnover"`
	PE        float64  `json:"pe"`
	PB        float64  `json:"pb"`
	MarketCap float64  `json:"market_cap"`
	Suspended bool     `json:"suspended"`
	Decimals  int      `json:"decimals"`
}

// buildSnapshot 把行情结果整理成稳定可解析的结构。
func buildSnapshot(quotes []quote.Quote, requested []quote.Symbol, source string, at time.Time) snapshot {
	got := make(map[string]quote.Quote, len(quotes))
	rows := make([]snapshotRow, 0, len(quotes))
	for _, q := range quotes {
		got[q.Symbol.Key()] = q
		row := snapshotRow{
			Code:      q.Symbol.Code,
			Market:    q.Symbol.Market,
			Name:      q.Name,
			PrevClose: round4(q.PrevClose),
			Open:      round4(q.Open),
			High:      round4(q.High),
			Low:       round4(q.Low),
			Change:    round4(q.Change),
			ChangePct: round4(q.ChangePct),
			Volume:    q.Volume,
			Amount:    round2(q.Amount),
			Turnover:  round4(q.Turnover),
			PE:        round4(q.PE),
			PB:        round4(q.PB),
			MarketCap: round2(q.MarketCap),
			Suspended: q.Price == 0,
			Decimals:  q.Decimals,
		}
		if q.Price != 0 {
			price := round4(q.Price)
			row.Price = &price
		}
		rows = append(rows, row)
	}

	var missing []string
	for _, s := range requested {
		if _, ok := got[s.Key()]; !ok {
			missing = append(missing, s.Key())
		}
	}

	return snapshot{
		Updated: at.Format(time.RFC3339),
		Source:  source,
		Count:   len(rows),
		Quotes:  rows,
		Missing: missing,
	}
}

// sourceName 取稳定的数据源名；Chain 的 Name() 会带上熔断标记，不适合写进 JSON。
func sourceName(p provider.Provider) string {
	if a, ok := p.(interface{ Active() string }); ok {
		name := a.Active()
		if name != "" {
			return name
		}
	}
	return p.Name()
}

// round2/round4 去掉浮点乘除带来的尾数噪声（如 1687584999999.9998）。
func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

// runJSON 输出一次性快照的 JSON，供脚本消费。
func runJSON(cfg *config.Config, path string, prov provider.Provider) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	quotes, err := prov.Quotes(ctx, cfg.Watchlist)
	if err != nil && len(quotes) == 0 {
		return err
	}
	if cfg.Learn(quotes) {
		_ = config.Save(path, cfg)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(buildSnapshot(quotes, cfg.Watchlist, sourceName(prov), time.Now()))
}

func dumpOnce(cfg *config.Config, path string, prov provider.Provider) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	quotes, err := prov.Quotes(ctx, cfg.Watchlist)
	if err != nil && len(quotes) == 0 {
		return err
	}
	if cfg.Learn(quotes) {
		_ = config.Save(path, cfg)
	}
	fmt.Printf("%-8s %s %10s %10s %9s %10s\n", "代码", padName("名称", 12), "现价", "涨跌", "涨幅", "成交额")
	for _, q := range quotes {
		fmt.Printf("%-8s %s %10s %10s %9s %10s\n",
			q.Symbol.Code,
			padName(q.Name, 12),
			quote.FmtPrice(q.Price, q.Decimals),
			quote.FmtChange(q.Change, q.Decimals),
			quote.FmtPct(q.ChangePct),
			quote.FmtAmount(q.Amount),
		)
	}
	return err
}

// padName 按终端显示宽度左对齐（CJK 字符占两列）。
func padName(s string, w int) string {
	n := 0
	for _, r := range s {
		if r > utf8.RuneSelf && isWide(r) {
			n += 2
		} else {
			n++
		}
	}
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

func isWide(r rune) bool {
	switch {
	case r < 0x1100:
		return false
	case r >= 0x2E80 && r < 0xA4D0: // CJK 部首、汉字、假名等
		return true
	case r >= 0xAC00 && r < 0xD7A4: // 谚文
		return true
	case r >= 0xF900 && r < 0xFB00: // CJK 兼容
		return true
	case r >= 0xFE30 && r < 0xFE70:
		return true
	case r >= 0xFF00 && r < 0xFF61: // 全角
		return true
	case r >= 0x1F300 && r < 0x1F900: // emoji
		return true
	}
	return false
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "quotui:", err)
	os.Exit(1)
}
