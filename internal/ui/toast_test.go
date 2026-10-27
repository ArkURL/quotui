package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestToastExpiresAndRestoresHints(t *testing.T) {
	m := newTestModel(t, 100, 24)

	upd, cmd := m.Update(runeKey('d'))
	m = upd.(*Model)
	if m.toast == "" {
		t.Fatal("删除后应有提示")
	}
	if cmd == nil {
		t.Fatal("应安排过期定时器")
	}
	if !strings.Contains(m.View(), m.toast) {
		t.Error("提示应显示在状态栏")
	}

	upd, _ = m.Update(msgToastExpired{gen: m.toastGen})
	m = upd.(*Model)
	if m.toast != "" {
		t.Errorf("到点后应清除提示，得到 %q", m.toast)
	}
	if !strings.Contains(m.View(), "? 帮助") {
		t.Errorf("应恢复帮助文案:\n%s", m.View())
	}
}

func TestStaleToastExpiryIgnored(t *testing.T) {
	m := newTestModel(t, 100, 24)

	upd, _ := m.Update(runeKey('d'))
	m = upd.(*Model)
	oldGen := m.toastGen

	// 又来一条新提示
	upd, _ = m.Update(runeKey('s'))
	m = upd.(*Model)
	if m.toastGen == oldGen {
		t.Fatal("提示代际应自增")
	}

	// 旧定时器到点，不应清掉新提示
	upd, _ = m.Update(msgToastExpired{gen: oldGen})
	m = upd.(*Model)
	if m.toast == "" {
		t.Error("新提示被旧定时器清掉了")
	}
}

func TestToastNotRenderedAfterExpiry(t *testing.T) {
	m := newTestModel(t, 100, 24)
	upd, _ := m.Update(runeKey('d'))
	m = upd.(*Model)

	// 模拟定时器丢失：只靠渲染时的判断兜底
	m.toastAt = time.Now().Add(-toastDuration - time.Second)
	out := m.View()
	if strings.Contains(out, "已删除") {
		t.Errorf("过期提示不应再渲染:\n%s", out)
	}
	if !strings.Contains(out, "? 帮助") {
		t.Errorf("应显示帮助文案:\n%s", out)
	}
}

// 仅手动模式下不再有数据 tick，提示仍要按时恢复。
func TestToastExpiresInManualMode(t *testing.T) {
	m := newTestModel(t, 100, 24)

	m = press(m, runeKey('t'))
	m.picker.cursor = 0 // 仅手动
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)

	if m.cfg.RefreshSeconds != 0 {
		t.Fatalf("应切到仅手动，得到 %d", m.cfg.RefreshSeconds)
	}
	if cmd == nil {
		t.Fatal("切换周期后也要安排提示过期")
	}

	upd, _ = m.Update(msgToastExpired{gen: m.toastGen})
	m = upd.(*Model)
	if m.toast != "" {
		t.Errorf("仅手动模式下提示仍应自动清除，得到 %q", m.toast)
	}
}

// 关闭自动刷新（休市降频到 30 秒）时，提示也不该跟着变慢。
func TestToastExpiryIndependentOfRefreshInterval(t *testing.T) {
	m := newTestModel(t, 100, 24)
	m.cfg.RefreshSeconds = 60
	m.now = time.Date(2026, 10, 26, 23, 0, 0, 0, time.UTC) // 全部休市，tick 会被降频

	if d := m.interval(); d < 30*time.Second {
		t.Fatalf("前提不成立，interval = %v", d)
	}

	upd, _ := m.Update(runeKey('s'))
	m = upd.(*Model)
	upd, _ = m.Update(msgToastExpired{gen: m.toastGen})
	m = upd.(*Model)
	if m.toast != "" {
		t.Errorf("提示过期不受刷新周期影响，得到 %q", m.toast)
	}
}
