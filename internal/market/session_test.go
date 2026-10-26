package market

import (
	"testing"
	"time"
)

func at(hour, min int) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	// 2026-10-26 是周一
	return time.Date(2026, 10, 26, hour, min, 0, 0, loc)
}

func TestAShareSession(t *testing.T) {
	cases := []struct {
		hour, min int
		want      bool
	}{
		{9, 0, false},
		{9, 15, true},
		{11, 30, true},
		{12, 0, false},
		{13, 0, true},
		{15, 0, true},
		{15, 1, false},
	}
	for _, c := range cases {
		if got := AShare(at(c.hour, c.min)); got != c.want {
			t.Errorf("AShare(%02d:%02d) = %v, 期望 %v", c.hour, c.min, got, c.want)
		}
	}
	if AShare(at(10, 0).AddDate(0, 0, 5)) {
		t.Error("周六不应判定为交易中")
	}
}
