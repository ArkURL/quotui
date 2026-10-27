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
		if m.color.cursor < 2 {
			m.color.cursor++
		}
	case "enter":
		switch m.color.cursor {
		case 0, 1:
			up, down := m.upDownHex()
			cur := up
			if m.color.cursor == 1 {
				cur = down
			}
			m.color.editing = true
			m.color.err = ""
			m.color.input.SetValue(cur)
			return m, m.color.input.Focus()
		case 2:
			m.resetColors()
		}
	}
	return m, nil
}

func (m *Model) applyColorInput() {
	hex := config.NormalizeHex(m.color.input.Value())
	if hex == "" {
		m.color.err = "颜色格式应为 #RGB 或 #RRGGBB"
		return
	}
	if m.color.cursor == 1 {
		m.cfg.DownColor = hex
	} else {
		m.cfg.UpColor = hex
	}
	_ = config.Save(m.path, m.cfg)
	m.color.editing = false
	m.color.err = ""
	m.color.input.Blur()
}

// resetColors 清掉自定义色并回到默认的红涨绿跌。
func (m *Model) resetColors() {
	m.cfg.UpColor = ""
	m.cfg.DownColor = ""
	m.cfg.ColorScheme = "red_up"
	_ = config.Save(m.path, m.cfg)
}

func (m *Model) colorView(th themeStyles) string {
	boxW := m.width - 6
	if boxW > 46 {
		boxW = 46
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

	b.WriteString(m.colorRow(th, 0, "上涨颜色", upHex, upStyle, m.cfg.UpColor != "", innerW))
	b.WriteString("\n")
	b.WriteString(m.colorRow(th, 1, "下降颜色", downHex, downStyle, m.cfg.DownColor != "", innerW))
	b.WriteString("\n")
	b.WriteString(m.colorResetRow(th))

	b.WriteString("\n\n")
	b.WriteString(th.dim.Render("预览  "))
	b.WriteString(upStyle.Render("+1.23% ▲"))
	b.WriteString(th.dim.Render("     "))
	b.WriteString(downStyle.Render("-0.37% ▼"))

	if m.color.err != "" {
		b.WriteString("\n\n")
		b.WriteString(th.up.Render(m.color.err))
	}

	b.WriteString("\n\n")
	hint := "Enter 编辑 · ↑↓ 选择 · Esc 返回"
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

func (m *Model) colorRow(th themeStyles, idx int, label, hex string, swatch lipgloss.Style, custom bool, innerW int) string {
	line := "  "
	if m.color.cursor == idx {
		line = th.accent.Render("▌ ")
	}
	line += th.normal.Render(padRight(label, 10))

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
	if m.color.cursor == 2 {
		line = th.accent.Render("▌ ")
	}
	return line + th.normal.Render("恢复默认（红涨绿跌）")
}
