package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arkurl/quotui/internal/quote"
)

const (
	quoteEndpoint  = "https://push2.eastmoney.com/api/qt/ulist.np/get"
	searchEndpoint = "https://searchapi.eastmoney.com/api/suggest/get"
	suggestToken   = "D43BF722C8E33BDC906FB84D85E326E8"
	fieldList      = "f1,f2,f3,f4,f5,f6,f8,f9,f10,f12,f13,f14,f15,f16,f17,f18,f20,f21,f23"
	batchSize      = 50
	userAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0 Safari/537.36"
)

// Client 是东方财富公开行情接口的客户端。
type Client struct {
	hc *http.Client
}

func New() *Client {
	return &Client{
		hc: &http.Client{
			Timeout: 8 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        16,
				MaxIdleConnsPerHost: 8,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

func (c *Client) Name() string { return "东财" }

func (c *Client) Quotes(ctx context.Context, symbols []quote.Symbol) ([]quote.Quote, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	out := make([]quote.Quote, 0, len(symbols))
	for start := 0; start < len(symbols); start += batchSize {
		end := start + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		qs, err := c.batch(ctx, symbols[start:end])
		if err != nil {
			return out, err
		}
		out = append(out, qs...)
	}
	return out, nil
}

func (c *Client) batch(ctx context.Context, symbols []quote.Symbol) ([]quote.Quote, error) {
	ids := make([]string, 0, len(symbols))
	for _, s := range symbols {
		ids = append(ids, s.ID())
	}
	q := url.Values{}
	q.Set("fltt", "2")
	q.Set("invt", "2")
	q.Set("fields", fieldList)
	q.Set("secids", strings.Join(ids, ","))
	q.Set("_", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, quoteEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("行情接口返回 %s", resp.Status)
	}

	var body emResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("解析行情失败: %w", err)
	}
	if body.Data == nil {
		return nil, nil
	}
	out := make([]quote.Quote, 0, len(body.Data.Diff))
	for _, d := range body.Data.Diff {
		if d.F12 == "" {
			continue
		}
		out = append(out, d.toQuote())
	}
	return out, nil
}

func (c *Client) Search(ctx context.Context, keyword string, limit int) ([]quote.Symbol, error) {
	kw := strings.TrimSpace(keyword)
	if kw == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("input", kw)
	q.Set("type", "14")
	q.Set("count", strconv.Itoa(limit*3))
	q.Set("token", suggestToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("搜索接口返回 %s", resp.Status)
	}

	var body searchResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	out := make([]quote.Symbol, 0, limit)
	for _, d := range body.Table.Data {
		if !quotable(d.SecurityTypeName) {
			continue
		}
		mkt, _ := strconv.Atoi(d.MktNum)
		secID := d.QuoteID
		if secID == "" {
			secID = fmt.Sprintf("%d.%s", mkt, d.Code)
		}
		out = append(out, quote.Symbol{
			Code:   d.Code,
			Market: marketOf(mkt, d.Code),
			SecID:  secID,
			Name:   d.Name,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// quotable 过滤掉期权、债券等行情接口不支持的品种。
func quotable(typeName string) bool {
	for _, kw := range []string{"股", "指数", "基金", "ETF"} {
		if strings.Contains(typeName, kw) {
			return true
		}
	}
	return false
}

// num 兼容接口的 "数字"、"字符串数字"、"-"（停牌）与 null。
type num float64

func (n *num) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	switch s {
	case "", "-", "null":
		*n = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*n = 0
		return nil
	}
	*n = num(v)
	return nil
}

type emResp struct {
	RC   int `json:"rc"`
	Data *struct {
		Total int       `json:"total"`
		Diff  []emQuote `json:"diff"`
	} `json:"data"`
}

type emQuote struct {
	F1  num    `json:"f1"`
	F2  num    `json:"f2"`
	F3  num    `json:"f3"`
	F4  num    `json:"f4"`
	F5  num    `json:"f5"`
	F6  num    `json:"f6"`
	F8  num    `json:"f8"`
	F9  num    `json:"f9"`
	F10 num    `json:"f10"`
	F12 string `json:"f12"`
	F13 num    `json:"f13"`
	F14 string `json:"f14"`
	F15 num    `json:"f15"`
	F16 num    `json:"f16"`
	F17 num    `json:"f17"`
	F18 num    `json:"f18"`
	F20 num    `json:"f20"`
	F21 num    `json:"f21"`
	F23 num    `json:"f23"`
}

func (d emQuote) toQuote() quote.Quote {
	dec := int(d.F1)
	if dec < 0 || dec > 4 {
		dec = 2
	}
	mkt := int(d.F13)
	// A 股接口返回的名称按两字对齐补了空格，展示前去掉。
	name := strings.TrimSpace(d.F14)
	if mkt == 0 || mkt == 1 {
		name = strings.ReplaceAll(name, " ", "")
	}
	q := quote.Quote{
		Symbol: quote.Symbol{
			Code:   d.F12,
			Market: marketOf(mkt, d.F12),
			SecID:  fmt.Sprintf("%d.%s", mkt, d.F12),
			Name:   name,
		},
		Name:      name,
		Price:     float64(d.F2),
		PrevClose: float64(d.F18),
		Open:      float64(d.F17),
		High:      float64(d.F15),
		Low:       float64(d.F16),
		Change:    float64(d.F4),
		ChangePct: float64(d.F3),
		Volume:    float64(d.F5),
		Amount:    float64(d.F6),
		Turnover:  float64(d.F8),
		PE:        float64(d.F9),
		PB:        float64(d.F23),
		VolRatio:  float64(d.F10),
		MarketCap: float64(d.F20),
		Decimals:  dec,
	}
	q.Suspended = q.Price == 0
	return q
}

const trendsEndpoint = "https://push2his.eastmoney.com/api/qt/stock/trends2/get"

// Intraday 取当日分时。trends 每项是逗号分隔的
// 时间,开,收,高,低,成交量,成交额,均价。
func (c *Client) Intraday(ctx context.Context, sym quote.Symbol) (quote.Intraday, error) {
	q := url.Values{}
	q.Set("secid", sym.ID())
	q.Set("fields1", "f1,f2,f3,f4,f5,f6,f7,f8")
	q.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58")
	q.Set("ndays", "1")
	q.Set("iscr", "0")
	q.Set("iscca", "0")
	q.Set("_", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trendsEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return quote.Intraday{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.hc.Do(req)
	if err != nil {
		return quote.Intraday{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return quote.Intraday{}, fmt.Errorf("分时接口返回 %s", resp.Status)
	}

	var body struct {
		Data *struct {
			Code     string   `json:"code"`
			Name     string   `json:"name"`
			Decimal  num      `json:"decimal"`
			PreClose num      `json:"preClose"`
			Trends   []string `json:"trends"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return quote.Intraday{}, fmt.Errorf("解析分时失败: %w", err)
	}
	if body.Data == nil {
		return quote.Intraday{}, fmt.Errorf("分时数据为空")
	}

	dec := int(body.Data.Decimal)
	if dec < 0 || dec > 4 {
		dec = 2
	}
	out := quote.Intraday{
		Symbol:    sym,
		Name:      body.Data.Name,
		PrevClose: float64(body.Data.PreClose),
		Decimals:  dec,
		Points:    make([]quote.Point, 0, len(body.Data.Trends)),
	}
	for _, line := range body.Data.Trends {
		f := strings.Split(line, ",")
		if len(f) < 8 {
			continue
		}
		at, err := time.Parse("2006-01-02 15:04", f[0])
		if err != nil {
			continue
		}
		out.Date = at.Format("2006-01-02")
		out.Points = append(out.Points, quote.Point{
			At:    at,
			Price: parseF(f[2]),
			Avg:   parseF(f[7]),
		})
	}
	if len(out.Points) == 0 {
		return quote.Intraday{}, fmt.Errorf("分时暂无数据")
	}
	return out, nil
}

const klineEndpoint = "https://push2his.eastmoney.com/api/qt/stock/kline/get"

var kltCodes = map[string]string{
	quote.PeriodDay:   "101",
	quote.PeriodWeek:  "102",
	quote.PeriodMonth: "103",
	quote.Period60Min: "60",
}

// Kline 取最近的 K 线。接口按 beg 升序返回，所以请求时把起点前移，
// 再在本地截取最后 limit 根。
func (c *Client) Kline(ctx context.Context, sym quote.Symbol, period string, limit int) (quote.Kline, error) {
	if limit <= 0 {
		limit = 120
	}
	klt, ok := kltCodes[period]
	if !ok {
		klt = kltCodes[quote.PeriodDay]
		period = quote.PeriodDay
	}

	q := url.Values{}
	q.Set("secid", sym.ID())
	q.Set("klt", klt)
	q.Set("fqt", "1") // 前复权
	q.Set("beg", klineBeg(period, time.Now()))
	q.Set("end", "20500101")
	q.Set("lmt", strconv.Itoa(limit*3))
	q.Set("fields1", "f1,f2,f3,f4,f5,f6")
	q.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61")
	q.Set("_", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, klineEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return quote.Kline{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.hc.Do(req)
	if err != nil {
		return quote.Kline{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return quote.Kline{}, fmt.Errorf("K 线接口返回 %s", resp.Status)
	}

	var body struct {
		Data *struct {
			Name    string   `json:"name"`
			Decimal num      `json:"decimal"`
			Klines  []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return quote.Kline{}, fmt.Errorf("解析 K 线失败: %w", err)
	}
	if body.Data == nil {
		return quote.Kline{}, fmt.Errorf("K 线数据为空")
	}

	candles := parseKlines(body.Data.Klines)
	if len(candles) == 0 {
		return quote.Kline{}, fmt.Errorf("K 线暂无数据")
	}
	if len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}

	dec := int(body.Data.Decimal)
	if dec < 0 || dec > 4 {
		dec = 2
	}
	return quote.Kline{
		Symbol:   sym,
		Period:   period,
		Decimals: dec,
		Candles:  candles,
	}, nil
}

// klineBeg 按周期把起点前移，避免一次拉回全部历史。
func klineBeg(period string, now time.Time) string {
	switch period {
	case quote.PeriodWeek:
		return now.AddDate(-3, 0, 0).Format("20060102")
	case quote.PeriodMonth:
		return now.AddDate(-15, 0, 0).Format("20060102")
	case quote.Period60Min:
		return now.AddDate(0, 0, -60).Format("20060102")
	}
	return now.AddDate(0, 0, -400).Format("20060102")
}

// parseKlines 解析 日期,开,收,高,低,成交量,成交额,振幅,涨跌幅,涨跌额,换手率。
func parseKlines(lines []string) []quote.Candle {
	out := make([]quote.Candle, 0, len(lines))
	for _, line := range lines {
		f := strings.Split(line, ",")
		if len(f) < 6 {
			continue
		}
		at, ok := parseKlineTime(f[0])
		if !ok {
			continue
		}
		var pct float64
		if len(f) > 8 {
			pct = parseF(f[8])
		}
		out = append(out, quote.Candle{
			At:     at,
			Open:   parseF(f[1]),
			Close:  parseF(f[2]),
			High:   parseF(f[3]),
			Low:    parseF(f[4]),
			Volume: parseF(f[5]),
			Pct:    pct,
		})
	}
	return out
}

func parseKlineTime(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseF(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func marketOf(mkt int, code string) string {
	switch mkt {
	case 1:
		return "sh"
	case 0:
		if len(code) == 6 && (code[0] == '4' || code[0] == '8') {
			return "bj"
		}
		return "sz"
	case 116:
		return "hk"
	case 105, 106, 107:
		return "us"
	case 100:
		return "hk"
	}
	return quote.InferMarket(code)
}

type searchResp struct {
	Table struct {
		Data []struct {
			Code             string `json:"Code"`
			Name             string `json:"Name"`
			MktNum           string `json:"MktNum"`
			SecurityTypeName string `json:"SecurityTypeName"`
			QuoteID          string `json:"QuoteID"`
		} `json:"Data"`
	} `json:"QuotationCodeTable"`
}
