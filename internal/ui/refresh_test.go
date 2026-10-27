package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRefreshPresetPicker(t *testing.T) {
	m := newTestModel(t, 100, 24)

	m = press(m, runeKey('t'))
	if !m.picker.active {
		t.Fatal("t 应打开刷新周期选择")
	}

	m.picker.cursor = 7 // 60 秒
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)
	if m.picker.active {
		t.Error("确认后应关闭选择器")
	}
	if m.cfg.RefreshSeconds != 60 {
		t.Errorf("RefreshSeconds = %d，期望 60", m.cfg.RefreshSeconds)
	}
	if cmd == nil {
		t.Error("切到自动刷新后应重启定时器")
	}
	if got := m.interval(); got != 60*time.Second {
		t.Errorf("interval = %v", got)
	}
}

func TestManualOnlyMode(t *testing.T) {
	m := newTestModel(t, 100, 24)

	m = press(m, runeKey('t'))
	m.picker.cursor = 0 // 仅手动
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(*Model)

	if m.cfg.RefreshSeconds != 0 {
		t.Errorf("RefreshSeconds = %d，期望 0", m.cfg.RefreshSeconds)
	}
	if got := m.interval(); got != 0 {
		t.Errorf("仅手动模式下 interval = %v，期望 0", got)
	}
	// 返回值里可能有提示定时器，这里只关心没有触发请求、没有排 tick。
	if m.fetching {
		t.Error("切到仅手动不应触发请求")
	}
	if m.tickCmd() != nil {
		t.Error("仅手动模式下不应再有 tick")
	}
	if !strings.Contains(m.View(), "手动") {
		t.Error("状态栏应显示「手动」")
	}
}

func TestManualRefreshCooldown(t *testing.T) {
	m := newTestModel(t, 100, 24)

	// 刚请求过，连按 r 应被拦下
	m.lastRequestAt = time.Now()
	upd, _ := m.Update(runeKey('r'))
	m = upd.(*Model)
	if m.fetching {
		t.Error("冷却期内不应发起请求")
	}
	if !strings.Contains(m.toast, "频繁") {
		t.Errorf("应提示刷新过快，得到 %q", m.toast)
	}

	// 冷却结束后可以手动刷新
	m.lastRequestAt = time.Now().Add(-2 * time.Second)
	upd, cmd := m.Update(runeKey('r'))
	m = upd.(*Model)
	if cmd == nil {
		t.Error("冷却结束后应允许刷新")
	}
	if !m.fetching {
		t.Error("应标记为请求中")
	}
}

func TestStaleTickIgnored(t *testing.T) {
	m := newTestModel(t, 100, 24)
	_, cmd := m.Update(msgTick{gen: m.tickGen - 1, at: time.Now()})
	if cmd != nil {
		t.Error("周期变更后的旧 tick 应被丢弃")
	}
}

func TestFailureBackoff(t *testing.T) {
	m := newTestModel(t, 100, 24)
	m.cfg.RefreshSeconds = 3
	if got := m.interval(); got != 3*time.Second {
		t.Fatalf("正常情况 interval = %v", got)
	}

	m.failStreak = 2
	if got := m.interval(); got != backoffInterval {
		t.Errorf("连续失败后 interval = %v，期望退避到 %v", got, backoffInterval)
	}

	m.failStreak = 0
	m.cfg.RefreshSeconds = 60
	if got := m.interval(); got != 60*time.Second {
		t.Errorf("退避不应压低用户选择的长周期，得到 %v", got)
	}
}
