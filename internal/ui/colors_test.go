package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestUpDownHex(t *testing.T) {
	m := newTestModel(t, 100, 24)

	up, down := m.upDownHex()
	if up != defaultUpColor || down != defaultDownColor {
		t.Errorf("默认配色错误: %s / %s", up, down)
	}

	m.cfg.ColorScheme = "green_up"
	up, down = m.upDownHex()
	if up != defaultDownColor || down != defaultUpColor {
		t.Errorf("绿涨红跌应互换: %s / %s", up, down)
	}

	// 只覆盖上涨色，下降色仍跟随主题
	m.cfg.UpColor = "#111111"
	up, down = m.upDownHex()
	if up != "#111111" || down != defaultUpColor {
		t.Errorf("自定义覆盖错误: %s / %s", up, down)
	}
}

func TestColorModalEditsUpColor(t *testing.T) {
	m := newTestModel(t, 100, 30)

	m = press(m, runeKey('C'))
	if !m.color.active {
		t.Fatal("C 应打开配色弹窗")
	}

	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)
	if !m.color.editing {
		t.Fatal("回车应进入编辑")
	}

	m.color.input.SetValue("")
	for _, r := range "#5a5a6e" {
		upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = upd.(*Model)
	}
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)

	if m.cfg.UpColor != "#5A5A6E" {
		t.Errorf("上涨色 = %q，期望 #5A5A6E", m.cfg.UpColor)
	}
	if m.color.editing {
		t.Error("确认后应退出编辑")
	}
	if up, _ := m.upDownHex(); up != "#5A5A6E" {
		t.Errorf("生效颜色 = %q", up)
	}
}

func TestColorModalRejectsInvalid(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m = press(m, runeKey('C'))

	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)
	m.color.input.SetValue("红色")
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)

	if m.color.err == "" {
		t.Error("非法输入应报错")
	}
	if m.cfg.UpColor != "" {
		t.Errorf("非法输入不应写入配置，得到 %q", m.cfg.UpColor)
	}
	if !m.color.editing {
		t.Error("报错后应停留在编辑态")
	}

	// Esc 取消编辑，改动丢弃
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = upd.(*Model)
	if m.color.editing {
		t.Error("Esc 应退出编辑")
	}
	if m.cfg.UpColor != "" {
		t.Error("取消后不应保留改动")
	}
}

func TestColorReset(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.cfg.UpColor = "#112233"
	m.cfg.DownColor = "#445566"
	m.cfg.ColorScheme = "green_up"

	m = press(m, runeKey('C'))
	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)

	if m.cfg.UpColor != "" || m.cfg.DownColor != "" {
		t.Errorf("恢复默认应清空自定义色: %q %q", m.cfg.UpColor, m.cfg.DownColor)
	}
	if m.cfg.ColorScheme != "red_up" {
		t.Errorf("应对回 red_up，得到 %q", m.cfg.ColorScheme)
	}
	if up, down := m.upDownHex(); up != defaultUpColor || down != defaultDownColor {
		t.Errorf("恢复后配色错误: %s / %s", up, down)
	}
}

func TestColorViewShowsCurrent(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.cfg.UpColor = "#5A5A6E"
	m = press(m, runeKey('C'))

	out := m.View()
	if !strings.Contains(out, "#5A5A6E") {
		t.Errorf("弹窗应显示当前上涨色:\n%s", out)
	}
	if !strings.Contains(out, "恢复默认") {
		t.Error("应有恢复默认选项")
	}
	if !strings.Contains(out, "预览") {
		t.Error("应有颜色预览")
	}
}

func TestColorModalFits(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {44, 14}} {
		m := newTestModel(t, size[0], size[1])
		m = press(m, runeKey('C'))
		lines := strings.Split(m.View(), "\n")
		for i, ln := range lines {
			if w := lipgloss.Width(ln); w > m.width {
				t.Errorf("%dx%d 第 %d 行宽 %d 超出", size[0], size[1], i, w)
			}
		}
	}
}
