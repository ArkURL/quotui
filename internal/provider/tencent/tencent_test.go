package tencent

import (
	"strings"
	"testing"

	"github.com/arkurl/quotui/internal/quote"
)

// 取自接口真实响应的一行，保留原始字段顺序。
const mtLine = "v_sh600519=\"1~贵州茅台~600519~1349.98~1335.00~1325.40~41599~24552~17047~1349.40~3~1349.00~1~1348.50~1~1348.00~16~1347.35~2~1349.98~0~1349.99~35~1350.00~748~1350.01~2~1350.03~5~~20261026153320~14.98~1.12~1350.00~1324.79~1349.98/41599/5574287630~41599~557429~0.33~20.72~~1350.00~1324.79~1.89~16875.85~16875.85~6.72~1468.50~1201.50~1.00~-767~1340.00~18.95~20.50~~~0.03~557428.7630~418.4938~31~   A~GP-A~\";"

func TestParseQuotes(t *testing.T) {
	qs := parseQuotes(mtLine)
	if len(qs) != 1 {
		t.Fatalf("解析出 %d 条，期望 1 条", len(qs))
	}
	q := qs[0]
	if q.Symbol.Market != "sh" || q.Symbol.Code != "600519" {
		t.Errorf("标的错误: %+v", q.Symbol)
	}
	if q.Name != "贵州茅台" {
		t.Errorf("名称 = %q", q.Name)
	}
	if q.Price != 1349.98 || q.ChangePct != 1.12 || q.Turnover != 0.33 {
		t.Errorf("字段错误: price=%v pct=%v turnover=%v", q.Price, q.ChangePct, q.Turnover)
	}
	if got := q.Amount; got < 5.57e9 || got > 5.58e9 {
		t.Errorf("成交额 = %v，期望约 55.7 亿", got)
	}
	if q.Decimals != 2 {
		t.Errorf("Decimals = %d", q.Decimals)
	}
}

func TestParseQuotesUSMarketSuffix(t *testing.T) {
	f := make([]string, 60)
	f[idxName] = "苹果"
	f[idxCode] = "AAPL.OQ"
	f[idxPrice] = "347.73"
	f[idxChangePct] = "-0.14"
	line := "v_usAAPL=\"" + strings.Join(f, "~") + "\";"

	qs := parseQuotes(line)
	if len(qs) != 1 {
		t.Fatalf("解析出 %d 条", len(qs))
	}
	if qs[0].Symbol.Code != "AAPL" {
		t.Errorf("Code = %q", qs[0].Symbol.Code)
	}
	if qs[0].Symbol.SecID != "105.AAPL" {
		t.Errorf("SecID = %q，期望 105.AAPL", qs[0].Symbol.SecID)
	}
	if qs[0].Symbol.Key() != "us/aapl" {
		t.Errorf("Key = %q", qs[0].Symbol.Key())
	}
}

func TestParseQuotesHKAmountUnit(t *testing.T) {
	f := make([]string, 50)
	f[idxName] = "腾讯控股"
	f[idxCode] = "00700"
	f[idxPrice] = "387.600"
	f[idxChangePct] = "0.41"
	f[idxAmount] = "5204065882.400" // 港股的成交额单位是元
	f[idxPB] = "TENCENT"            // 该位置对港股是英文名
	line := "v_hk00700=\"" + strings.Join(f, "~") + "\";"

	qs := parseQuotes(line)
	if len(qs) != 1 {
		t.Fatalf("解析出 %d 条", len(qs))
	}
	q := qs[0]
	if q.Amount < 5.2e9 || q.Amount > 5.21e9 {
		t.Errorf("港股成交额 = %v，期望约 52 亿", q.Amount)
	}
	if q.Decimals != 3 {
		t.Errorf("港股小数位 = %d，期望 3", q.Decimals)
	}
	if q.PB != 0 {
		t.Errorf("港股不应解析市净率，得到 %v", q.PB)
	}
}

func TestParseSearch(t *testing.T) {
	body := "v_hint=\"sh~600519~贵州茅台~gzmt~GP-A^hk~00700~腾讯控股~txkg~GP^hk~13005~某权证~qz~QZ\""
	res := parseSearch(body, 10)
	if len(res) != 2 {
		t.Fatalf("解析出 %d 条，期望 2 条（权证应被过滤）", len(res))
	}
	if res[0].Name != "贵州茅台" || res[0].Market != "sh" {
		t.Errorf("第一条错误: %+v", res[0])
	}
	if res[1].Name != "腾讯控股" || res[1].Key() != "hk/00700" {
		t.Errorf("第二条错误: %+v", res[1])
	}
}

func TestUnescapeUnicode(t *testing.T) {
	if got := unescapeUnicode("\\u8d35\\u5dde\\u8305\\u53f0"); got != "贵州茅台" {
		t.Errorf("unescapeUnicode = %q", got)
	}
	if got := unescapeUnicode("贵州茅台"); got != "贵州茅台" {
		t.Errorf("纯文本应原样返回，得到 %q", got)
	}
}

func TestRequestCode(t *testing.T) {
	cases := []struct {
		sym  quote.Symbol
		want string
	}{
		{quote.Symbol{Code: "600519", Market: "sh"}, "sh600519"},
		{quote.Symbol{Code: "000001", Market: "sz"}, "sz000001"},
		{quote.Symbol{Code: "899050", Market: "bj"}, "bj899050"},
		{quote.Symbol{Code: "700", Market: "hk"}, "hk00700"},
		{quote.Symbol{Code: "aapl", Market: "us"}, "usAAPL"},
	}
	for _, c := range cases {
		if got := requestCode(c.sym); got != c.want {
			t.Errorf("requestCode(%+v) = %q, 期望 %q", c.sym, got, c.want)
		}
	}
}
