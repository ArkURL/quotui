package quote

import "testing"

func TestSymbolID(t *testing.T) {
	cases := []struct {
		sym Symbol
		id  string
	}{
		{Symbol{Code: "600519", Market: "sh"}, "1.600519"},
		{Symbol{Code: "000001", Market: "sz"}, "0.000001"},
		{Symbol{Code: "899050", Market: "bj"}, "0.899050"},
		{Symbol{Code: "700", Market: "hk"}, "116.00700"},
		{Symbol{Code: "00700", Market: "hk"}, "116.00700"},
		{Symbol{Code: "aapl", Market: "us"}, "105.AAPL"},
		{Symbol{Code: "AAPL", Market: "us", SecID: "106.AAPL"}, "106.AAPL"},
	}
	for _, c := range cases {
		if got := c.sym.ID(); got != c.id {
			t.Errorf("Symbol%+v.ID() = %q, 期望 %q", c.sym, got, c.id)
		}
	}
}

func TestInferMarket(t *testing.T) {
	cases := map[string]string{
		"600519": "sh",
		"688981": "sh",
		"000001": "sz",
		"300750": "sz",
		"830799": "bj",
		"00700":  "hk",
		"AAPL":   "us",
	}
	for code, want := range cases {
		if got := InferMarket(code); got != want {
			t.Errorf("InferMarket(%q) = %q, 期望 %q", code, got, want)
		}
	}
}

func TestLimitPct(t *testing.T) {
	cases := []struct {
		code, name string
		want       float64
	}{
		{"600519", "贵州茅台", 10},
		{"000001", "平安银行", 10},
		{"300750", "宁德时代", 20},
		{"688981", "中芯国际", 20},
		{"899050", "北证50", 30},
		{"600519", "*ST测试", 5},
		{"00700", "腾讯控股", 0},
		{"AAPL", "苹果", 0},
	}
	for _, c := range cases {
		if got := LimitPct(c.code, c.name); got != c.want {
			t.Errorf("LimitPct(%s, %s) = %v, 期望 %v", c.code, c.name, got, c.want)
		}
	}
}

func TestFormatters(t *testing.T) {
	if got := FmtPrice(1349.984, 2); got != "1349.98" {
		t.Errorf("FmtPrice = %q", got)
	}
	if got := FmtPct(1.234); got != "+1.23%" {
		t.Errorf("FmtPct = %q", got)
	}
	if got := FmtAmount(5574287630); got != "55.74亿" {
		t.Errorf("FmtAmount = %q", got)
	}
	if got := FmtAmount(0); got != "--" {
		t.Errorf("FmtAmount(0) = %q", got)
	}
}
