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

func TestColorPresetsApply(t *testing.T) {
	if len(colorPresets) != 4 || colorRowUp != 4 || colorRowReset != 6 {
		t.Fatalf("预设数量与行号常量不一致: %d", len(colorPresets))
	}

	for i, p := range colorPresets {
		m := newTestModel(t, 100, 30)
		m = press(m, runeKey('C'))
		m.color.cursor = i

		upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = upd.(*Model)

		wantUp, wantDown := presetColors(p)
		if up, down := m.upDownHex(); up != wantUp || down != wantDown {
			t.Errorf("预设 %q 生效颜色 = %s/%s，期望 %s/%s", p.name, up, down, wantUp, wantDown)
		}

		// scheme 型预设应清空自定义色，调色板型预设应写死自定义色
		if p.scheme != "" {
			if m.cfg.UpColor != "" || m.cfg.DownColor != "" {
				t.Errorf("预设 %q 应清空自定义色", p.name)
			}
			if m.cfg.ColorScheme != p.scheme {
				t.Errorf("预设 %q 的 scheme = %q", p.name, m.cfg.ColorScheme)
			}
		} else if m.cfg.UpColor != p.up || m.cfg.DownColor != p.down {
			t.Errorf("预设 %q 未写入配置: %q/%q", p.name, m.cfg.UpColor, m.cfg.DownColor)
		}
	}
}

func TestPresetSwitchesFromGlobalToAShare(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m = press(m, runeKey('C'))

	// 切到港美股：绿涨红跌
	m.color.cursor = 1
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)
	if up, _ := m.upDownHex(); up != defaultDownColor {
		t.Errorf("港美股预设上涨色 = %s", up)
	}

	// 再切回 A 股
	m.color.cursor = 0
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)
	if up, down := m.upDownHex(); up != defaultUpColor || down != defaultDownColor {
		t.Errorf("切回 A 股后 = %s/%s", up, down)
	}
}

func TestColorModalEditsUpColor(t *testing.T) {
	m := newTestModel(t, 100, 30)

	m = press(m, runeKey('C'))
	m.color.cursor = colorRowUp

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
	// 自定义色不再匹配任何预设
	for _, p := range colorPresets {
		if m.isCurrentPreset(p) {
			t.Errorf("自定义色不应命中预设 %q", p.name)
		}
	}
}

func TestColorModalRejectsInvalid(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m = press(m, runeKey('C'))
	m.color.cursor = colorRowUp

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
	m.color.cursor = colorRowReset
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

func TestColorViewShowsPresetsAndCurrent(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.cfg.UpColor = "#5A5A6E"
	m = press(m, runeKey('C'))

	out := m.View()
	for _, want := range []string{"A股 红涨绿跌", "港美股 绿涨红跌", "低调 暖橙灰蓝", "色弱 橙蓝", "#5A5A6E", "恢复默认"} {
		if !strings.Contains(out, want) {
			t.Errorf("弹窗缺少 %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "当前") {
		t.Error("自定义色不该把某个预设标成当前")
	}

	// 默认状态下 A 股预设应标为当前
	n := newTestModel(t, 100, 30)
	n = press(n, runeKey('C'))
	if !strings.Contains(n.View(), "当前") {
		t.Errorf("默认应把 A 股预设标成当前:\n%s", n.View())
	}
}

func TestColorModalFits(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {60, 20}, {44, 16}} {
		m := newTestModel(t, size[0], size[1])
		m = press(m, runeKey('C'))
		for i, ln := range strings.Split(m.View(), "\n") {
			if w := lipgloss.Width(ln); w > m.width {
				t.Errorf("%dx%d 第 %d 行宽 %d 超出", size[0], size[1], i, w)
			}
		}
	}
}
