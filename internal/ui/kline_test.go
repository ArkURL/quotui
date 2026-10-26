package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/arkurl/quotui/internal/config"
	"github.com/arkurl/quotui/internal/quote"
)

func makeCandles(n int) []quote.Candle {
	out := make([]quote.Candle, 0, n)
	day := time.Date(2026, 1, 5, 0, 0, 0, 0, time.Local)
	for i := 0; i < n; i++ {
		open := 100 + float64(i)
		close := open + float64(i%5) - 2
		out = append(out, quote.Candle{
			At:     day.AddDate(0, 0, i),
			Open:   open,
			Close:  close,
			High:   open + 3,
			Low:    open - 3,
			Volume: 1000,
		})
	}
	return out
}

func TestToggleKlineMode(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.chartMode != chartIntraday {
		t.Fatal("默认应是分时")
	}

	upd, cmd := m.Update(runeKey('c'))
	m = upd.(*Model)
	if m.chartMode != chartKline {
		t.Fatal("c 应切到 K 线")
	}
	if cmd == nil {
		t.Error("切到 K 线应触发请求")
	}

	upd, cmd = m.Update(runeKey('c'))
	m = upd.(*Model)
	if m.chartMode != chartIntraday {
		t.Fatal("再按 c 应切回分时")
	}
	if cmd == nil {
		t.Error("切回分时应触发请求")
	}
}

func TestPeriodCycleOnlyInKline(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})

	before := m.period
	m = press(m, runeKey('p'))
	if m.period != before {
		t.Errorf("分时模式下 p 不应切换周期，得到 %q", m.period)
	}

	m = press(m, runeKey('c'))
	m = press(m, runeKey('p'))
	if m.period != quote.PeriodWeek {
		t.Errorf("周期 = %q，期望 week", m.period)
	}
	for _, want := range []string{quote.PeriodMonth, quote.Period60Min, quote.PeriodDay} {
		m = press(m, runeKey('p'))
		if m.period != want {
			t.Fatalf("周期 = %q，期望 %q", m.period, want)
		}
	}
}

func TestStaleKlineIgnored(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = press(m, runeKey('c'))

	// 周期对不上，应丢弃
	upd, _ := m.Update(msgKline{key: m.detail.key, period: quote.PeriodMonth, data: quote.Kline{Candles: makeCandles(5)}})
	m = upd.(*Model)
	if len(m.kline.data.Candles) != 0 {
		t.Error("周期不匹配的结果不应写入")
	}

	upd, _ = m.Update(msgKline{key: m.detail.key, period: m.period, data: quote.Kline{Candles: makeCandles(5)}})
	m = upd.(*Model)
	if len(m.kline.data.Candles) != 5 {
		t.Errorf("应接受匹配的 K 线，得到 %d", len(m.kline.data.Candles))
	}
}

func TestKlineWindow(t *testing.T) {
	cases := []struct {
		n, w            int
		take, step, off int
	}{
		{200, 96, 48, 2, 0},
		{5, 96, 5, 2, 86},
		{20, 20, 20, 1, 0},
		{100, 40, 20, 2, 0},
	}
	for _, c := range cases {
		take, step, off := klineWindow(c.n, c.w)
		if take != c.take || step != c.step || off != c.off {
			t.Errorf("klineWindow(%d,%d) = (%d,%d,%d)，期望 (%d,%d,%d)",
				c.n, c.w, take, step, off, c.take, c.step, c.off)
		}
	}
}

func TestKlineLinesDimensions(t *testing.T) {
	th := (&Model{cfg: config.Default()}).theme()
	for _, size := range [][2]int{{96, 16}, {40, 8}, {20, 5}} {
		lines := klineLines(th, makeCandles(120), size[0], size[1])
		if len(lines) != size[1] {
			t.Fatalf("%dx%d 行数 %d", size[0], size[1], len(lines))
		}
		nonEmpty := 0
		for i, ln := range lines {
			if w := lipgloss.Width(ln); w != size[0] {
				t.Errorf("%dx%d 第 %d 行宽 %d", size[0], size[1], i, w)
			}
			if strings.TrimSpace(ln) != "" {
				nonEmpty++
			}
		}
		if nonEmpty < 2 {
			t.Errorf("%dx%d 图表为空", size[0], size[1])
		}
	}
}

func TestDetailKlineViewFits(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {80, 20}} {
		m := newTestModel(t, size[0], size[1])
		m.detail.active = true
		m.detail.key = m.cfg.Watchlist[0].Key()
		m.chartMode = chartKline
		m.kline.data = quote.Kline{
			Symbol:  m.cfg.Watchlist[0],
			Period:  quote.PeriodDay,
			Candles: makeCandles(120),
		}
		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) != m.height {
			t.Errorf("%dx%d 行数 %d，期望 %d", size[0], size[1], len(lines), m.height)
		}
		for i, ln := range lines {
			if w := lipgloss.Width(ln); w > m.width {
				t.Errorf("%dx%d 第 %d 行宽 %d 超出", size[0], size[1], i, w)
			}
		}
	}
}
