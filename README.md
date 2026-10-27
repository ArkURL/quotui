简体中文 | [English](README.en.md)

# quotui

[![CI](https://github.com/ArkURL/quotui/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ArkURL/quotui/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ArkURL/quotui?color=3FBF6F)](https://github.com/ArkURL/quotui/releases)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-6e6e82)](#安装)
[![Bubble Tea](https://img.shields.io/badge/Made%20with-Bubble%20Tea-ff69b4)](https://github.com/charmbracelet/bubbletea)
[![License](https://img.shields.io/badge/License-MIT-3FBF6F)](LICENSE)

> 终端看盘工具：一个二进制、零依赖，盯着自选股的现价。

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

## 特性

- **多市场**：A 股（沪/深/北）、港股、美股，指数同样支持。
- **自选股**：搜索添加（代码/名称/拼音）、删除、拖拽式排序、分组排序，全部本地持久化。
- **详情页**：当日分时图与 K 线（日/周/月/60 分），`c` / `p` 一键切换。
- **省流量**：批量请求、缺口补齐、失败熔断、休市降频，另有「仅手动」模式。
- **双源兜底**：东方财富为主、腾讯备用，任一不可用自动切换并在状态栏提示。
- **窄屏友好**：列自动裁剪、CJK 宽度对齐，40 列也能看。
- **可脚本化**：`-once` 输出纯文本快照，可直接进 tmux 状态栏。

## 安装

不想装 Go 的话，直接从 [Releases](https://github.com/ArkURL/quotui/releases) 下载对应平台的压缩包，解压即用。

```bash
# 需要 Go 1.27 或更高版本
go install github.com/arkurl/quotui@latest

# 或者从源码构建
git clone git@github.com:ArkURL/quotui.git
cd quotui
go build -o quotui .
```

产物是单个静态二进制，无需运行时依赖。

> 机器上存在多个副本时，`which -a quotui` 里排第一的才是实际执行的；按 `?` 打开帮助，底部会显示当前版本号，可用来确认没跑错二进制。旧版本保存配置时会丢掉它不认识的新字段。

## 快速开始

```bash
quotui                  # 打开 TUI
quotui -once            # 抓一次行情，输出纯文本后退出
quotui -json            # 抓一次行情，输出 JSON（隐含 -once）
quotui -config ./c.toml # 指定配置文件
quotui -version         # 打印版本号
```

首次启动会在系统配置目录生成默认配置并预置 6 只自选股：

- macOS：`~/Library/Application Support/quotui/config.toml`
- Linux：`~/.config/quotui/config.toml`
- Windows：`%AppData%\quotui\config.toml`

### 脚本化

`-json` 输出结构化快照，可以直接交给 jq：

```bash
# 涨幅超过 5% 的标的
quotui -json | jq -r '.quotes[] | select(.change_pct > 5) | "\(.name) \(.change_pct)%"'
```

每支标的的字段：`code` `market` `name` `price` `prev_close` `open` `high` `low` `change` `change_pct` `volume` `amount` `turnover` `pe` `pb` `market_cap` `suspended` `decimals`。
停牌或无价时 `price` 为 `null` 且 `suspended` 为 `true`，不会和「跌到 0」混淆。

顶层还有 `updated`（RFC3339）、`source`（当前生效的数据源）、`count`，以及取数失败时的 `missing`（如 `hk/00700`）。
成交量单位随市场：A 股为手，港美股为股。

## 快捷键

| 按键 | 作用 |
|---|---|
| `↑` / `k`、`↓` / `j` | 上下移动 |
| `g` / `G` | 跳到首行 / 末行 |
| `Enter` | 打开详情页；再按一次返回 |
| `c` | 详情页内切换 分时 / K 线 |
| `p` | K 线内切换周期（日/周/月/60 分） |
| `Esc` | 返回列表 |
| `r` | 立即刷新（1 秒冷却） |
| `t` | 选择刷新周期（含「仅手动」） |
| `空格` | 暂停 / 继续刷新 |
| `s` / `S` | 切换排序字段 / 升降序 |
| `a` / `d` | 搜索添加 / 删除自选 |
| `J` / `K` | 调整自定义顺序 |
| `?` | 帮助 |
| `q` | 退出 |

## 配置

```toml
refresh_seconds = 3       # 3 秒自动刷新；0 表示只用 r 手动刷新
color_scheme = "red_up"   # red_up（A 股习惯，红涨绿跌）或 green_up
up_color = ""             # 自定义上涨色，如 "#B08968"；留空跟随 color_scheme
down_color = ""           # 自定义下跌色，如 "#6D8A96"；留空跟随 color_scheme
sort_by = "order"         # order / pct / amount / code
sort_desc = true
columns = ["code", "name", "price", "chg", "pct", "amount", "turnover"]
holidays = []             # 额外休市日，如 ["2027-01-01"]，用于补充内置日历

[[watchlist]]
  code = "600519"
  market = "sh"           # sh / sz / bj / hk / us
  name = "贵州茅台"
```

- `columns` 决定展示哪些列及顺序，可选 `code` / `name` / `price` / `chg` / `pct` / `amount` / `turnover`（`chg` 为涨跌额，`pct` 为涨跌幅）。终端变窄时按「换手 → 成交额 → 涨跌额 → 现价 → 代码」依次裁列，名称和涨幅始终保留。
- `secid` 字段可选，用于固定港美股的市场编号（如 `105.AAPL`）。通过 `a` 键搜索添加时自动写入；手工填写时若留空，会在首次取到行情后回填。
- 排序字段、升降序、光标位置都会写回配置，重启后保持原样。
- 按 `C` 打开配色面板，内置 4 套预设，选中即应用并写入配置：

  | 预设 | 上涨 | 下跌 |
  |---|---|---|
  | A股 红涨绿跌 | `#F2575B` | `#3FBF6F` |
  | 港美股 绿涨红跌 | `#3FBF6F` | `#F2575B` |
  | 低调 暖橙灰蓝 | `#B08968` | `#6D8A96` |
  | 色弱 橙蓝 | `#E69F00` | `#56B4E9` |

- 前两套用 `color_scheme` 表达（会清空自定义色），后两套写死 `up_color` / `down_color`。面板里也支持逐项输入自定义色，支持 `#RGB` 与 `#RRGGBB`，非法值忽略并回落到 `color_scheme`。
- 「恢复默认」= 清空自定义色并把 `color_scheme` 切回 `red_up`。当前生效的预设会标「当前」，手动改过颜色之后不再命中任何预设。
- `holidays` 用于补充内置休市表未覆盖的年份（详见[已知限制](#已知限制)）。

## 详情页

在列表上按 `Enter` 进入，`Esc` 返回；`j` / `k` 直接切到上一只 / 下一只，不用退出再选。

### 分时

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

- 竖条是该列采样点的价格区间，末尾 `•` 是现价，青色 `·` 是均价，虚线是昨收。
- 纵轴贴合当日振幅，横轴按采样点拉伸，两端显示真实时间（美股为北京时间）。

### K 线

详情页按 `c` 切到 K 线，`p` 依次切换 日 / 周 / 月 / 60 分。

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
                                   █ █               █ █     █       █ │     │ │
                                 █ █ █   │           █           █ █ █ █
                     │         █       █ │     │   │ │         █ █ █ │
08-12                                                                          10-26
```

- 实体用实心块，影线用竖线；默认红涨绿跌，`green_up` 主题下互换。
- 每根 K 线占 2 列（实体 + 间隔），宽度不足时降为 1 列；右对齐，最新一根贴住右边缘。
- 日线用前复权数据；周/月线按周期回溯足够长的历史，再截取最近 120 根。

> 嫌红绿太跳？按 `C` 选「低调 暖橙灰蓝」，涨跌仍可分但不再刺眼；红绿色盲可以用「色弱 橙蓝」。

## 刷新策略

不想被频繁轮询打扰时，按 `t` 选「仅手动」，此后只在按 `r` 时请求。

- 可选周期：仅手动 / 1 / 2 / 3 / 5 / 10 / 30 / 60 秒，选择后写入配置文件。
- 手动刷新有 1 秒冷却，连按 `r` 会被拦下并给出提示。
- 上一次请求未返回时不会再发新请求，弱网下不会堆积。
- 全部市场休市时，自动周期最多降到 30 秒。
- 连续两次请求失败后退避到 30 秒，恢复后回到你选的周期。
- 详情页分时最多 10 秒刷新一次，K 线 30 秒，与列表请求互不叠加。

## 行情来源

东方财富为主源，腾讯为备用源；任一失败（超时、限流、返回空）自动切换，状态栏显示当前生效的数据源与熔断状态。

| 市场 | 东方财富 | 腾讯 |
|---|---|---|
| A 股 | `push2` · 沪 `1.`、深/北 `0.` | `qt.gtimg.cn` · `sh`/`sz`/`bj` |
| 港股 | `push2` · `116.` | `qt.gtimg.cn` · `hk` |
| 美股 | `push2` · `105/106/107.` | `qt.gtimg.cn` · `us` |
| 指数 | `1.000001`、`100.HSI` | `sh000001`、`hkHSI` |
| 分时 | `push2his/.../trends2` | `web.ifzq.gtimg.cn/.../minute` |
| K 线 | `push2his/.../kline` | `web.ifzq.gtimg.cn/.../fqkline` |

- 一次请求批量拉全部自选（50 只一批），不会逐支轮询。
- 主源只返回部分标的时，用备用源补齐缺口，不拿半成品当成功。
- 连续失败会熔断 30 秒（逐次翻倍，最长 5 分钟），期间直接跳过。
- 从备用源学到的美股交易所编号会写回配置，之后主源也能正确定位。
- 两个都是公开的非官方接口，可能随时限流或变更；`provider.Provider` 抽象下新增数据源只需实现三个方法。

## 已知限制

- 不支持逐笔 / WebSocket 推送，按秒级快照刷新。
- A 股内置 2025–2026 休市表，由上证指数的实际交易日反推得到；港股、美股仍只看星期与时段，港股半日市未单独处理。
- 2027 年起需自行补充休市日（`holidays` 配置项），否则按星期推断，状态栏会显示「A股 交易中?」提示该年日历未收录。
- 分钟级 K 线只有东方财富提供，主源不可用时该视图会提示失败。
- 数据以接口返回为准，仅供参考，不构成投资建议。

## 开发

```bash
go build -o quotui .    # 构建
go test ./...           # 单元测试
go test -race ./...     # 竞态检测
go vet ./...
```

```
main.go                    命令行入口、-once 文本输出
internal/config            配置读写与行情补全
internal/quote             标的、行情、分时、K 线的数据结构与格式化
internal/provider          数据源接口
internal/provider/eastmoney 东财实现（主源）
internal/provider/tencent   腾讯实现（备用源）
internal/provider/failover  多源调度：缺口补齐 + 熔断
internal/market            交易时段判断
internal/ui                Bubble Tea 界面：列表、详情、分时图、K 线
```

## 许可证

[MIT](LICENSE) © 2026 ArkURL
