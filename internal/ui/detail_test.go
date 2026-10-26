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

func buildSeries(n int, start float64) []quote.Point {
	pts := make([]quote.Point, 0, n)
	base := time.Date(2026, 10, 26, 9, 30, 0, 0, time.Local)
	for i := 0; i < n; i++ {
		price := start + float64(i%20)
		pts = append(pts, quote.Point{
			At:    base.Add(time.Duration(i) * time.Minute),
			Price: price,
			Avg:   price - 0.5,
		})
	}
	return pts
}

func TestOpenAndCloseDetail(t *testing.T) {
	m := newTestModel(t, 100, 30)

	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)
	if !m.detail.active {
		t.Fatal("Enter 应打开详情页")
	}
	if cmd == nil {
		t.Error("打开详情应触发分时请求")
	}
	if m.detail.key != m.cfg.Watchlist[0].Key() {
		t.Errorf("详情 key = %q", m.detail.key)
	}

	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = upd.(*Model)
	if m.detail.active {
		t.Error("Esc 应关闭详情页")
	}
}

func TestDetailSwitchesSymbol(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})

	upd, cmd := m.Update(runeKey('j'))
	m = upd.(*Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d，期望 1", m.cursor)
	}
	if m.detail.key != m.cfg.Watchlist[1].Key() {
		t.Errorf("切换后 detail.key = %q", m.detail.key)
	}
	if cmd == nil {
		t.Error("切换标的应重新拉分时")
	}
}

func TestStaleIntradayIgnored(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})

	upd, _ := m.Update(msgIntraday{key: "us/other", data: quote.Intraday{Points: buildSeries(5, 10)}})
	m = upd.(*Model)
	if len(m.detail.data.Points) != 0 {
		t.Error("迟到的分时结果不应覆盖当前标的")
	}

	upd, _ = m.Update(msgIntraday{key: m.detail.key, data: quote.Intraday{Points: buildSeries(5, 10)}})
	m = upd.(*Model)
	if len(m.detail.data.Points) != 5 {
		t.Errorf("应接受当前标的的分时，得到 %d 点", len(m.detail.data.Points))
	}
}

func TestChartLines(t *testing.T) {
	th := (&Model{cfg: config.Default()}).theme()
	pts := buildSeries(240, 100)

	lines := chartLines(th, pts, 105, 80, 12)
	if len(lines) != 12 {
		t.Fatalf("行数 = %d，期望 12", len(lines))
	}
	nonEmpty := 0
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w != 80 {
			t.Errorf("第 %d 行宽度 = %d，期望 80", i, w)
		}
		if strings.TrimSpace(ln) != "" {
			nonEmpty++
		}
	}
	if nonEmpty < 3 {
		t.Error("折线应画出内容")
	}
}

func TestChartLinesDimensions(t *testing.T) {
	th := (&Model{cfg: config.Default()}).theme()
	base := time.Date(2026, 10, 26, 9, 30, 0, 0, time.Local)

	flat := make([]quote.Point, 30)
	ramp := make([]quote.Point, 240)
	for i := range flat {
		flat[i] = quote.Point{At: base.Add(time.Duration(i) * time.Minute), Price: 10, Avg: 10}
	}
	for i := range ramp {
		ramp[i] = quote.Point{At: base.Add(time.Duration(i) * time.Minute), Price: 10 + float64(i)*0.1}
	}

	cases := []struct {
		name string
		pts  []quote.Point
		prev float64
	}{
		{"振荡+均价", buildSeries(240, 100), 105},
		{"单调无均价", ramp, 0},
		{"完全平坦", flat, 10},
		{"仅两点", buildSeries(2, 10), 10},
	}
	sizes := [][2]int{{80, 12}, {40, 6}, {120, 20}, {8, 3}}

	for _, c := range cases {
		for _, s := range sizes {
			lines := chartLines(th, c.pts, c.prev, s[0], s[1])
			if len(lines) != s[1] {
				t.Fatalf("%s %dx%d: 行数 %d", c.name, s[0], s[1], len(lines))
			}
			for i, ln := range lines {
				if w := lipgloss.Width(ln); w != s[0] {
					t.Fatalf("%s %dx%d: 第 %d 行宽 %d", c.name, s[0], s[1], i, w)
				}
			}
		}
	}
}

func TestDetailViewFits(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {80, 20}, {40, 14}} {
		m := newTestModel(t, size[0], size[1])
		m.detail.active = true
		m.detail.key = m.cfg.Watchlist[0].Key()
		m.detail.data = quote.Intraday{
			Symbol:    m.cfg.Watchlist[0],
			PrevClose: 105,
			Decimals:  2,
			Points:    buildSeries(120, 100),
		}
		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) != m.height {
			t.Errorf("%dx%d 行数 = %d，期望 %d", size[0], size[1], len(lines), m.height)
		}
		for i, ln := range lines {
			if w := lipgloss.Width(ln); w > m.width {
				t.Errorf("%dx%d 第 %d 行宽 %d 超出", size[0], size[1], i, w)
			}
		}
	}
}
