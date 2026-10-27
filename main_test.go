package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/arkurl/quotui/internal/quote"
)

func TestBuildSnapshot(t *testing.T) {
	req := []quote.Symbol{
		{Code: "600519", Market: "sh"},
		{Code: "00700", Market: "hk"},
	}
	qs := []quote.Quote{
		{
			Symbol: req[0], Name: "贵州茅台",
			Price: 1349.984, Change: 14.9799999, ChangePct: 1.1199999,
			Amount: 5574287630, MarketCap: 1687584999999.9998, Decimals: 2,
		},
		{Symbol: req[1], Name: "腾讯控股", Price: 0, Decimals: 3},
	}

	s := buildSnapshot(qs, req, "东财", time.Date(2026, 10, 26, 15, 0, 0, 0, time.UTC))

	if s.Count != 2 || len(s.Quotes) != 2 {
		t.Fatalf("count = %d, quotes = %d", s.Count, len(s.Quotes))
	}
	if s.Updated != "2026-10-26T15:00:00Z" {
		t.Errorf("updated = %q", s.Updated)
	}
	if s.Quotes[0].Price == nil || *s.Quotes[0].Price != 1349.984 {
		t.Errorf("price 保留原始精度，得到 %v", s.Quotes[0].Price)
	}
	if s.Quotes[0].ChangePct != 1.12 || s.Quotes[0].Change != 14.98 {
		t.Errorf("浮点噪声未清理: %v / %v", s.Quotes[0].Change, s.Quotes[0].ChangePct)
	}
	if s.Quotes[0].MarketCap != 1687585000000 {
		t.Errorf("market_cap = %v，期望 1687585000000", s.Quotes[0].MarketCap)
	}
	if s.Quotes[1].Price != nil || !s.Quotes[1].Suspended {
		t.Errorf("无价标的应输出 null 并标记 suspended，得到 %v / %v", s.Quotes[1].Price, s.Quotes[1].Suspended)
	}
	if len(s.Missing) != 0 {
		t.Errorf("不应有缺失项: %v", s.Missing)
	}
}

func TestBuildSnapshotMissing(t *testing.T) {
	req := []quote.Symbol{
		{Code: "600519", Market: "sh"},
		{Code: "00700", Market: "hk"},
	}
	qs := []quote.Quote{{Symbol: req[0], Name: "贵州茅台", Price: 10}}

	s := buildSnapshot(qs, req, "腾讯", time.Now())
	if len(s.Missing) != 1 || s.Missing[0] != "hk/00700" {
		t.Errorf("missing = %v，期望 [hk/00700]", s.Missing)
	}
}

func TestSnapshotJSONShape(t *testing.T) {
	req := []quote.Symbol{{Code: "600519", Market: "sh"}}
	qs := []quote.Quote{{Symbol: req[0], Name: "贵州茅台", Price: 0}}

	b, err := json.Marshal(buildSnapshot(qs, req, "东财", time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)

	for _, want := range []string{"\"price\":null", "\"suspended\":true", "\"source\":\"东财\"", "\"count\":1"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON 中缺少 %s：%s", want, out)
		}
	}
	if strings.Contains(out, "missing") {
		t.Errorf("没有缺失项时不应输出 missing 字段: %s", out)
	}
}
