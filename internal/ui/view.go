package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/arkurl/quotui/internal/config"
	"github.com/arkurl/quotui/internal/market"
	"github.com/arkurl/quotui/internal/quote"
)

type themeStyles struct {
	up     lipgloss.Style
	down   lipgloss.Style
	flat   lipgloss.Style
	dim    lipgloss.Style
	accent lipgloss.Style
	title  lipgloss.Style
	normal lipgloss.Style
}

func (t themeStyles) sign(v float64) lipgloss.Style {
	switch {
	case v > 0:
		return t.up
	case v < 0:
		return t.down
	}
	return t.flat
}

func (m *Model) theme() themeStyles {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#F2575B"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#3FBF6F"))
	up, down := red, green
	if m.cfg.ColorScheme == "green_up" {
		up, down = green, red
	}
	return themeStyles{
		up:     up,
		down:   down,
		flat:   lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9AAE")),
		dim:    lipgloss.NewStyle().Foreground(lipgloss.Color("#6E6E82")),
		accent: lipgloss.NewStyle().Foreground(lipgloss.Color("#7FD1E8")),
		title:  lipgloss.NewStyle().Foreground(lipgloss.Color("#E8E8F0")).Bold(true),
		normal: lipgloss.NewStyle().Foreground(lipgloss.Color("#D6D6E2")),
	}
}

func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	th := m.theme()
	if m.showHelp {
		return m.helpView(th)
	}
	if m.add.active {
		return m.addView(th)
	}
	if m.picker.active {
		return m.pickerView(th)
	}
	if m.detail.active {
		return m.detailView(th)
	}

	rows := m.visible()
	cols := m.columns()
	bodyH := m.height - 5
	if bodyH < 1 {
		bodyH = 1
	}

	var b strings.Builder
	b.WriteString(m.titleLine(th, len(rows)))
	b.WriteString("\n")
	b.WriteString(m.rule(th))
	b.WriteString("\n")
	b.WriteString(m.headerLine(th, cols))
	b.WriteString("\n")

	if len(rows) == 0 {
		b.WriteString(lipgloss.Place(m.width, bodyH, lipgloss.Center, lipgloss.Center,
			th.dim.Render("暂无自选，按 a 添加")))
		b.WriteString("\n")
	} else {
		offset := m.offset(bodyH, len(rows))
		for i := offset; i < offset+bodyH; i++ {
			if i < len(rows) {
				b.WriteString(m.rowLine(th, cols, rows[i], i == m.cursor))
			} else {
				b.WriteString(strings.Repeat(" ", m.width))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString(m.rule(th))
	b.WriteString("\n")
	b.WriteString(m.statusLine(th))
	return b.String()
}

func (m *Model) offset(bodyH, total int) int {
	offset := 0
	if m.cursor >= bodyH {
		offset = m.cursor - bodyH + 1
	}
	if offset+bodyH > total {
		offset = total - bodyH
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

type column struct {
	key   string
	title string
	width int
	align lipgloss.Position
}

var columnDefs = map[string]column{
	"code":     {"code", "代码", 8, lipgloss.Left},
	"name":     {"name", "名称", 10, lipgloss.Left},
	"price":    {"price", "现价", 10, lipgloss.Right},
	"chg":      {"chg", "涨跌", 9, lipgloss.Right},
	"pct":      {"pct", "涨幅", 9, lipgloss.Right},
	"amount":   {"amount", "成交额", 9, lipgloss.Right},
	"turnover": {"turnover", "换手", 7, lipgloss.Right},
}

// dropOrder 是空间不足时丢弃列的顺序；名称和涨幅是兜底列，不参与丢弃。
var dropOrder = []string{"turnover", "amount", "chg", "price", "code"}

// columns 按配置顺序产出可见列，并在窄屏下按优先级依次裁掉。
func (m *Model) columns() []column {
	cols := make([]column, 0, len(m.cfg.Columns))
	for _, key := range m.cfg.Columns {
		if def, ok := columnDefs[key]; ok {
			cols = append(cols, def)
		}
	}
	if len(cols) == 0 {
		for _, key := range config.AllColumns {
			cols = append(cols, columnDefs[key])
		}
	}

	for _, key := range dropOrder {
		if lineWidth(cols) <= m.width || len(cols) <= 2 {
			break
		}
		cols = dropColumn(cols, key)
	}

	// 名称列吃掉剩余宽度。
	if extra := m.width - lineWidth(cols); extra > 0 {
		for i := range cols {
			if cols[i].key != "name" {
				continue
			}
			w := cols[i].width + extra
			if w > 24 {
				w = 24
			}
			cols[i].width = w
		}
	}
	return cols
}

func lineWidth(cols []column) int {
	total := 2 // 选中标记 + 空格
	for _, c := range cols {
		total += c.width
	}
	return total + len(cols) - 1
}

func dropColumn(cols []column, key string) []column {
	for i, c := range cols {
		if c.key == key {
			return append(cols[:i:i], cols[i+1:]...)
		}
	}
	return cols
}

func (m *Model) titleLine(th themeStyles, n int) string {
	left := th.title.Render("quotui") +
		th.dim.Render("  自选股 ") +
		th.accent.Render(fmt.Sprintf("%d", n))
	if m.paused {
		left += th.dim.Render("  暂停")
	}
	right := th.dim.Render("排序 " + m.sortLabel())
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) headerLine(th themeStyles, cols []column) string {
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		parts = append(parts, th.dim.Width(c.width).Align(c.align).Render(fitText(c.title, c.width)))
	}
	return "  " + strings.Join(parts, " ")
}

func (m *Model) rowLine(th themeStyles, cols []column, sym quote.Symbol, selected bool) string {
	q, ok := m.quotes[sym.Key()]
	marker := "  "
	if selected {
		marker = th.accent.Render("▌") + " "
	}
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		text, style := m.cellValue(th, c.key, sym, q, ok)
		if selected {
			style = style.Bold(true)
		}
		parts = append(parts, style.Width(c.width).Align(c.align).Render(fitText(text, c.width)))
	}
	return marker + strings.Join(parts, " ")
}

func (m *Model) cellValue(th themeStyles, key string, sym quote.Symbol, q quote.Quote, ok bool) (string, lipgloss.Style) {
	blank := !ok || q.Price == 0 || q.Suspended
	switch key {
	case "code":
		return sym.Code, th.dim
	case "name":
		return m.name(sym), th.normal
	case "price":
		if blank {
			return "--", th.dim
		}
		return quote.FmtPrice(q.Price, q.Decimals), th.sign(q.Change)
	case "chg":
		if blank {
			return "--", th.dim
		}
		return quote.FmtChange(q.Change, q.Decimals), th.sign(q.Change)
	case "pct":
		if blank {
			return "--", th.dim
		}
		label := quote.FmtPct(q.ChangePct)
		// 触及涨跌停时补一个箭头，比只靠颜色更醒目。
		if limit := quote.LimitPct(sym.Code, m.name(sym)); limit > 0 && math.Abs(q.ChangePct) >= limit-0.05 {
			if q.ChangePct > 0 {
				label += "↑"
			} else {
				label += "↓"
			}
		}
		return label, th.sign(q.ChangePct)
	case "amount":
		if !ok {
			return "--", th.dim
		}
		return quote.FmtAmount(q.Amount), th.dim
	case "turnover":
		if !ok || q.Turnover <= 0 {
			return "--", th.dim
		}
		return quote.FmtPctPlain(q.Turnover), th.dim
	}
	return "", th.normal
}

func (m *Model) statusLine(th themeStyles) string {
	rightPlain := "? 帮助"
	switch {
	case m.detail.active:
		rightPlain = "Esc 返回  j/k 换股  c 分时/K线  p 周期  r 刷新"
		if m.width < 86 {
			rightPlain = "Esc 返回  c 分时/K线  p 周期"
		}
		if m.width < 52 {
			rightPlain = "Esc 返回"
		}
	case m.width >= 108:
		rightPlain = "? 帮助  a 添加  d 删除  s 排序  t 周期  r 刷新  空格 暂停"
	case m.width >= 78:
		rightPlain = "? 帮助  a 添加  d 删除  s 排序  t 周期"
	}
	rightStyle := th.dim
	// 时间到了就不再渲染，即便定时器因故没有回来。
	if m.toast != "" && time.Since(m.toastAt) < toastDuration {
		rightPlain = m.toast
		rightStyle = th.accent
	}
	rw := lipgloss.Width(rightPlain)
	avail := m.width - rw - 1
	if avail < 12 {
		rightPlain = ""
		rw = 0
		avail = m.width
	}

	period := intervalLabel(m.cfg.RefreshSeconds)
	if m.cfg.RefreshSeconds > 0 {
		period = fmt.Sprintf("%ds", int(m.interval()/time.Second))
	}
	parts := []string{period, m.prov.Name()}
	if m.loading {
		parts = append(parts, "更新中")
	} else if !m.fetchedAt.IsZero() {
		parts = append(parts, "更新 "+m.fetchedAt.Format("15:04:05"))
	}
	if m.paused && m.cfg.RefreshSeconds > 0 {
		parts = append(parts, "已暂停")
	}
	if m.fetchErr != nil {
		parts = append(parts, "数据异常")
	}

	leftPlain := joinFitting(parts, avail)
	if summary := market.Summary(m.now); leftPlain != "" &&
		lipgloss.Width(leftPlain)+3+lipgloss.Width(summary) <= avail {
		leftPlain += " · " + summary
	}
	leftStyle := th.dim
	if m.fetchErr != nil {
		leftPlain = "⚠ " + m.fetchErr.Error()
		leftStyle = th.down
	}
	leftPlain = fitText(leftPlain, avail)

	left := leftStyle.Render(leftPlain)
	right := rightStyle.Render(rightPlain)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) rule(th themeStyles) string {
	if m.width <= 0 {
		return ""
	}
	return th.dim.Render(strings.Repeat("─", m.width))
}

func (m *Model) helpView(th themeStyles) string {
	rows := [][2]string{
		{"↑/k ↓/j", "移动"},
		{"g / G", "首行 / 末行"},
		{"Enter", "详情（再按返回）"},
		{"c / p", "分时·K线 / 切换周期"},
		{"r", "立即刷新（1 秒冷却）"},
		{"空格", "暂停 / 继续刷新"},
		{"s / S", "排序字段 / 升降序"},
		{"a / d", "添加 / 删除自选"},
		{"J / K", "调整自定义顺序"},
		{"t", "刷新周期（含仅手动）"},
		{"? / q", "帮助 / 退出"},
	}

	var b strings.Builder
	b.WriteString(th.title.Render("快捷键"))
	b.WriteString("\n\n")
	for _, r := range rows {
		b.WriteString(th.accent.Render(fitText(r[0], 9)))
		b.WriteString("  ")
		b.WriteString(th.normal.Render(r[1]))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.dim.Render("按任意键返回"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4A4A5E")).
		Padding(1, 3).
		Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) addView(th themeStyles) string {
	boxW := m.width - 8
	if boxW > 60 {
		boxW = 60
	}
	if boxW < 26 {
		boxW = 26
	}
	innerW := boxW - 2

	m.add.input.Width = innerW - 2
	if m.add.input.Width < 12 {
		m.add.input.Width = 12
	}

	var b strings.Builder
	b.WriteString(th.title.Render("添加自选"))
	b.WriteString("\n\n")
	b.WriteString(m.add.input.View())
	b.WriteString("\n")

	switch {
	case m.add.searching:
		b.WriteString(th.dim.Render("搜索中…"))
	case m.add.err != nil:
		b.WriteString(th.down.Render(fitText(m.add.err.Error(), innerW)))
	case len(m.add.results) == 0:
		if m.add.searched {
			b.WriteString(th.dim.Render("没有匹配的标的"))
		} else {
			b.WriteString(th.dim.Render("输入代码、名称或拼音后回车搜索"))
		}
	default:
		b.WriteString("\n")
		maxRows := 7
		offset := 0
		if m.add.cursor >= maxRows {
			offset = m.add.cursor - maxRows + 1
		}
		for i := offset; i < len(m.add.results) && i < offset+maxRows; i++ {
			s := m.add.results[i]
			label := s.Code + "  " + s.Name + "  " + market.Label(s.Market)
			if i == m.add.cursor {
				b.WriteString(th.accent.Render("▌ " + fitText(label, innerW-2)))
			} else {
				b.WriteString("  " + th.normal.Render(fitText(label, innerW-2)))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(th.dim.Render(fitText("Enter 确认 · ↑↓ 选择 · Esc 取消", innerW)))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4A4A5E")).
		Width(boxW).
		Padding(1, 2).
		Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) pickerView(th themeStyles) string {
	var b strings.Builder
	b.WriteString(th.title.Render("刷新周期"))
	b.WriteString("\n\n")
	for i, sec := range refreshPresets {
		label := fmt.Sprintf("%d 秒", sec)
		if sec == 0 {
			label = "仅手动（按 r 刷新）"
		}
		if i == m.picker.cursor {
			b.WriteString(th.accent.Render("▌ " + label))
		} else {
			b.WriteString("  " + th.normal.Render(label))
		}
		if sec == m.cfg.RefreshSeconds {
			b.WriteString(th.dim.Render("  ← 当前"))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.dim.Render("Enter 确定 · ↑↓ 选择 · Esc 取消"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4A4A5E")).
		Padding(1, 3).
		Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// joinFitting 按顺序拼接字段，超宽的后继字段直接跳过而不是截断。
func joinFitting(parts []string, limit int) string {
	out := ""
	for _, p := range parts {
		next := p
		if out != "" {
			next = out + " · " + p
		}
		if lipgloss.Width(next) > limit {
			continue
		}
		out = next
	}
	return out
}

// spread 把 left 与 right 分别贴到两端。
func spread(left, right string, width int) string {
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	if lw+rw+1 > width {
		return left
	}
	return left + strings.Repeat(" ", width-lw-rw) + right
}

// padRight 用空格把字符串补齐到指定显示宽度。
func padRight(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// fitText 按显示宽度截断（中文按 2 列计），超长时补省略号。
func fitText(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	target := w - 1
	var b strings.Builder
	cur := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if cur+rw > target {
			break
		}
		b.WriteRune(r)
		cur += rw
	}
	return b.String() + "…"
}
