package provider

import (
	"context"

	"github.com/arkurl/quotui/internal/quote"
)

// Provider 是行情数据源。所有实现必须支持批量查询，避免逐支请求。
type Provider interface {
	// Name 用于状态栏展示。
	Name() string
	// Quotes 批量取快照，返回顺序不保证与入参一致。
	Quotes(ctx context.Context, symbols []quote.Symbol) ([]quote.Quote, error)
	// Search 按代码 / 名称 / 拼音检索标的。
	Search(ctx context.Context, keyword string, limit int) ([]quote.Symbol, error)
	// Intraday 取当日分时序列，用于详情页绘图。
	Intraday(ctx context.Context, symbol quote.Symbol) (quote.Intraday, error)
	// Kline 取最近 limit 根 K 线，period 见 quote.Period* 常量。
	Kline(ctx context.Context, symbol quote.Symbol, period string, limit int) (quote.Kline, error)
}
