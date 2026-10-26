package market

import (
	"strings"
	"time"
	_ "time/tzdata"
)

var (
	shanghai = mustLoad("Asia/Shanghai", 8)
	hongkong = mustLoad("Asia/Hong_Kong", 8)
	newyork  = mustLoad("America/New_York", -5)
)

func mustLoad(name string, offsetHours int) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone(name, offsetHours*3600)
	}
	return loc
}

func isWeekday(t time.Time) bool {
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return true
}

func minuteOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

func within(t time.Time, spans ...[2]int) bool {
	m := minuteOfDay(t)
	for _, s := range spans {
		if m >= s[0] && m <= s[1] {
			return true
		}
	}
	return false
}

// AShare 判断 A 股是否处于连续竞价（含集合竞价）时段。
func AShare(t time.Time) bool {
	lt := t.In(shanghai)
	if !isWeekday(lt) {
		return false
	}
	return within(lt, [2]int{9*60 + 15, 11*60 + 30}, [2]int{13 * 60, 15 * 60})
}

func HK(t time.Time) bool {
	lt := t.In(hongkong)
	if !isWeekday(lt) {
		return false
	}
	return within(lt, [2]int{9*60 + 30, 12 * 60}, [2]int{13 * 60, 16 * 60})
}

func US(t time.Time) bool {
	lt := t.In(newyork)
	if !isWeekday(lt) {
		return false
	}
	return within(lt, [2]int{9*60 + 30, 16 * 60})
}

// IsAnyOpen 表示至少有一个市场在交易中（用于决定刷新频率）。
func IsAnyOpen(t time.Time) bool {
	return AShare(t) || HK(t) || US(t)
}

// Summary 返回形如 "A股 交易中 · 港股 休市" 的状态摘要。
func Summary(t time.Time) string {
	parts := make([]string, 0, 3)
	for _, s := range []struct {
		name string
		open bool
	}{
		{"A股", AShare(t)},
		{"港股", HK(t)},
		{"美股", US(t)},
	} {
		state := "休市"
		if s.open {
			state = "交易中"
		}
		parts = append(parts, s.name+" "+state)
	}
	return strings.Join(parts, " · ")
}

// Label 给单个市场的中文名。
func Label(m string) string {
	switch m {
	case "sh":
		return "沪"
	case "sz":
		return "深"
	case "bj":
		return "京"
	case "hk":
		return "港"
	case "us":
		return "美"
	}
	return m
}
