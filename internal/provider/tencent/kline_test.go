package tencent

import (
	"encoding/json"
	"testing"
)

func TestParseTencentCandles(t *testing.T) {
	raw := json.RawMessage(`[["2026-10-23","1311.840","1335.000","1343.880","1310.000","57686.000"],["2026-10-26","1325.40","1349.98","1350.00","1324.79","41599"]]`)
	cs := parseTencentCandles(raw)
	if len(cs) != 2 {
		t.Fatalf("解析出 %d 根", len(cs))
	}
	c := cs[1]
	if c.Open != 1325.40 || c.Close != 1349.98 || c.High != 1350 || c.Low != 1324.79 {
		t.Errorf("OHLC 错误: %+v", c)
	}
	if c.Volume != 41599 {
		t.Errorf("成交量 = %v", c.Volume)
	}
}

func TestParseTencentCandlesSkipsBadRows(t *testing.T) {
	raw := json.RawMessage(`[["bad-date","1","2","3","4","5"],["2026-10-26","1325.40","1349.98","1350.00","1324.79","41599"],["2026-10-27"]]`)
	if got := len(parseTencentCandles(raw)); got != 1 {
		t.Errorf("应只保留 1 根有效数据，得到 %d", got)
	}
}
