package ui

import (
	"strings"
	"testing"

	"github.com/arkurl/quotui/internal/quote"
)

func colKeys(cols []column) []string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, c.key)
	}
	return out
}

func TestColumnsFollowConfig(t *testing.T) {
	m := newTestModel(t, 120, 24)
	m.cfg.Columns = []string{"code", "pct"}
	got := colKeys(m.columns())
	if len(got) != 2 || got[0] != "code" || got[1] != "pct" {
		t.Errorf("应按配置产出列，得到 %v", got)
	}
}

func TestColumnsDroppedOnNarrow(t *testing.T) {
	m := newTestModel(t, 60, 24)
	cols := m.columns()
	if lineWidth(cols) > m.width {
		t.Errorf("窄屏下总宽 %d 超出 %d", lineWidth(cols), m.width)
	}
	for _, key := range []string{"turnover", "amount"} {
		for _, c := range cols {
			if c.key == key {
				t.Errorf("宽度 %d 时应先丢掉 %s，实际列 %v", m.width, key, colKeys(cols))
			}
		}
	}
	if len(cols) < 3 {
		t.Errorf("不应裁到只剩 %v", colKeys(cols))
	}
}

func TestLimitUpMarker(t *testing.T) {
	m := newTestModel(t, 120, 24)
	s := m.cfg.Watchlist[0] // 沪市主板，10% 涨停
	m.quotes[s.Key()] = quote.Quote{
		Symbol: s, Name: s.Name, Price: 11, Change: 1, ChangePct: 10, Decimals: 2,
	}
	out := m.View()
	if !strings.Contains(out, "+10.00%↑") {
		t.Errorf("涨停应带箭头标记:\n%s", out)
	}

	m.quotes[s.Key()] = quote.Quote{
		Symbol: s, Name: s.Name, Price: 9, Change: -1, ChangePct: -10, Decimals: 2,
	}
	if out := m.View(); !strings.Contains(out, "-10.00%↓") {
		t.Errorf("跌停应带箭头标记:\n%s", out)
	}
}
