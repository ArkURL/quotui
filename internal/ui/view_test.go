package ui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/arkurl/quotui/internal/config"
	"github.com/arkurl/quotui/internal/quote"
)

type fakeProvider struct{}

func (fakeProvider) Name() string { return "fake" }
func (fakeProvider) Quotes(context.Context, []quote.Symbol) ([]quote.Quote, error) {
	return nil, nil
}
func (fakeProvider) Search(context.Context, string, int) ([]quote.Symbol, error) {
	return nil, nil
}
func (fakeProvider) Intraday(context.Context, quote.Symbol) (quote.Intraday, error) {
	return quote.Intraday{}, nil
}
func (fakeProvider) Kline(context.Context, quote.Symbol, string, int) (quote.Kline, error) {
	return quote.Kline{}, nil
}

func newTestModel(t *testing.T, w, h int) *Model {
	t.Helper()
	cfg := config.Default()
	m := New(cfg, filepath.Join(t.TempDir(), "config.toml"), fakeProvider{})
	m.width, m.height = w, h
	// 固定用东八区，否则在 UTC 机器上 10:00 会落在 A 股收盘后，测试结果随时区漂移。
	m.now = time.Date(2026, 10, 26, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	for i, s := range cfg.Watchlist {
		m.quotes[s.Key()] = quote.Quote{
			Symbol:    s,
			Name:      s.Name,
			Price:     10 + float64(i),
			Change:    0.12,
			ChangePct: 1.23,
			Amount:    1.5e8,
			Turnover:  1.5,
			Decimals:  2,
		}
	}
	return m
}

func assertFits(t *testing.T, m *Model, out string) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) != m.height {
		t.Fatalf("行数 %d，期望 %d", len(lines), m.height)
	}
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w > m.width {
			t.Errorf("第 %d 行宽度 %d 超出 %d: %q", i, w, m.width, ln)
		}
	}
}

func TestListViewFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{120, 24}, {90, 20}, {70, 15}, {50, 12}} {
		m := newTestModel(t, size[0], size[1])
		out := m.View()
		assertFits(t, m, out)
		if !strings.Contains(out, "+1.23%") {
			t.Errorf("宽 %d 时未渲染出涨幅: %q", size[0], out)
		}
	}
}

func TestHelpAndAddViewFit(t *testing.T) {
	m := newTestModel(t, 100, 24)
	m.showHelp = true
	assertFits(t, m, m.View())

	m.showHelp = false
	m.add.active = true
	m.add.results = []quote.Symbol{
		{Code: "00700", Market: "hk", Name: "腾讯控股", SecID: "116.00700"},
		{Code: "AAPL", Market: "us", Name: "苹果", SecID: "105.AAPL"},
	}
	assertFits(t, m, m.View())
}

func TestFitText(t *testing.T) {
	if got := fitText("贵州茅台", 8); got != "贵州茅台" {
		t.Errorf("刚好放得下时不应截断，得到 %q", got)
	}
	if got := fitText("贵州茅台酒股份有限公司", 8); lipgloss.Width(got) > 8 {
		t.Errorf("截断后宽度超限: %q", got)
	}
	if got := fitText("abcdef", 4); got != "abc…" {
		t.Errorf("期望 abc…，得到 %q", got)
	}
}

func TestHelpShowsVersion(t *testing.T) {
	old := Version
	Version = "9.9.9"
	t.Cleanup(func() { Version = old })

	m := newTestModel(t, 80, 24)
	m.showHelp = true
	if !strings.Contains(m.View(), "quotui 9.9.9") {
		t.Errorf("帮助页应显示版本号:\n%s", m.View())
	}
}
