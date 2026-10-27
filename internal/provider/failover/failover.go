package failover

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/arkurl/quotui/internal/provider"
	"github.com/arkurl/quotui/internal/quote"
)

const (
	attemptTimeout = 4 * time.Second
	// 熔断退避：30s 起，逐次翻倍，最多 5 分钟。
	breakerBase = 30 * time.Second
	breakerMax  = 5 * time.Minute
)

type downState struct {
	fails int
	until time.Time
}

// Chain 按顺序尝试多个数据源。
//
// 两个关键行为：
//   - 缺口补齐：某个源只返回了一部分标的时，用下一个源补齐剩下的，而不是直接返回半成品；
//   - 熔断：某个源连续失败后短暂跳过，避免每轮都白撞一次。
type Chain struct {
	providers []provider.Provider
	mu        sync.RWMutex
	active    string
	down      map[string]downState
}

func New(providers ...provider.Provider) *Chain {
	c := &Chain{providers: providers, down: make(map[string]downState)}
	if len(providers) > 0 {
		c.active = providers[0].Name()
	}
	return c
}

func (c *Chain) Name() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := c.active
	now := time.Now()
	var broken []string
	for _, p := range c.providers {
		if st, ok := c.down[p.Name()]; ok && now.Before(st.until) {
			broken = append(broken, p.Name()+"熔断")
		}
	}
	if len(broken) > 0 {
		return out + " · " + strings.Join(broken, ",")
	}
	return out
}

// Active 返回当前生效的数据源名称，不含熔断等展示信息，适合机器消费。
func (c *Chain) Active() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

func (c *Chain) Quotes(ctx context.Context, symbols []quote.Symbol) ([]quote.Quote, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	got := make(map[string]quote.Quote, len(symbols))
	var errs []string

	for _, p := range c.attemptOrder() {
		missing := missingSymbols(symbols, got)
		if len(missing) == 0 {
			break
		}

		actx, cancel := context.WithTimeout(ctx, attemptTimeout)
		qs, err := p.Quotes(actx, missing)
		cancel()

		if err != nil {
			errs = append(errs, p.Name()+"("+err.Error()+")")
			c.fail(p.Name())
			continue
		}

		added := 0
		for _, q := range qs {
			key := q.Symbol.Key()
			if key == "" {
				continue
			}
			if _, ok := got[key]; ok {
				continue
			}
			got[key] = q
			added++
		}
		if added == 0 {
			errs = append(errs, p.Name()+"(无数据)")
			c.fail(p.Name())
			continue
		}
		c.succeed(p.Name())
	}

	if len(got) == 0 {
		return nil, fmt.Errorf("行情获取失败: %s", strings.Join(errs, "；"))
	}
	return flatten(symbols, got), nil
}

func (c *Chain) Search(ctx context.Context, keyword string, limit int) ([]quote.Symbol, error) {
	var errs []string
	for _, p := range c.attemptOrder() {
		actx, cancel := context.WithTimeout(ctx, attemptTimeout)
		res, err := p.Search(actx, keyword, limit)
		cancel()

		if err != nil {
			errs = append(errs, p.Name()+"("+err.Error()+")")
			c.fail(p.Name())
			continue
		}
		if len(res) == 0 {
			// 搜不到是正常结果，不算故障，继续问下一个源。
			continue
		}
		c.succeed(p.Name())
		return res, nil
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("搜索失败: %s", strings.Join(errs, "；"))
	}
	return nil, nil
}

func (c *Chain) Intraday(ctx context.Context, sym quote.Symbol) (quote.Intraday, error) {
	var errs []string
	for _, p := range c.attemptOrder() {
		actx, cancel := context.WithTimeout(ctx, attemptTimeout)
		d, err := p.Intraday(actx, sym)
		cancel()

		if err != nil {
			errs = append(errs, p.Name()+"("+err.Error()+")")
			c.fail(p.Name())
			continue
		}
		if len(d.Points) == 0 {
			errs = append(errs, p.Name()+"(分时为空)")
			continue
		}
		c.succeed(p.Name())
		return d, nil
	}
	return quote.Intraday{}, fmt.Errorf("分时获取失败: %s", strings.Join(errs, "；"))
}

func (c *Chain) Kline(ctx context.Context, sym quote.Symbol, period string, limit int) (quote.Kline, error) {
	var errs []string
	for _, p := range c.attemptOrder() {
		actx, cancel := context.WithTimeout(ctx, attemptTimeout)
		k, err := p.Kline(actx, sym, period, limit)
		cancel()

		if err != nil {
			errs = append(errs, p.Name()+"("+err.Error()+")")
			c.fail(p.Name())
			continue
		}
		if len(k.Candles) == 0 {
			errs = append(errs, p.Name()+"(K 线为空)")
			continue
		}
		c.succeed(p.Name())
		return k, nil
	}
	return quote.Kline{}, fmt.Errorf("K 线获取失败: %s", strings.Join(errs, "；"))
}

// attemptOrder 返回本轮可尝试的数据源；全部熔断时仍然全部尝试，避免彻底失明。
func (c *Chain) attemptOrder() []provider.Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	out := make([]provider.Provider, 0, len(c.providers))
	for _, p := range c.providers {
		if st, ok := c.down[p.Name()]; ok && now.Before(st.until) {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return c.providers
	}
	return out
}

func (c *Chain) fail(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	st := c.down[name]
	st.fails++
	d := breakerBase << (st.fails - 1)
	if d <= 0 || d > breakerMax {
		d = breakerMax
	}
	st.until = time.Now().Add(d)
	c.down[name] = st
}

func (c *Chain) succeed(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.down, name)
	c.active = name
}

func missingSymbols(symbols []quote.Symbol, got map[string]quote.Quote) []quote.Symbol {
	out := make([]quote.Symbol, 0, len(symbols))
	for _, s := range symbols {
		if _, ok := got[s.Key()]; !ok {
			out = append(out, s)
		}
	}
	return out
}

// flatten 按请求顺序输出，保证界面顺序稳定。
func flatten(symbols []quote.Symbol, got map[string]quote.Quote) []quote.Quote {
	out := make([]quote.Quote, 0, len(got))
	for _, s := range symbols {
		if q, ok := got[s.Key()]; ok {
			out = append(out, q)
		}
	}
	return out
}
