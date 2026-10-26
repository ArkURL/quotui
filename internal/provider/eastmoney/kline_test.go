package eastmoney

import (
	"testing"
	"time"
)

func TestParseKlines(t *testing.T) {
	lines := []string{
		"2026-10-23,1311.84,1335.00,1343.88,1310.00,57686,772109000.00,2.55,1.79,23.50,0.46",
		"2026-10-26,1325.40,1349.98,1350.00,1324.79,4159916,5574287630.00,1.89,1.12,14.98,0.33",
	}
	cs := parseKlines(lines)
	if len(cs) != 2 {
		t.Fatalf("解析出 %d 根", len(cs))
	}
	c := cs[1]
	if c.Open != 1325.40 || c.Close != 1349.98 || c.High != 1350 || c.Low != 1324.79 {
		t.Errorf("OHLC 错误: %+v", c)
	}
	if c.Pct != 1.12 {
		t.Errorf("涨跌幅 = %v", c.Pct)
	}
	if !c.At.Equal(time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("日期解析错误: %v", c.At)
	}
}

func TestParseKlinesMinute(t *testing.T) {
	// 分钟线的时间带时分，且可能只有 6 个字段
	cs := parseKlines([]string{"2026-09-03 10:30,1297.50,1299.32,1305.00,1293.02,6736"})
	if len(cs) != 1 {
		t.Fatalf("解析出 %d 根", len(cs))
	}
	if cs[0].At.Hour() != 10 || cs[0].At.Minute() != 30 {
		t.Errorf("时间解析错误: %v", cs[0].At)
	}
	if cs[0].Close != 1299.32 {
		t.Errorf("收盘价 = %v", cs[0].Close)
	}
}

func TestKlineBeg(t *testing.T) {
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	if got := klineBeg("day", now); got != "20250921" {
		t.Errorf("day beg = %s", got)
	}
	if got := klineBeg("60m", now); len(got) != 8 {
		t.Errorf("60m beg = %s", got)
	}
}
