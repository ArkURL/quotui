package tencent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/arkurl/quotui/internal/quote"
)

const (
	quoteEndpoint  = "https://qt.gtimg.cn/q="
	searchEndpoint = "https://smartbox.gtimg.cn/s3/"
	userAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0 Safari/537.36"
	batchSize      = 50
	maxBody        = 1 << 20
)

// Client 是腾讯行情接口客户端，作为东财的备用源。
// 响应是 GBK 编码的 ~ 分隔文本，字段位置见下方常量。
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

func (c *Client) Name() string { return "腾讯" }

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
		codes := make([]string, 0, end-start)
		for _, s := range symbols[start:end] {
			codes = append(codes, requestCode(s))
		}
		text, err := c.get(ctx, quoteEndpoint+strings.Join(codes, ","))
		if err != nil {
			return out, err
		}
		out = append(out, parseQuotes(text)...)
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
	u := searchEndpoint + "?v=2&t=all&q=" + url.QueryEscape(kw)
	text, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseSearch(text, limit), nil
}

// getRaw 用于纯 JSON 接口：不做 GBK 转码，避免破坏转义字符。
func (c *Client) getRaw(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://gu.qq.com/")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("腾讯接口返回 %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
}

func (c *Client) get(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://gu.qq.com/")

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("腾讯接口返回 %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", err
	}
	// 行情与搜索接口都是 GBK，需要转成 UTF-8 才能正确展示中文。
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	if err != nil {
		return string(raw), nil
	}
	return string(decoded), nil
}

const minuteEndpoint = "https://web.ifzq.gtimg.cn/appstock/app/minute/query?code="

// Intraday 取分时。该接口只给每分钟的最新价，没有均价。
func (c *Client) Intraday(ctx context.Context, sym quote.Symbol) (quote.Intraday, error) {
	code := requestCode(sym)
	raw, err := c.getRaw(ctx, minuteEndpoint+code)
	if err != nil {
		return quote.Intraday{}, err
	}

	var body struct {
		Data map[string]struct {
			Data struct {
				Data []string `json:"data"`
				Date string   `json:"date"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return quote.Intraday{}, fmt.Errorf("解析分时失败: %w", err)
	}

	item, ok := body.Data[code]
	if !ok || len(item.Data.Data) == 0 {
		return quote.Intraday{}, fmt.Errorf("分时暂无数据")
	}
	day, err := time.Parse("20060102", item.Data.Date)
	if err != nil {
		day = time.Now()
	}

	out := quote.Intraday{
		Symbol:   sym,
		Date:     day.Format("2006-01-02"),
		Decimals: 2,
		Points:   make([]quote.Point, 0, len(item.Data.Data)),
	}
	for _, line := range item.Data.Data {
		f := strings.Fields(line)
		if len(f) < 2 || len(f[0]) != 4 {
			continue
		}
		hh, err1 := strconv.Atoi(f[0][:2])
		mm, err2 := strconv.Atoi(f[0][2:])
		if err1 != nil || err2 != nil {
			continue
		}
		out.Points = append(out.Points, quote.Point{
			At:    time.Date(day.Year(), day.Month(), day.Day(), hh, mm, 0, 0, time.Local),
			Price: fnum(f[1]),
		})
	}
	if len(out.Points) == 0 {
		return quote.Intraday{}, fmt.Errorf("分时暂无数据")
	}
	return out, nil
}

const klineEndpoint = "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param="

// Kline 取 K 线。该接口不支持分钟级，分钟线只有东财能提供。
func (c *Client) Kline(ctx context.Context, sym quote.Symbol, period string, limit int) (quote.Kline, error) {
	if period == quote.Period60Min {
		return quote.Kline{}, fmt.Errorf("腾讯接口不支持分钟 K 线")
	}
	if limit <= 0 {
		limit = 120
	}
	code := requestCode(sym)
	u := klineEndpoint + code + "," + period + ",,," + strconv.Itoa(limit) + ",qfq"

	raw, err := c.getRaw(ctx, u)
	if err != nil {
		return quote.Kline{}, err
	}

	var body struct {
		Code int                                   `json:"code"`
		Data map[string]map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return quote.Kline{}, fmt.Errorf("解析 K 线失败: %w", err)
	}
	if body.Code != 0 {
		return quote.Kline{}, fmt.Errorf("K 线接口返回码 %d", body.Code)
	}

	item, ok := body.Data[code]
	if !ok {
		return quote.Kline{}, fmt.Errorf("K 线数据为空")
	}
	// 复权数据键名是 qfqday / qfqweek，港股等无复权键时退化为 day。
	for _, key := range []string{"qfq" + period, period} {
		payload, ok := item[key]
		if !ok {
			continue
		}
		candles := parseTencentCandles(payload)
		if len(candles) == 0 {
			continue
		}
		if len(candles) > limit {
			candles = candles[len(candles)-limit:]
		}
		return quote.Kline{
			Symbol:   sym,
			Period:   period,
			Decimals: decimalsFor(sym.Market),
			Candles:  candles,
		}, nil
	}
	return quote.Kline{}, fmt.Errorf("K 线数据为空")
}

// parseTencentCandles 解析 [日期,开,收,高,低,量]。
func parseTencentCandles(raw json.RawMessage) []quote.Candle {
	var rows [][]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil
	}
	out := make([]quote.Candle, 0, len(rows))
	for _, r := range rows {
		if len(r) < 6 {
			continue
		}
		at, ok := parseDay(jsonText(r[0]))
		if !ok {
			continue
		}
		out = append(out, quote.Candle{
			At:     at,
			Open:   candleNum(r[1]),
			Close:  candleNum(r[2]),
			High:   candleNum(r[3]),
			Low:    candleNum(r[4]),
			Volume: candleNum(r[5]),
		})
	}
	return out
}

func jsonText(r json.RawMessage) string {
	if s, err := strconv.Unquote(string(r)); err == nil {
		return s
	}
	return string(r)
}

func candleNum(r json.RawMessage) float64 {
	return fnum(jsonText(r))
}

func parseDay(s string) (time.Time, bool) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// 腾讯行情字段下标（~ 分隔）。
const (
	idxName      = 1
	idxCode      = 2
	idxPrice     = 3
	idxPrevClose = 4
	idxOpen      = 5
	idxVolume    = 6
	idxChange    = 31
	idxChangePct = 32
	idxHigh      = 33
	idxLow       = 34
	idxAmount    = 37 // 万元
	idxTurnover  = 38
	idxPE        = 39
	idxFloatCap  = 44 // 亿元
	idxMarketCap = 45 // 亿元
	idxPB        = 46
	minFields    = 47
)

func parseQuotes(body string) []quote.Quote {
	out := make([]quote.Quote, 0, 8)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "v_") {
			continue
		}
		eq := strings.Index(line, "=\"")
		if eq < 0 {
			continue
		}
		key := line[2:eq]
		val := strings.TrimSuffix(line[eq+2:], ";")
		val = strings.TrimSuffix(val, "\"")

		f := strings.Split(val, "~")
		if len(f) < minFields {
			continue
		}
		market, code := splitKey(key)
		if market == "" || code == "" {
			continue
		}

		name := strings.TrimSpace(f[idxName])
		if market == "sh" || market == "sz" || market == "bj" {
			// A 股名称按两字对齐补了空格，展示前去掉。
			name = strings.ReplaceAll(name, " ", "")
		}
		sym := quote.Symbol{
			Code:   code,
			Market: market,
			Name:   name,
			SecID:  secIDFor(market, code, f[idxCode]),
		}
		// 成交额单位按市场区分：A 股接口给的是万元，港美股给的是元。
		amount := fnum(f[idxAmount])
		if market == "sh" || market == "sz" || market == "bj" {
			amount *= 1e4
		}

		q := quote.Quote{
			Symbol:    sym,
			Name:      sym.Name,
			Price:     fnum(f[idxPrice]),
			PrevClose: fnum(f[idxPrevClose]),
			Open:      fnum(f[idxOpen]),
			High:      fnum(f[idxHigh]),
			Low:       fnum(f[idxLow]),
			Change:    fnum(f[idxChange]),
			ChangePct: fnum(f[idxChangePct]),
			Volume:    fnum(f[idxVolume]),
			Amount:    amount,
			Turnover:  fnum(f[idxTurnover]),
			PE:        fnum(f[idxPE]),
			MarketCap: fnum(f[idxMarketCap]) * 1e8,
			Decimals:  decimalsFor(market),
		}
		// 港美股的第 46 位是英文名而不是市净率。
		if market == "sh" || market == "sz" || market == "bj" {
			q.PB = fnum(f[idxPB])
		}
		q.Suspended = q.Price == 0
		out = append(out, q)
	}
	return out
}

func parseSearch(body string, limit int) []quote.Symbol {
	start := strings.Index(body, "\"")
	end := strings.LastIndex(body, "\"")
	if start < 0 || end <= start {
		return nil
	}
	out := make([]quote.Symbol, 0, limit)
	for _, entry := range strings.Split(body[start+1:end], "^") {
		parts := strings.Split(entry, "~")
		if len(parts) < 4 {
			continue
		}
		market := parts[0]
		if market != "sh" && market != "sz" && market != "bj" && market != "hk" && market != "us" {
			continue
		}
		kind := ""
		if len(parts) > 4 {
			kind = parts[4]
		}
		if !searchableKind(kind) {
			continue
		}
		code := strings.ToUpper(stripSuffix(parts[1]))
		name := unescapeUnicode(parts[2])
		if code == "" || name == "" {
			continue
		}
		out = append(out, quote.Symbol{
			Code:   code,
			Market: market,
			Name:   name,
			SecID:  secIDFor(market, code, parts[1]),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// searchableKind 只要股票、指数和 ETF，过滤掉权证等品种。
func searchableKind(kind string) bool {
	switch {
	case kind == "":
		return true
	case strings.HasPrefix(kind, "GP"):
		return true
	case kind == "ZS", kind == "ETF", kind == "JJ":
		return true
	}
	return false
}

func requestCode(s quote.Symbol) string {
	switch s.Market {
	case "sh", "sz", "bj":
		return s.Market + s.Code
	case "hk":
		return "hk" + padLeft(s.Code, 5)
	case "us":
		return "us" + strings.ToUpper(s.Code)
	}
	return "sh" + s.Code
}

func splitKey(key string) (market, code string) {
	if len(key) < 3 {
		return "", ""
	}
	switch key[:2] {
	case "sh", "sz", "bj", "hk", "us":
		return key[:2], key[2:]
	}
	return "", ""
}

// secIDFor 把腾讯的美股交易所后缀翻译成东财的市场编号，便于跨源复用。
func secIDFor(market, code, raw string) string {
	if market != "us" {
		return ""
	}
	upper := strings.ToUpper(raw)
	switch {
	case strings.HasSuffix(upper, ".OQ"):
		return "105." + code
	case strings.HasSuffix(upper, ".N"):
		return "106." + code
	case strings.HasSuffix(upper, ".A"):
		return "107." + code
	}
	return ""
}

func stripSuffix(code string) string {
	if i := strings.Index(code, "."); i > 0 {
		return code[:i]
	}
	return code
}

func decimalsFor(market string) int {
	if market == "hk" {
		return 3
	}
	return 2
}

func fnum(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat("0", n-len(s)) + s
}

func unescapeUnicode(s string) string {
	if !strings.Contains(s, "\\u") {
		return s
	}
	if out, err := strconv.Unquote("\"" + s + "\""); err == nil {
		return out
	}
	return s
}
