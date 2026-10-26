package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arkurl/quotui/internal/config"
	"github.com/arkurl/quotui/internal/provider"
	"github.com/arkurl/quotui/internal/provider/eastmoney"
	"github.com/arkurl/quotui/internal/provider/failover"
	"github.com/arkurl/quotui/internal/provider/tencent"
	"github.com/arkurl/quotui/internal/quote"
	"github.com/arkurl/quotui/internal/ui"
)

func main() {
	once := flag.Bool("once", false, "抓取一次自选股行情，以纯文本输出后退出")
	cfgPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

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
	prov := failover.New(eastmoney.New(), tencent.New())
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
