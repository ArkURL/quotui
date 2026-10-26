package config

import (
	"path/filepath"
	"testing"

	"github.com/arkurl/quotui/internal/quote"
)

func TestLoadCreatesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("首次加载失败: %v", err)
	}
	if len(cfg.Watchlist) == 0 {
		t.Fatal("默认配置应包含自选股")
	}

	cfg.Watchlist = append(cfg.Watchlist, cfg.Watchlist[0])
	cfg.ColorScheme = "green_up"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("重新加载失败: %v", err)
	}
	if again.ColorScheme != "green_up" || len(again.Watchlist) != len(cfg.Watchlist) {
		t.Errorf("往返不一致: %+v", again)
	}
}

func TestColumnsAndSortNormalize(t *testing.T) {
	cfg := &Config{Columns: []string{"pct", "name", "pct", "bogus"}, SortBy: "nope"}
	cfg.normalize()
	if len(cfg.Columns) != 2 || cfg.Columns[0] != "pct" || cfg.Columns[1] != "name" {
		t.Errorf("列应去重并剔除未知项，得到 %v", cfg.Columns)
	}
	if cfg.SortBy != "order" {
		t.Errorf("非法排序字段应回落 order，得到 %q", cfg.SortBy)
	}

	empty := &Config{}
	empty.normalize()
	if len(empty.Columns) != len(AllColumns) {
		t.Errorf("空列应回落默认集合，得到 %v", empty.Columns)
	}
}

func TestLearnBackfills(t *testing.T) {
	cfg := &Config{Watchlist: []quote.Symbol{{Code: "BABA", Market: "us"}}}
	q := quote.Quote{
		Symbol: quote.Symbol{Code: "BABA", Market: "us", SecID: "106.BABA"},
		Name:   "阿里巴巴",
	}
	if !cfg.Learn([]quote.Quote{q}) {
		t.Fatal("首次应报告有改动")
	}
	w := cfg.Watchlist[0]
	if w.SecID != "106.BABA" || w.Name != "阿里巴巴" {
		t.Errorf("未补全: %+v", w)
	}
	if cfg.Learn([]quote.Quote{q}) {
		t.Error("已补全后不应再报告改动")
	}
}

func TestNormalize(t *testing.T) {
	cfg := &Config{RefreshSeconds: -5, ColorScheme: "rainbow", Watchlist: []quote.Symbol{{Code: "600519"}}}
	cfg.normalize()
	if cfg.RefreshSeconds != 0 {
		t.Errorf("负数应归零（仅手动），得到 %d", cfg.RefreshSeconds)
	}

	auto := &Config{RefreshSeconds: 99999}
	auto.normalize()
	if auto.RefreshSeconds != 3600 {
		t.Errorf("超大值应被限制，得到 %d", auto.RefreshSeconds)
	}
	if cfg.ColorScheme != "red_up" {
		t.Errorf("ColorScheme = %q", cfg.ColorScheme)
	}
	if cfg.Watchlist[0].Market != "sh" {
		t.Errorf("Market = %q", cfg.Watchlist[0].Market)
	}
}
