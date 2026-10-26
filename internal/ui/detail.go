package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/arkurl/quotui/internal/market"
	"github.com/arkurl/quotui/internal/quote"
)

func (m *Model) detailView(th themeStyles) string {
	if m.height < 12 || m.width < 30 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			th.dim.Render("窗口太小，按 Esc 返回"))
	}

	rows := m.visible()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			th.dim.Render("按 Esc 返回"))
	}

	sym := rows[m.cursor]
	q, hasQuote := m.quotes[sym.Key()]
	d := m.detail.data

	dec := q.Decimals
	if dec <= 0 {
		dec = d.Decimals
	}
	if dec <= 0 {
		dec = 2
	}
	prevClose := q.PrevClose
	if prevClose <= 0 {
		prevClose = d.PrevClose
	}

	var b strings.Builder

	head := th.title.Render(m.name(sym)) +
		th.dim.Render("  "+sym.Code+"  "+market.Label(sym.Market))
	if m.chartMode == chartKline {
		head += th.accent.Render("  " + quote.PeriodLabel(m.period))
	}
	b.WriteString(spread(head, th.dim.Render(m.now.Format("15:04:05")), m.width))
	b.WriteString("\n")
	b.WriteString(m.rule(th))
	b.WriteString("\n")

	if hasQuote && q.Price > 0 {
		st := th.sign(q.Change)
		b.WriteString(st.Bold(true).Render(quote.FmtPrice(q.Price, dec)))
		b.WriteString("  ")
		b.WriteString(st.Render(quote.FmtChange(q.Change, dec)))
		b.WriteString("  ")
		b.WriteString(st.Render(quote.FmtPct(q.ChangePct)))
	} else {
		b.WriteString(th.dim.Render("--"))
	}
	b.WriteString("\n")

	fields := []struct{ label, value string }{
		{"今开", fmtOrDash(q.Open, dec)},
		{"最高", fmtOrDash(q.High, dec)},
		{"最低", fmtOrDash(q.Low, dec)},
		{"昨收", fmtOrDash(prevClose, dec)},
		{"成交额", quote.FmtAmount(q.Amount)},
		{"换手", quote.FmtPctPlain(q.Turnover)},
		{"量比", fmtRatio(q.VolRatio)},
		{"市盈", fmtRatio(q.PE)},
	}

	perRow := (m.width - 2) / 18
	if perRow < 1 {
		perRow = 1
	}
	if perRow > len(fields) {
		perRow = len(fields)
	}
	fieldRows := (len(fields) + perRow - 1) / perRow
	for i := 0; i < len(fields); i += perRow {
		end := i + perRow
		if end > len(fields) {
			end = len(fields)
		}
		var line strings.Builder
		for _, f := range fields[i:end] {
			line.WriteString(th.dim.Render(padRight(f.label, 7)))
			line.WriteString(th.normal.Render(padRight(f.value, 11)))
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		b.WriteString("\n")
	}

	// 固定 6 行：标题、分隔线、价格、时间轴、分隔线、状态栏
	chartH := m.height - 6 - fieldRows
	if chartH < 3 {
		chartH = 3
	}

	var chart []string
	var axis string
	blank := strings.Repeat(" ", m.width)

	if m.chartMode == chartKline {
		k := m.kline.data
		switch {
		case m.kline.err != nil:
			chart = m.placeholder(th, chartH, "K 线加载失败: "+m.kline.err.Error())
			axis = blank
		case len(k.Candles) < 2:
			chart = m.placeholder(th, chartH, "加载"+quote.PeriodLabel(m.period)+"…")
			axis = blank
		default:
			chart = klineLines(th, k.Candles, m.width, chartH)
			axis = klineAxis(th, k.Candles, m.period, m.width)
		}
	} else {
		switch {
		case m.detail.err != nil:
			chart = m.placeholder(th, chartH, "分时加载失败: "+m.detail.err.Error())
			axis = blank
		case len(d.Points) < 2:
			chart = m.placeholder(th, chartH, "加载分时…")
			axis = blank
		default:
			chart = chartLines(th, d.Points, prevClose, m.width, chartH)
			first := th.dim.Render(d.Points[0].At.Format("15:04"))
			last := th.dim.Render(d.Points[len(d.Points)-1].At.Format("15:04"))
			axis = spread(first, last, m.width)
		}
	}

	for _, ln := range chart {
		b.WriteString(ln)
		b.WriteString("\n")
	}
	b.WriteString(axis)
	b.WriteString("\n")
	b.WriteString(m.rule(th))
	b.WriteString("\n")
	b.WriteString(m.statusLine(th))
	return b.String()
}

func (m *Model) placeholder(th themeStyles, h int, msg string) []string {
	out := make([]string, 0, h)
	mid := h / 2
	for i := 0; i < h; i++ {
		if i == mid {
			out = append(out, lipgloss.Place(m.width, 1, lipgloss.Center, lipgloss.Center,
				th.dim.Render(fitText(msg, m.width-2))))
		} else {
			out = append(out, strings.Repeat(" ", m.width))
		}
	}
	return out
}

// chartLines 把分时序列画成 h 行 w 列的折线。
// 每一列取该列内的极值画成竖条，再与相邻列连起来，
// 这样 240 个采样点压到几十列也不会散成断点。
func chartLines(th themeStyles, pts []quote.Point, prevClose float64, w, h int) []string {
	if w < 4 || h < 3 || len(pts) < 2 {
		return blankLines(w, h)
	}

	lo, hi := pts[0].Price, pts[0].Price
	hasAvg := false
	for _, p := range pts {
		lo = math.Min(lo, p.Price)
		hi = math.Max(hi, p.Price)
		if p.Avg > 0 {
			hasAvg = true
			lo = math.Min(lo, p.Avg)
			hi = math.Max(hi, p.Avg)
		}
	}
	if prevClose > 0 {
		lo = math.Min(lo, prevClose)
		hi = math.Max(hi, prevClose)
	}
	if hi-lo < 1e-9 {
		hi += 0.01
		lo -= 0.01
	}
	pad := (hi - lo) * 0.06
	lo -= pad
	hi += pad

	yFor := func(v float64) int {
		y := int(math.Round((hi - v) / (hi - lo) * float64(h-1)))
		if y < 0 {
			y = 0
		}
		if y > h-1 {
			y = h - 1
		}
		return y
	}

	type bucket struct {
		min, max, last float64
		has            bool
	}
	priceCols := make([]bucket, w)
	avgCols := make([]bucket, w)

	for i, p := range pts {
		x := 0
		if len(pts) > 1 && w > 1 {
			x = int(math.Round(float64(i) * float64(w-1) / float64(len(pts)-1)))
		}
		b := &priceCols[x]
		if !b.has {
			b.min, b.max = p.Price, p.Price
			b.has = true
		} else {
			b.min = math.Min(b.min, p.Price)
			b.max = math.Max(b.max, p.Price)
		}
		b.last = p.Price

		if p.Avg > 0 {
			a := &avgCols[x]
			if !a.has {
				a.min, a.max = p.Avg, p.Avg
				a.has = true
			} else {
				a.min = math.Min(a.min, p.Avg)
				a.max = math.Max(a.max, p.Avg)
			}
			a.last = p.Avg
		}
	}

	priceG := newGrid(h, w)
	avgG := newGrid(h, w)

	prevLast := -1
	lastCol := -1
	for x := 0; x < w; x++ {
		b := priceCols[x]
		if !b.has {
			continue
		}
		top := yFor(b.max)
		bottom := yFor(b.min)
		for y := top; y <= bottom; y++ {
			priceG[y][x] = 1
		}
		// 与上一列的收盘位置连起来，避免陡峭处断线。
		if prevLast >= 0 {
			if prevLast < top {
				for y := prevLast; y < top; y++ {
					priceG[y][x] = 1
				}
			} else if prevLast > bottom {
				for y := bottom + 1; y <= prevLast; y++ {
					priceG[y][x] = 1
				}
			}
		}
		prevLast = yFor(b.last)
		lastCol = x
	}

	if hasAvg {
		for x := 0; x < w; x++ {
			a := avgCols[x]
			if !a.has {
				continue
			}
			top := yFor(a.max)
			bottom := yFor(a.min)
			for y := top; y <= bottom; y++ {
				avgG[y][x] = 1
			}
		}
	}

	baseRow := -1
	if prevClose > 0 {
		baseRow = yFor(prevClose)
	}

	trend := th.flat
	if prevClose > 0 {
		trend = th.sign(pts[len(pts)-1].Price - prevClose)
	}

	lines := make([]string, 0, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			switch {
			case y == prevLast && x == lastCol:
				sb.WriteString(trend.Render("•"))
			case priceG[y][x] == 1:
				sb.WriteString(trend.Render("│"))
			case avgG[y][x] == 1:
				sb.WriteString(th.accent.Render("·"))
			case y == baseRow:
				sb.WriteString(th.dim.Render("┈"))
			default:
				sb.WriteString(" ")
			}
		}
		lines = append(lines, sb.String())
	}
	return lines
}

func newGrid(h, w int) [][]uint8 {
	g := make([][]uint8, h)
	for i := range g {
		g[i] = make([]uint8, w)
	}
	return g
}

func blankLines(w, h int) []string {
	out := make([]string, 0, h)
	for i := 0; i < h; i++ {
		out = append(out, strings.Repeat(" ", w))
	}
	return out
}

func fmtOrDash(v float64, dec int) string {
	if v <= 0 {
		return "--"
	}
	return quote.FmtPrice(v, dec)
}

func fmtRatio(v float64) string {
	if v <= 0 {
		return "--"
	}
	return fmt.Sprintf("%.2f", v)
}
