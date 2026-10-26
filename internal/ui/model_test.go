package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arkurl/quotui/internal/config"
)

func press(m *Model, k tea.KeyMsg) *Model {
	upd, _ := m.Update(k)
	return upd.(*Model)
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestNavigationAndPause(t *testing.T) {
	m := newTestModel(t, 100, 24)

	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, 期望 1", m.cursor)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, 期望 1", m.cursor)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if m.cursor != len(m.cfg.Watchlist)-1 {
		t.Fatalf("G 之后 cursor = %d", m.cursor)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if !m.paused {
		t.Error("空格应暂停刷新")
	}
}

func TestSortAndDelete(t *testing.T) {
	m := newTestModel(t, 100, 24)
	m = press(m, runeKey('s'))
	if m.sortBy != "pct" || !m.sortDesc {
		t.Fatalf("sortBy = %q desc = %v", m.sortBy, m.sortDesc)
	}
	before := len(m.cfg.Watchlist)
	m = press(m, runeKey('d'))
	if len(m.cfg.Watchlist) != before-1 {
		t.Fatalf("删除后自选数 = %d", len(m.cfg.Watchlist))
	}
	if m.cursor >= before-1 {
		t.Fatalf("删除后 cursor 越界: %d", m.cursor)
	}
}

func TestQuitPersistsCursor(t *testing.T) {
	m := newTestModel(t, 100, 24)
	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(m, tea.KeyMsg{Type: tea.KeyDown})

	upd, cmd := m.Update(runeKey('q'))
	m = upd.(*Model)
	if cmd == nil {
		t.Fatal("q 应返回退出命令")
	}

	reloaded, err := config.Load(m.path)
	if err != nil {
		t.Fatalf("重新加载配置失败: %v", err)
	}
	if reloaded.Cursor != 2 {
		t.Errorf("游标未持久化，得到 %d", reloaded.Cursor)
	}
}

func TestMoveReordersWatchlist(t *testing.T) {
	m := newTestModel(t, 100, 24)
	first, second := m.cfg.Watchlist[0].Code, m.cfg.Watchlist[1].Code
	m = press(m, runeKey('J'))
	if m.cfg.Watchlist[0].Code != second || m.cfg.Watchlist[1].Code != first {
		t.Fatalf("J 未交换顺序: %+v", m.cfg.Watchlist[:2])
	}
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, 期望 1", m.cursor)
	}
	if m.sortBy != "order" {
		t.Errorf("调整顺序后应切回自定义排序，得到 %q", m.sortBy)
	}
}
