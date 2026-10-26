package market

import (
	"strings"
	"testing"
	"time"
)

func resetExtra(t *testing.T) {
	t.Helper()
	extraClosures.Lock()
	extraClosures.days = map[string]bool{}
	extraClosures.years = map[int]bool{}
	extraClosures.Unlock()
	t.Cleanup(func() {
		extraClosures.Lock()
		extraClosures.days = map[string]bool{}
		extraClosures.years = map[int]bool{}
		extraClosures.Unlock()
	})
}

func TestAShareHolidays(t *testing.T) {
	cases := []struct {
		day  string
		want bool
	}{
		{"2026-10-01", false}, // 国庆
		{"2026-10-02", false},
		{"2026-02-17", false}, // 春节假期中的工作日
		{"2026-06-19", false}, // 端午
		{"2026-09-25", false}, // 中秋
		{"2026-09-30", true},  // 节前正常交易日
		{"2026-10-08", true},  // 节后首个交易日
	}
	for _, c := range cases {
		day, err := time.Parse("2006-01-02", c.day)
		if err != nil {
			t.Fatal(err)
		}
		lt := time.Date(day.Year(), day.Month(), day.Day(), 10, 0, 0, 0, shanghai)
		if got := AShare(lt); got != c.want {
			t.Errorf("%s 10:00 AShare = %v，期望 %v", c.day, got, c.want)
		}
	}
}

func TestUnknownYearFallsBackToWeekday(t *testing.T) {
	resetExtra(t)
	lt := time.Date(2027, 1, 1, 10, 0, 0, 0, shanghai) // 周五，元旦
	if CalendarKnown(lt) {
		t.Fatal("2027 年不应被视为已收录")
	}
	if !AShare(lt) {
		t.Error("未收录年份应按星期判断，期望 true")
	}
	if !strings.Contains(Summary(lt), "A股 交易中?") {
		t.Errorf("未收录年份应带问号，得到 %q", Summary(lt))
	}
}

func TestAddClosures(t *testing.T) {
	resetExtra(t)
	AddClosures([]string{"2027-01-01", " 2027-05-03 ", "bad-date", "2027-01-01"})

	lt := time.Date(2027, 1, 1, 10, 0, 0, 0, shanghai)
	if AShare(lt) {
		t.Error("自定义休市日应生效")
	}
	if !CalendarKnown(lt) {
		t.Error("给出某年日期后该年应视为已收录")
	}
	if strings.Contains(Summary(lt), "?") {
		t.Errorf("已收录年份不应带问号，得到 %q", Summary(lt))
	}
}

func TestSummaryNoMarkInCoveredYear(t *testing.T) {
	lt := time.Date(2026, 10, 26, 10, 0, 0, 0, shanghai)
	if strings.Contains(Summary(lt), "?") {
		t.Errorf("2026 年已收录，不应带问号: %q", Summary(lt))
	}
	if !strings.Contains(Summary(lt), "A股 交易中") {
		t.Errorf("周一上午应为交易中: %q", Summary(lt))
	}
}
