package ui

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// 走一遍真实的事件循环，确认能拿到尺寸、渲染出内容并正常退出。
func TestProgramRendersAndQuits(t *testing.T) {
	m := newTestModel(t, 0, 0)
	r, w := io.Pipe()
	defer w.Close()

	var buf bytes.Buffer
	pr := tea.NewProgram(m,
		tea.WithInput(r),
		tea.WithOutput(&buf),
		tea.WithoutSignalHandler(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := pr.Run()
		done <- err
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		pr.Send(tea.WindowSizeMsg{Width: 100, Height: 24})
		time.Sleep(200 * time.Millisecond)
		pr.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("程序退出出错: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("程序未在 5 秒内退出")
	}

	out := buf.String()
	if !strings.Contains(out, "自选股") {
		t.Fatalf("渲染输出缺少列表内容: %q", out)
	}
}
