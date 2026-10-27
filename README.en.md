[简体中文](README.md) | English

# quotui

[![CI](https://github.com/ArkURL/quotui/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ArkURL/quotui/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ArkURL/quotui?color=3FBF6F)](https://github.com/ArkURL/quotui/releases)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-6e6e82)](#installation)
[![Bubble Tea](https://img.shields.io/badge/Made%20with-Bubble%20Tea-ff69b4)](https://github.com/charmbracelet/bubbletea)
[![License](https://img.shields.io/badge/License-MIT-3FBF6F)](LICENSE)

> A terminal stock watcher: one binary, no runtime dependencies, just your watchlist.

```
quotui  自选股 6                                              排序 自定义
────────────────────────────────────────────────────────────────────────
  代码     名称              现价      涨跌      涨幅     成交额    换手
▌ 600519   贵州茅台         1349.98    +14.98    +1.12%    55.74亿   0.33%
  000001   平安银行           11.30     +0.10    +0.89%    14.86亿   0.68%
  300750   宁德时代          238.00     +0.40    +0.17%   146.17亿   0.94%
────────────────────────────────────────────────────────────────────────
3s · 东财 · 更新 15:33:20 · A股 交易中 · 港股 休市 · 美股 休市
```

<sub>Interface labels are currently Chinese; see [Limitations](#limitations).</sub>

## Features

- **Multi-market**: A-shares (Shanghai / Shenzhen / Beijing), Hong Kong, US, plus indices.
- **Watchlist**: search by code, name or pinyin; delete; reorder; sort by any column. Everything persists locally.
- **Detail view**: intraday chart and candlesticks (daily / weekly / monthly / 60 min), toggled with `c` and `p`.
- **Frugal with requests**: batched quotes, gap filling, circuit breaker, off-hours throttling, and a manual-only mode.
- **Dual source**: Eastmoney primary, Tencent fallback, with automatic failover announced in the status bar.
- **Narrow terminals**: columns are dropped by priority and CJK widths are aligned; usable at 40 columns.
- **Scriptable**: `-once` prints a plain-text snapshot, ready for a tmux status line.

## Installation

No Go toolchain? Grab the archive for your platform from [Releases](https://github.com/ArkURL/quotui/releases) and run it.

```bash
# requires Go 1.27 or newer
go install github.com/arkurl/quotui@latest

# or build from source
git clone git@github.com:ArkURL/quotui.git
cd quotui
go build -o quotui .
```

The result is a single static binary with no runtime dependencies.

> With several copies on the machine, the first entry of `which -a quotui` is the one that runs. Press `?` — the help footer shows the version, which is the quickest way to confirm. An older build will drop config keys it does not know about when it saves.

## Quick start

```bash
quotui                  # open the TUI
quotui -once            # fetch once, print plain text, exit
quotui -json            # fetch once, print JSON (implies -once)
quotui -config ./c.toml # use a custom config file
quotui -version         # print version
```

On first run a default config is written to the system config directory, preloaded with six symbols:

- macOS: `~/Library/Application Support/quotui/config.toml`
- Linux: `~/.config/quotui/config.toml`
- Windows: `%AppData%\quotui\config.toml`

### Scripting

`-json` emits a structured snapshot that pipes straight into jq:

```bash
# symbols up more than 5%
quotui -json | jq -r '.quotes[] | select(.change_pct > 5) | "\(.name) \(.change_pct)%"'
```

Fields per symbol: `code` `market` `name` `price` `prev_close` `open` `high` `low` `change` `change_pct` `volume` `amount` `turnover` `pe` `pb` `market_cap` `suspended` `decimals`.
When a symbol is suspended or has no price, `price` is `null` and `suspended` is `true`, so it cannot be confused with a real zero.

The top level also carries `updated` (RFC3339), `source` (the data source currently serving), `count`, and `missing` (such as `hk/00700`) when some symbols could not be fetched.
Volume units follow the market: lots for A-shares, shares for Hong Kong and US.

## Key bindings

| Key | Action |
|---|---|
| `↑` / `k`, `↓` / `j` | Move |
| `g` / `G` | First / last row |
| `Enter` | Open detail view; press again to return |
| `c` | Toggle intraday / candlestick in the detail view |
| `p` | Switch candle period (daily / weekly / monthly / 60 min) |
| `Esc` | Back to the list |
| `r` | Refresh now (1 second cooldown) |
| `t` | Pick a refresh interval (including manual only) |
| `C` | Up/down colours (4 presets, custom, restore defaults) |
| `Space` | Pause / resume auto refresh |
| `s` / `S` | Cycle sort field / toggle direction |
| `a` / `d` | Add / remove a symbol |
| `J` / `K` | Move the row within the custom order |
| `?` | Help |
| `q` | Quit |

## Configuration

```toml
refresh_seconds = 3       # 3s auto refresh; 0 means manual only (press r)
color_scheme = "red_up"   # red_up (red = up, A-share convention) or green_up
up_color = ""             # custom up colour, e.g. "#B08968"; empty follows color_scheme
down_color = ""           # custom down colour, e.g. "#6D8A96"
sort_by = "order"         # order / pct / amount / code
sort_desc = true
columns = ["code", "name", "price", "chg", "pct", "amount", "turnover"]
holidays = []             # extra closed days, e.g. ["2027-01-01"]

[[watchlist]]
  code = "600519"
  market = "sh"           # sh / sz / bj / hk / us
  name = "贵州茅台"
```

- `columns` sets which columns appear and in what order. Valid keys: `code`, `name`, `price`, `chg` (change), `pct` (percent), `amount`, `turnover`. On narrow terminals columns are dropped in the order turnover, amount, chg, price, code; name and pct always stay.
- `secid` is optional and pins the venue id for HK and US symbols (for example `105.AAPL`). It is written automatically when you add a symbol with `a`, and backfilled from the fallback source if left empty.
- Sort field, direction and cursor position are persisted, so a restart looks the same.
- Press `C` for the colour panel. Four presets are built in and applied immediately:

  | Preset | Up | Down |
  |---|---|---|
  | A-shares, red up | `#F2575B` | `#3FBF6F` |
  | HK/US, green up | `#3FBF6F` | `#F2575B` |
  | Muted amber / slate | `#B08968` | `#6D8A96` |
  | Colour-blind safe | `#E69F00` | `#56B4E9` |

- The first two are expressed through `color_scheme` (clearing custom colours); the other two write `up_color` / `down_color`. Each colour row also accepts typed input: `#RGB` and `#RRGGBB` work, invalid input is ignored and falls back to `color_scheme`.
- "Restore defaults" clears the custom colours and returns `color_scheme` to `red_up`. The active preset is marked; after editing a colour by hand no preset matches.
- `holidays` supplements the built-in A-share calendar for years it does not cover (see [Limitations](#limitations)).

## Detail view

Press `Enter` on a row; `Esc` returns. Use `j` / `k` to move straight to the next or previous symbol without going back.

### Intraday

```
贵州茅台  600519  沪                                                        15:33:20
────────────────────────────────────────────────────────────────────────────────────
1349.98  +14.98  +1.12%
今开   1325.40    最高   1350.00    最低   1324.79    昨收   1335.00    成交额 55.74亿
换手   0.33%      量比   1.00       市盈   18.95
                                                        ││││││││││•
                                              ││  ││  │││││││││
                                         │  ││││││││││   │  │
                              ││  ││  ││││││││││
                  ││ ││ ││││││││││││││
│││││││││││││││││││││││││││││││││││││││││││││││││││││││││
┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈
09:30                                                                          15:00
```

- Each column shows the price range of the samples mapped to it; the trailing `•` is the last price, cyan `·` is the average price, and the dashed line is the previous close.
- The y-axis fits the session range, the x-axis is stretched across samples, and both ends show real timestamps (US symbols are shown in Beijing time).

### Candlesticks

Press `c` in the detail view, then `p` to cycle daily / weekly / monthly / 60 min.

```
贵州茅台  600519  沪  日K                                                       15:33:20
────────────────────────────────────────────────────────────────────────────────────
1349.98  +14.98  +1.12%
今开   1325.40    最高   1350.00    最低   1324.79    昨收   1335.00    成交额 55.74亿
换手   0.33%      量比   1.00       市盈   18.95
                                                                         │         █
                                                             │           │       │ █
                                                           █ │         █ │ █ █   █ █
                                                           █ █ │       █ █ █ █ │ █ █
                                                         █ █ █       █ █   │ █ █ █
                                     █                 █ █   █       █ │     █ █ │
                                   █ █               █ █     █       │ │     │ │
                                 █ █ █   │           █           █ █ █ █
                     │         █       █ │     │   │ │         █ █ █ │
08-12                                                                          10-26
```

- Bodies are solid blocks and wicks are vertical bars. Red means up by default; `green_up` swaps them.
- Each candle takes two columns (body plus gap) and falls back to one column when space is tight. Newest candle is pinned to the right edge.
- Daily candles use forward-adjusted prices. Weekly and monthly look back far enough and then keep the latest 120 bars.

> Red and green drawing too much attention? Press `C` and pick the muted amber/slate preset: still distinguishable, far less obvious. For red-green colour blindness, the orange/blue preset is a better fit.

## Refresh policy

Pick manual-only with `t` when polling is not welcome; after that, data is fetched only when you press `r`.

- Intervals: manual / 1 / 2 / 3 / 5 / 10 / 30 / 60 seconds, saved to the config file.
- Manual refresh has a one second cooldown, so repeated `r` presses are rejected with a hint.
- A new request is never issued while the previous one is still in flight.
- When every market is closed, an automatic interval is throttled to at most 30 seconds.
- After two consecutive failures the interval backs off to 30 seconds and returns to your choice once it recovers.
- In the detail view, intraday refreshes at most every 10 seconds and candles every 30 seconds, independently of the list.

## Data sources

Eastmoney is the primary source and Tencent the fallback. Any failure (timeout, rate limit, empty payload) triggers a switch, and the status bar shows which source is serving and whether one is tripped.

| Market | Eastmoney | Tencent |
|---|---|---|
| A-shares | `push2` · `1.` Shanghai, `0.` Shenzhen/Beijing | `qt.gtimg.cn` · `sh` / `sz` / `bj` |
| Hong Kong | `push2` · `116.` | `qt.gtimg.cn` · `hk` |
| US | `push2` · `105/106/107.` | `qt.gtimg.cn` · `us` |
| Indices | `1.000001`, `100.HSI` | `sh000001`, `hkHSI` |
| Intraday | `push2his/.../trends2` | `web.ifzq.gtimg.cn/.../minute` |
| Candles | `push2his/.../kline` | `web.ifzq.gtimg.cn/.../fqkline` |

- Quotes are fetched in batches of 50, never one request per symbol.
- When the primary returns only part of the watchlist, the fallback fills the gap instead of the partial result being treated as success.
- Repeated failures open a circuit breaker for 30 seconds, doubling up to 5 minutes.
- US venue ids learned from the fallback are written back, so the primary can locate those symbols next time.
- Both are public, unofficial endpoints and may change. The `provider.Provider` interface keeps a new source to three methods.

## Limitations

- No tick-by-tick or WebSocket feed; the UI polls snapshots.
- A-share holidays for 2025-2026 are built in, derived from the index actual trading days. Hong Kong and US only check weekday and session time, and HK half-day sessions are not handled.
- From 2027 onward you need to supply closed days through `holidays`; otherwise the app falls back to weekday logic and shows a `?` next to 交易中 to flag the uncovered year.
- Minute-level candles come from Eastmoney only; that view reports an error when the primary is down.
- All figures come straight from the endpoints and are for reference only, not investment advice.
- The interface text is currently Chinese; English labels are not implemented yet.

## Development

```bash
go build -o quotui .    # build
go test ./...           # unit tests
go test -race ./...     # race detector
go vet ./...
```

```
main.go                      CLI entry point and -once text output
internal/config              config load/save and quote backfill
internal/quote               symbol, quote, intraday and candle types
internal/provider            data source interface
internal/provider/eastmoney  Eastmoney client (primary)
internal/provider/tencent    Tencent client (fallback)
internal/provider/failover   multi-source scheduling: gap filling + breaker
internal/market              trading session detection
internal/ui                  Bubble Tea UI: list, detail, charts
```

## License

[MIT](LICENSE) © 2026 ArkURL
