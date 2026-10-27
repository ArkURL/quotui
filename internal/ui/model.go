package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/arkurl/quotui/internal/config"
	"github.com/arkurl/quotui/internal/market"
	"github.com/arkurl/quotui/internal/provider"
	"github.com/arkurl/quotui/internal/quote"
)

const (
	fetchTimeout = 10 * time.Second
	// manualGap 限制手动刷新的最小间隔，避免连按 r 打爆接口。
	manualGap = time.Second
	// detailGap 限制详情页分时的刷新间隔，避免和列表请求叠加。
	detailGap = 10 * time.Second
	// klineGap 是 K 线的刷新间隔，日线级别不需要太勤。
	klineGap = 30 * time.Second
	// toastDuration 是右下角提示的停留时长。
	toastDuration = 3 * time.Second
	// backoffInterval 连续失败后的退避周期。
	backoffInterval = 30 * time.Second
)

// refreshPresets 是可选的刷新周期（秒），0 表示只手动刷新。
var refreshPresets = []int{0, 1, 2, 3, 5, 10, 30, 60}

type msgQuotes struct {
	quotes []quote.Quote
	at     time.Time
}

type msgFetchErr struct{ err error }

type msgTick struct {
	gen int
	at  time.Time
}

type msgSearch struct {
	results []quote.Symbol
	err     error
}

type msgIntraday struct {
	key  string
	data quote.Intraday
	at   time.Time
}

type msgIntradayErr struct {
	key string
	err error
}

type msgKline struct {
	key    string
	period string
	data   quote.Kline
	at     time.Time
}

type msgToastExpired struct{ gen int }

type msgKlineErr struct {
	key    string
	period string
	err    error
}

type addState struct {
	active    bool
	input     textinput.Model
	results   []quote.Symbol
	cursor    int
	searching bool
	searched  bool
	err       error
}

type pickerState struct {
	active bool
	cursor int
}

type chartMode int

const (
	chartIntraday chartMode = iota
	chartKline
)

var chartPeriods = []string{quote.PeriodDay, quote.PeriodWeek, quote.PeriodMonth, quote.Period60Min}

type klineState struct {
	data    quote.Kline
	err     error
	loading bool
	at      time.Time
}

type detailState struct {
	active  bool
	key     string
	data    quote.Intraday
	err     error
	loading bool
	at      time.Time
}

// Model 是 quotui 的主界面状态。
type Model struct {
	cfg  *config.Config
	path string
	prov provider.Provider

	quotes     map[string]quote.Quote
	fetchedAt  time.Time
	fetchErr   error
	loading    bool
	fetching   bool
	failStreak int

	// lastRequestAt 记录最近一次发起请求的时间，用于手动刷新冷却。
	lastRequestAt time.Time
	// tickGen 让旧的定时器失效：周期变更时自增，迟到的 tick 会被丢弃。
	tickGen int

	cursor   int
	sortBy   string
	sortDesc bool

	paused bool
	now    time.Time

	width, height int
	showHelp      bool
	toast         string
	toastAt       time.Time
	toastGen      int

	add    addState
	picker pickerState
	detail detailState

	chartMode chartMode
	period    string
	kline     klineState
}

func New(cfg *config.Config, path string, prov provider.Provider) *Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Placeholder = "代码 / 名称 / 拼音"
	ti.CharLimit = 24
	return &Model{
		cfg:      cfg,
		path:     path,
		prov:     prov,
		quotes:   make(map[string]quote.Quote),
		sortBy:   cfg.SortBy,
		sortDesc: cfg.SortDesc,
		cursor:   cfg.Cursor,
		now:      time.Now(),
		add:      addState{input: ti},
		period:   quote.PeriodDay,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), m.tickCmd())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampCursor()
		return m, nil

	case msgTick:
		// 周期被改过之后，旧定时器迟到的 tick 直接丢弃。
		if msg.gen != m.tickGen {
			return m, nil
		}
		m.now = msg.at
		cmds := []tea.Cmd{m.tickCmd()}
		if !m.paused {
			cmds = append(cmds, m.fetchCmd())
		}
		if m.detail.active {
			if m.chartMode == chartKline {
				if time.Since(m.kline.at) >= klineGap {
					cmds = append(cmds, m.klineCmd())
				}
			} else if time.Since(m.detail.at) >= detailGap {
				cmds = append(cmds, m.intradayCmd())
			}
		}
		return m, tea.Batch(cmds...)

	case msgQuotes:
		m.applyQuotes(msg.quotes)
		m.fetchedAt = msg.at
		m.fetchErr = nil
		m.loading = false
		m.fetching = false
		m.failStreak = 0
		return m, nil

	case msgFetchErr:
		m.fetchErr = msg.err
		m.loading = false
		m.fetching = false
		m.failStreak++
		return m, nil

	case msgToastExpired:
		// 代际不符说明期间弹过新提示，旧定时器不该把它清掉。
		if msg.gen == m.toastGen {
			m.toast = ""
		}
		return m, nil

	case msgKline:
		if msg.key != m.detail.key || msg.period != m.period {
			return m, nil
		}
		m.kline.data = msg.data
		m.kline.err = nil
		m.kline.loading = false
		m.kline.at = msg.at
		return m, nil

	case msgKlineErr:
		if msg.key != m.detail.key || msg.period != m.period {
			return m, nil
		}
		m.kline.err = msg.err
		m.kline.loading = false
		m.kline.at = time.Now()
		return m, nil

	case msgIntraday:
		if msg.key != m.detail.key {
			return m, nil
		}
		m.detail.data = msg.data
		m.detail.err = nil
		m.detail.loading = false
		m.detail.at = msg.at
		return m, nil

	case msgIntradayErr:
		if msg.key != m.detail.key {
			return m, nil
		}
		m.detail.err = msg.err
		m.detail.loading = false
		m.detail.at = time.Now()
		return m, nil

	case msgSearch:
		m.add.searching = false
		m.add.searched = true
		m.add.results = msg.results
		m.add.err = msg.err
		m.add.cursor = 0
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.add.active {
		var cmd tea.Cmd
		m.add.input, cmd = m.add.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.picker.active {
		return m.handlePickerKey(k)
	}
	if m.add.active {
		return m.handleAddKey(k)
	}
	if m.showHelp {
		if k.String() == "q" || k.String() == "ctrl+c" {
			return m.quit()
		}
		m.showHelp = false
		return m, nil
	}
	if m.detail.active {
		return m.handleDetailKey(k)
	}

	rows := m.visible()
	switch k.String() {
	case "q", "ctrl+c":
		return m.quit()
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(rows)-1 {
			m.cursor++
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(rows) - 1
	case "r":
		return m, m.manualFetch()
	case "t":
		m.openPicker()
	case " ":
		m.paused = !m.paused
		if m.paused {
			return m, m.setToast("已暂停刷新")
		}
		return m, tea.Batch(m.setToast("已恢复刷新"), m.fetchCmd())
	case "s":
		return m, m.cycleSort()
	case "S":
		if m.sortBy != "order" {
			m.sortDesc = !m.sortDesc
			m.saveView()
		}
	case "a":
		return m, m.openAdd()
	case "enter":
		return m, m.openDetail()
	case "d":
		return m, m.deleteCurrent(rows)
	case "J":
		m.moveCurrent(1)
	case "K":
		m.moveCurrent(-1)
	case "?":
		m.showHelp = true
	}
	m.clampCursor()
	return m, nil
}

// setToast 显示一条临时提示，并在到点后自动恢复成帮助文案。
// 用独立定时器而不是挂靠数据刷新，否则切到「仅手动」后就不会再有 tick 来清理。
func (m *Model) setToast(text string) tea.Cmd {
	m.toast = text
	m.toastAt = time.Now()
	m.toastGen++
	gen := m.toastGen
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return msgToastExpired{gen: gen}
	})
}

// manualFetch 手动刷新，带冷却与去重，避免连按导致请求风暴。
func (m *Model) manualFetch() tea.Cmd {
	if m.fetching {
		return m.setToast("正在刷新…")
	}
	if !m.lastRequestAt.IsZero() && time.Since(m.lastRequestAt) < manualGap {
		return m.setToast("刷新过于频繁，稍等一下")
	}
	cmd := m.fetchCmd()
	if cmd == nil {
		return m.setToast("自选为空，按 a 添加")
	}
	return cmd
}

func (m *Model) handleDetailKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "enter", "backspace":
		m.detail.active = false
		return m, nil
	case "q", "ctrl+c":
		return m.quit()
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			return m, m.openDetail()
		}
	case "down", "j":
		if m.cursor < len(m.visible())-1 {
			m.cursor++
			return m, m.openDetail()
		}
	case "r":
		return m, m.chartCmd()
	case "c":
		if m.chartMode == chartKline {
			m.chartMode = chartIntraday
		} else {
			m.chartMode = chartKline
		}
		m.resetChart()
		return m, m.chartCmd()
	case "p":
		if m.chartMode != chartKline {
			return m, nil
		}
		for i, p := range chartPeriods {
			if p == m.period {
				m.period = chartPeriods[(i+1)%len(chartPeriods)]
				break
			}
		}
		m.resetChart()
		return m, m.klineCmd()
	}
	return m, nil
}

// chartCmd 按当前视图拉取对应的图表数据。
func (m *Model) chartCmd() tea.Cmd {
	if m.chartMode == chartKline {
		return m.klineCmd()
	}
	return m.intradayCmd()
}

func (m *Model) resetChart() {
	m.detail.data = quote.Intraday{}
	m.detail.err = nil
	m.detail.loading = false
	m.detail.at = time.Time{}
	m.kline.data = quote.Kline{}
	m.kline.err = nil
	m.kline.loading = false
	m.kline.at = time.Time{}
}

func (m *Model) klineCmd() tea.Cmd {
	if m.kline.loading {
		return nil
	}
	rows := m.visible()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return nil
	}
	sym := rows[m.cursor]
	key, period := sym.Key(), m.period

	m.detail.key = key
	m.kline.loading = true

	prov := m.prov
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		k, err := prov.Kline(ctx, sym, period, 120)
		if err != nil {
			return msgKlineErr{key: key, period: period, err: err}
		}
		return msgKline{key: key, period: period, data: k, at: time.Now()}
	}
}

// openDetail 进入详情页并拉取当前标的的分时。
func (m *Model) openDetail() tea.Cmd {
	rows := m.visible()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return nil
	}
	m.detail.active = true
	m.detail.key = rows[m.cursor].Key()
	m.resetChart()
	return m.chartCmd()
}

func (m *Model) intradayCmd() tea.Cmd {
	if m.detail.loading {
		return nil
	}
	rows := m.visible()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return nil
	}
	sym := rows[m.cursor]
	key := sym.Key()

	m.detail.key = key
	m.detail.loading = true

	prov := m.prov
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		d, err := prov.Intraday(ctx, sym)
		if err != nil {
			return msgIntradayErr{key: key, err: err}
		}
		return msgIntraday{key: key, data: d, at: time.Now()}
	}
}

func (m *Model) openPicker() {
	m.picker.active = true
	m.picker.cursor = 0
	for i, v := range refreshPresets {
		if v == m.cfg.RefreshSeconds {
			m.picker.cursor = i
			break
		}
	}
}

func (m *Model) handlePickerKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "t", "q":
		m.picker.active = false
	case "up", "k":
		if m.picker.cursor > 0 {
			m.picker.cursor--
		}
	case "down", "j":
		if m.picker.cursor < len(refreshPresets)-1 {
			m.picker.cursor++
		}
	case "enter":
		return m, m.applyRefreshPreset()
	}
	return m, nil
}

func (m *Model) applyRefreshPreset() tea.Cmd {
	sec := refreshPresets[m.picker.cursor]
	m.picker.active = false
	m.cfg.RefreshSeconds = sec
	_ = config.Save(m.path, m.cfg)

	// 换周期：让旧的 tick 链失效，再按新周期重启。
	m.tickGen++

	if sec == 0 {
		return m.setToast("已切换为手动刷新，按 r 拉取")
	}
	return tea.Batch(
		m.setToast(fmt.Sprintf("刷新周期 %s", intervalLabel(sec))),
		m.tickCmd(),
		m.fetchCmd(),
	)
}

// intervalLabel 把秒数转成展示文案。
func intervalLabel(sec int) string {
	if sec <= 0 {
		return "手动"
	}
	return fmt.Sprintf("%ds", sec)
}

func (m *Model) handleAddKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.closeAdd()
		return m, nil
	case "enter":
		if len(m.add.results) > 0 {
			return m, m.commitAdd()
		}
		return m, m.searchCmd()
	case "up", "ctrl+p":
		if m.add.cursor > 0 {
			m.add.cursor--
		}
		return m, nil
	case "down", "ctrl+n", "tab":
		if m.add.cursor < len(m.add.results)-1 {
			m.add.cursor++
		}
		return m, nil
	}

	if len(m.add.results) > 0 {
		m.add.results = nil
		m.add.searched = false
	}
	m.add.err = nil
	var cmd tea.Cmd
	m.add.input, cmd = m.add.input.Update(k)
	return m, cmd
}

func (m *Model) openAdd() tea.Cmd {
	m.add.active = true
	m.add.results = nil
	m.add.cursor = 0
	m.add.err = nil
	m.add.searched = false
	m.add.searching = false
	m.add.input.SetValue("")
	m.add.input.Width = min(m.width-10, 56)
	if m.add.input.Width < 16 {
		m.add.input.Width = 16
	}
	return m.add.input.Focus()
}

func (m *Model) closeAdd() {
	m.add.active = false
	m.add.input.Blur()
	m.add.input.SetValue("")
	m.add.results = nil
	m.add.err = nil
	m.add.searched = false
	m.add.searching = false
}

func (m *Model) searchCmd() tea.Cmd {
	kw := strings.TrimSpace(m.add.input.Value())
	if kw == "" {
		return nil
	}
	m.add.searching = true
	m.add.searched = false
	m.add.err = nil

	existing := make(map[string]bool, len(m.cfg.Watchlist))
	for _, s := range m.cfg.Watchlist {
		existing[s.Key()] = true
	}
	prov := m.prov

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		res, err := prov.Search(ctx, kw, 12)
		if err != nil {
			return msgSearch{err: err}
		}
		filtered := make([]quote.Symbol, 0, len(res))
		for _, s := range res {
			if !existing[s.Key()] {
				filtered = append(filtered, s)
			}
		}
		return msgSearch{results: filtered}
	}
}

func (m *Model) commitAdd() tea.Cmd {
	if m.add.cursor < 0 || m.add.cursor >= len(m.add.results) {
		return nil
	}
	sym := m.add.results[m.add.cursor]
	m.cfg.Watchlist = append(m.cfg.Watchlist, sym)
	m.sortBy = "order"
	m.cursor = len(m.cfg.Watchlist) - 1
	_ = config.Save(m.path, m.cfg)
	m.closeAdd()
	return tea.Batch(m.setToast("已添加 "+sym.Name), m.fetchCmd())
}

func (m *Model) deleteCurrent(rows []quote.Symbol) tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(rows) {
		return nil
	}
	target := rows[m.cursor].Key()
	label := m.name(rows[m.cursor])

	for i, s := range m.cfg.Watchlist {
		if s.Key() == target {
			m.cfg.Watchlist = append(m.cfg.Watchlist[:i], m.cfg.Watchlist[i+1:]...)
			break
		}
	}
	delete(m.quotes, target)
	_ = config.Save(m.path, m.cfg)
	m.clampCursor()
	return m.setToast("已删除 " + label)
}

// moveCurrent 只在自定义顺序下有意义，调用后会切回自定义顺序。
func (m *Model) moveCurrent(delta int) {
	m.sortBy = "order"
	for i := range m.cfg.Watchlist {
		if i == m.cursor {
			j := i + delta
			if j < 0 || j >= len(m.cfg.Watchlist) {
				return
			}
			m.cfg.Watchlist[i], m.cfg.Watchlist[j] = m.cfg.Watchlist[j], m.cfg.Watchlist[i]
			m.cursor = j
			m.saveView()
			return
		}
	}
}

func (m *Model) cycleSort() tea.Cmd {
	order := []string{"order", "pct", "amount", "code"}
	for i, k := range order {
		if k == m.sortBy {
			m.sortBy = order[(i+1)%len(order)]
			break
		}
	}
	m.sortDesc = m.sortBy == "pct" || m.sortBy == "amount"
	m.saveView()
	return m.setToast("排序: " + m.sortLabel())
}

// quit 落盘界面状态后退出。
func (m *Model) quit() (tea.Model, tea.Cmd) {
	m.cfg.Cursor = m.cursor
	m.saveView()
	return m, tea.Quit
}

// saveView 把排序与游标写回配置，重启后保持一致。
func (m *Model) saveView() {
	m.cfg.SortBy = m.sortBy
	m.cfg.SortDesc = m.sortDesc
	_ = config.Save(m.path, m.cfg)
}

func (m *Model) sortLabel() string {
	name := map[string]string{
		"order":  "自定义",
		"pct":    "涨幅",
		"amount": "成交额",
		"code":   "代码",
	}[m.sortBy]
	if m.sortBy == "order" {
		return name
	}
	if m.sortDesc {
		return name + "↓"
	}
	return name + "↑"
}

func (m *Model) fetchCmd() tea.Cmd {
	syms := append([]quote.Symbol(nil), m.cfg.Watchlist...)
	if len(syms) == 0 || m.fetching {
		return nil
	}
	m.fetching = true
	m.loading = true
	m.lastRequestAt = time.Now()
	prov := m.prov
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		qs, err := prov.Quotes(ctx, syms)
		if err != nil {
			return msgFetchErr{err: err}
		}
		return msgQuotes{quotes: qs, at: time.Now()}
	}
}

func (m *Model) tickCmd() tea.Cmd {
	d := m.interval()
	if d <= 0 {
		return nil // 仅手动模式，不排定时器
	}
	gen := m.tickGen
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return msgTick{gen: gen, at: t}
	})
}

// interval 返回实际生效的刷新周期，0 表示只手动刷新。
func (m *Model) interval() time.Duration {
	sec := m.cfg.RefreshSeconds
	if sec <= 0 {
		return 0
	}
	base := time.Duration(sec) * time.Second
	// 全部市场休市时降频，避免无意义的请求。
	if !market.IsAnyOpen(m.now) && base < backoffInterval {
		return backoffInterval
	}
	// 连续失败时退避，避免在接口异常时反复重试。
	if m.failStreak >= 2 && base < backoffInterval {
		return backoffInterval
	}
	return base
}

func (m *Model) applyQuotes(qs []quote.Quote) {
	for _, q := range qs {
		m.quotes[q.Symbol.Key()] = q
	}
	if m.cfg.Learn(qs) {
		_ = config.Save(m.path, m.cfg)
	}
}

func (m *Model) name(s quote.Symbol) string {
	if s.Name != "" {
		return s.Name
	}
	if q, ok := m.quotes[s.Key()]; ok && q.Name != "" {
		return q.Name
	}
	return s.Code
}

// visible 返回按当前排序规则排好的自选股列表。
func (m *Model) visible() []quote.Symbol {
	syms := append([]quote.Symbol(nil), m.cfg.Watchlist...)
	if m.sortBy == "" || m.sortBy == "order" || len(syms) < 2 {
		return syms
	}

	value := func(s quote.Symbol) float64 {
		q, ok := m.quotes[s.Key()]
		if !ok {
			return 0
		}
		switch m.sortBy {
		case "pct":
			return q.ChangePct
		case "amount":
			return q.Amount
		}
		return 0
	}

	sort.SliceStable(syms, func(i, j int) bool {
		switch m.sortBy {
		case "code":
			return syms[i].Code < syms[j].Code
		}
		a, b := value(syms[i]), value(syms[j])
		if m.sortDesc {
			return a > b
		}
		return a < b
	})
	return syms
}

func (m *Model) clampCursor() {
	n := len(m.cfg.Watchlist)
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}
