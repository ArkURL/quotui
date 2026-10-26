package failover

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/arkurl/quotui/internal/quote"
)

type stub struct {
	name   string
	quotes map[string]quote.Quote
	err    error
	calls  int
	last   []quote.Symbol
}

func (s *stub) Name() string { return s.name }

func (s *stub) Quotes(_ context.Context, syms []quote.Symbol) ([]quote.Quote, error) {
	s.calls++
	s.last = syms
	if s.err != nil {
		return nil, s.err
	}
	out := make([]quote.Quote, 0, len(syms))
	for _, sym := range syms {
		if q, ok := s.quotes[sym.Key()]; ok {
			out = append(out, q)
		}
	}
	return out, nil
}

func (s *stub) Search(context.Context, string, int) ([]quote.Symbol, error) { return nil, nil }

func (s *stub) Intraday(context.Context, quote.Symbol) (quote.Intraday, error) {
	return quote.Intraday{}, errors.New("not implemented")
}

func (s *stub) Kline(context.Context, quote.Symbol, string, int) (quote.Kline, error) {
	return quote.Kline{}, errors.New("not implemented")
}

func sym(code, market string) quote.Symbol {
	return quote.Symbol{Code: code, Market: market}
}

func quoteOf(s quote.Symbol) quote.Quote {
	return quote.Quote{Symbol: s, Name: s.Code, Price: 10}
}

// 主源只给了一半数据时，用备用源把缺的补上。
func TestGapFilling(t *testing.T) {
	a, b := sym("600519", "sh"), sym("00700", "hk")
	primary := &stub{name: "p", quotes: map[string]quote.Quote{a.Key(): quoteOf(a)}}
	backup := &stub{name: "q", quotes: map[string]quote.Quote{b.Key(): quoteOf(b)}}

	c := New(primary, backup)
	qs, err := c.Quotes(context.Background(), []quote.Symbol{a, b})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("应补齐两只，得到 %d", len(qs))
	}
	if len(backup.last) != 1 || backup.last[0].Key() != b.Key() {
		t.Errorf("备用源只应被要求补缺口，实际收到 %+v", backup.last)
	}
	// 输出顺序跟随请求顺序
	if qs[0].Symbol.Key() != a.Key() || qs[1].Symbol.Key() != b.Key() {
		t.Errorf("顺序错乱: %s, %s", qs[0].Symbol.Key(), qs[1].Symbol.Key())
	}
}

// 主源报错后应熔断，下一轮直接跳过。
func TestBreakerSkipsFailingProvider(t *testing.T) {
	s := sym("600519", "sh")
	primary := &stub{name: "p", err: errors.New("boom")}
	backup := &stub{name: "q", quotes: map[string]quote.Quote{s.Key(): quoteOf(s)}}

	c := New(primary, backup)
	for i := 0; i < 3; i++ {
		if _, err := c.Quotes(context.Background(), []quote.Symbol{s}); err != nil {
			t.Fatalf("第 %d 轮失败: %v", i+1, err)
		}
	}
	if primary.calls != 1 {
		t.Errorf("熔断后主源应只被调用 1 次，实际 %d 次", primary.calls)
	}
	if backup.calls != 3 {
		t.Errorf("备用源应每轮都被调用，实际 %d 次", backup.calls)
	}
	if got := c.Name(); got == "" || got == "p" {
		t.Errorf("Name 应报告当前生效源与熔断状态，得到 %q", got)
	}
}

func TestBreakerRecovers(t *testing.T) {
	c := New(&stub{name: "p"}, &stub{name: "q"})
	c.fail("p")
	if len(c.attemptOrder()) != 1 {
		t.Fatal("熔断期间应跳过该源")
	}

	c.mu.Lock()
	c.down["p"] = downState{fails: 1, until: time.Now().Add(-time.Second)}
	c.mu.Unlock()
	if len(c.attemptOrder()) != 2 {
		t.Error("熔断到期后应恢复尝试")
	}
}

func TestAllDownStillTries(t *testing.T) {
	c := New(&stub{name: "p"}, &stub{name: "q"})
	c.fail("p")
	c.fail("q")
	if len(c.attemptOrder()) != 2 {
		t.Error("全部熔断时应退回全量尝试，避免彻底失明")
	}
}

func TestPartialResultWhenBackupFails(t *testing.T) {
	a, b := sym("600519", "sh"), sym("00700", "hk")
	primary := &stub{name: "p", quotes: map[string]quote.Quote{a.Key(): quoteOf(a)}}
	backup := &stub{name: "q", err: errors.New("down")}

	c := New(primary, backup)
	qs, err := c.Quotes(context.Background(), []quote.Symbol{a, b})
	if err != nil {
		t.Fatalf("有部分数据时不应整体报错: %v", err)
	}
	if len(qs) != 1 {
		t.Errorf("应返回已有的 1 条，得到 %d", len(qs))
	}
}
