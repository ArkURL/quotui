package ui

import (
	"bytes"
	"io"
	"strings"
	"sync"
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

// syncBuffer 让渲染协程写、测试读 buffer 之间不会打架。
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// 走真实事件循环：删一只后不做任何操作，等提示自己超时，
// 确认屏幕能自己回到帮助文案。这是用户报的那个 bug 的端到端回归。
func TestToastRecoversInRealLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("需要真实等待提示超时")
	}

	m := newTestModel(t, 100, 24)
	// 仅手动模式：不排任何数据 tick，此时唯一能触发重绘的就是提示定时器本身。
	// 否则 3 秒一次的数据刷新会顺带重绘，把「定时器没生效」这个 bug 掩盖掉。
	m.cfg.RefreshSeconds = 0

	r, w := io.Pipe()
	defer w.Close()

	var out syncBuffer
	pr := tea.NewProgram(m,
		tea.WithInput(r),
		tea.WithOutput(&out),
		tea.WithoutSignalHandler(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := pr.Run()
		done <- err
	}()

	time.Sleep(150 * time.Millisecond)
	pr.Send(tea.WindowSizeMsg{Width: 100, Height: 24})
	time.Sleep(150 * time.Millisecond)
	pr.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	time.Sleep(600 * time.Millisecond)

	if !strings.Contains(out.String(), "已删除") {
		t.Fatalf("删除后状态栏应出现提示，实际输出:\n%s", out.String())
	}

	// 期间不再有任何输入，屏幕应该自己恢复
	time.Sleep(toastDuration + time.Second)

	text := out.String()
	lastToast := strings.LastIndex(text, "已删除")
	lastHelp := strings.LastIndex(text, "? 帮助")
	if lastHelp < lastToast {
		t.Fatalf("提示没有自动恢复：最后一次帮助文案(%d) 早于 提示(%d)", lastHelp, lastToast)
	}

	pr.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("程序退出出错: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("程序未退出")
	}

	// 光看画面不够：渲染时有过期兜底，即使定时器没回来，任何重绘都会把提示藏起来。
	// 这里直接查模型状态，确认定时器确实把提示清掉了。
	if m.toast != "" {
		t.Fatalf("提示没有被清除（定时器未生效），toast = %q", m.toast)
	}
}
