package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/arkurl/quotui/internal/config"
)

// 涨跌默认配色，与 color_scheme 的 red_up 一致。
const (
	defaultUpColor   = "#F2575B"
	defaultDownColor = "#3FBF6F"
)

// colorPreset 是一套预设配色。
// scheme 非空表示用 color_scheme 表达（清空自定义色），否则直接写死两个颜色。
type colorPreset struct {
	name   string
	scheme string
	up     string
	down   string
}

var colorPresets = []colorPreset{
	{name: "A股 红涨绿跌", scheme: "red_up"},
	{name: "港美股 绿涨红跌", scheme: "green_up"},
	{name: "低调 暖橙灰蓝", up: "#B08968", down: "#6D8A96"},
	{name: "色弱 橙蓝", up: "#E69F00", down: "#56B4E9"},
}

// 弹窗里的行：先是预设，然后是自定义与恢复默认。
const (
	colorRowUp    = 4
	colorRowDown  = 5
	colorRowReset = 6
)

// upDownHex 返回当前生效的涨跌颜色：先按 color_scheme 取默认值，
// 再被 up_color / down_color 的自定义色覆盖。
func (m *Model) upDownHex() (string, string) {
	up, down := defaultUpColor, defaultDownColor
	if m.cfg.ColorScheme == "green_up" {
		up, down = down, up
	}
	if c := m.cfg.UpColor; c != "" {
		up = c
	}
	if c := m.cfg.DownColor; c != "" {
		down = c
	}
	return up, down
}

// presetColors 返回某套预设对应的颜色。
func presetColors(p colorPreset) (string, string) {
	if p.scheme == "" {
		return p.up, p.down
	}
	up, down := defaultUpColor, defaultDownColor
	if p.scheme == "green_up" {
		up, down = down, up
	}
	return up, down
}

func (m *Model) isCurrentPreset(p colorPreset) bool {
	up, down := presetColors(p)
	curUp, curDown := m.upDownHex()
	return curUp == up && curDown == down
}

func (m *Model) openColor() {
	m.color.active = true
	m.color.cursor = 0
	m.color.editing = false
	m.color.err = ""
	if m.color.input.Width == 0 {
		ti := textinput.New()
		ti.Prompt = "» "
		ti.Placeholder = "#F2575B"
		ti.CharLimit = 9
		ti.Width = 12
		m.color.input = ti
	}
}

func (m *Model) closeColor() {
	m.color.active = false
	m.color.editing = false
	m.color.err = ""
	m.color.input.Blur()
}

func (m *Model) handleColorKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.color.editing {
		switch k.String() {
		case "esc":
			m.color.editing = false
			m.color.err = ""
			m.color.input.Blur()
			return m, nil
		case "enter":
			m.applyColorInput()
			return m, nil
		}
		var cmd tea.Cmd
		m.color.input, cmd = m.color.input.Update(k)
		return m, cmd
	}

	switch k.String() {
	case "esc", "C", "q":
		m.closeColor()
	case "up", "k":
		if m.color.cursor > 0 {
			m.color.cursor--
		}
	case "down", "j":
		if m.color.cursor < colorRowReset {
			m.color.cursor++
		}
	case "enter":
		switch {
		case m.color.cursor < len(colorPresets):
			m.applyPreset(colorPresets[m.color.cursor])
		case m.color.cursor == colorRowReset:
			m.applyPreset(colorPresets[0])
		default:
			up, down := m.upDownHex()
			cur := up
			if m.color.cursor == colorRowDown {
				cur = down
			}
			m.color.editing = true
			m.color.err = ""
			m.color.input.SetValue(cur)
			return m, m.color.input.Focus()
		}
	}
	return m, nil
}

func (m *Model) applyPreset(p colorPreset) {
	if p.scheme != "" {
		m.cfg.ColorScheme = p.scheme
		m.cfg.UpColor = ""
		m.cfg.DownColor = ""
	} else {
		up, down := presetColors(p)
		m.cfg.UpColor = up
		m.cfg.DownColor = down
	}
	_ = config.Save(m.path, m.cfg)
}

func (m *Model) applyColorInput() {
	hex := config.NormalizeHex(m.color.input.Value())
	if hex == "" {
		m.color.err = "颜色格式应为 #RGB 或 #RRGGBB"
		return
	}
	if m.color.cursor == colorRowDown {
		m.cfg.DownColor = hex
	} else {
		m.cfg.UpColor = hex
	}
	_ = config.Save(m.path, m.cfg)
	m.color.editing = false
	m.color.err = ""
	m.color.input.Blur()
}

func (m *Model) colorView(th themeStyles) string {
	boxW := m.width - 6
	if boxW > 50 {
		boxW = 50
	}
	if boxW < 26 {
		boxW = 26
	}
	innerW := boxW - 4

	upHex, downHex := m.upDownHex()
	upStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(upHex))
	downStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(downHex))

	var b strings.Builder
	b.WriteString(th.title.Render("涨跌配色"))
	b.WriteString("\n\n")

	for i, p := range colorPresets {
		b.WriteString(m.presetRow(th, i, p, innerW))
		b.WriteString("\n")
	}

	b.WriteString(m.colorRow(th, colorRowUp, "上涨颜色", upHex, upStyle, m.cfg.UpColor != "", innerW))
	b.WriteString("\n")
	b.WriteString(m.colorRow(th, colorRowDown, "下降颜色", downHex, downStyle, m.cfg.DownColor != "", innerW))
	b.WriteString("\n")
	b.WriteString(m.colorResetRow(th))

	if m.color.err != "" {
		b.WriteString("\n\n")
		b.WriteString(th.up.Render(m.color.err))
	}

	b.WriteString("\n\n")
	hint := "Enter 应用 · ↑↓ 选择 · Esc 返回"
	if m.color.editing {
		hint = "Enter 确认 · Esc 取消"
	}
	b.WriteString(th.dim.Render(hint))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4A4A5E")).
		Width(boxW).
		Padding(1, 2).
		Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) presetRow(th themeStyles, idx int, p colorPreset, innerW int) string {
	upHex, downHex := presetColors(p)
	upStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(upHex))
	downStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(downHex))

	line := "  "
	if m.color.cursor == idx {
		line = th.accent.Render("▌ ")
	}
	line += th.normal.Render(padRight(p.name, 16))

	if innerW >= 31 {
		line += upStyle.Render("+1.23%")
		line += th.dim.Render(" ")
		line += downStyle.Render("-0.37%")
	}
	if m.isCurrentPreset(p) && innerW >= 37 {
		line += th.dim.Render(" 当前")
	}
	return line
}

func (m *Model) colorRow(th themeStyles, idx int, label, hex string, swatch lipgloss.Style, custom bool, innerW int) string {
	line := "  "
	if m.color.cursor == idx {
		line = th.accent.Render("▌ ")
	}
	line += th.normal.Render(padRight(label, 16))

	if m.color.editing && m.color.cursor == idx {
		return line + m.color.input.View()
	}

	line += th.normal.Render(padRight(hex, 9))
	// 窄屏时逐级省略装饰，先去掉「自定义」再去掉色块。
	if innerW >= 22 {
		line += swatch.Render("███")
	}
	if custom && innerW >= 31 {
		line += th.dim.Render(" 自定义")
	}
	return line
}

func (m *Model) colorResetRow(th themeStyles) string {
	line := "  "
	if m.color.cursor == colorRowReset {
		line = th.accent.Render("▌ ")
	}
	return line + th.normal.Render("恢复默认（A股 红涨绿跌）")
}
