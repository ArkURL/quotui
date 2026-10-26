package market

import (
	"strings"
	"sync"
	"time"
)

// aShareClosures 是 A 股全市场休市日；周末由星期判断覆盖，这里只列工作日。
//
// 数据来源：以上证指数（000001）的实际交易日反推，覆盖 2025-01-01 至 2026-12-31。
// 2026 年 11 月之后没有法定节假日，故年底前无休市日。
// 2027 年及以后请按交易所公告补充，或用配置项 holidays 追加，无需等发版。
var aShareClosures = map[string]bool{
	"2025-01-01": true,
	"2025-01-28": true,
	"2025-01-29": true,
	"2025-01-30": true,
	"2025-01-31": true,
	"2025-02-03": true,
	"2025-02-04": true,
	"2025-04-04": true,
	"2025-05-01": true,
	"2025-05-02": true,
	"2025-05-05": true,
	"2025-06-02": true,
	"2025-10-01": true,
	"2025-10-02": true,
	"2025-10-03": true,
	"2025-10-06": true,
	"2025-10-07": true,
	"2025-10-08": true,
	"2026-01-01": true,
	"2026-01-02": true,
	"2026-02-16": true,
	"2026-02-17": true,
	"2026-02-18": true,
	"2026-02-19": true,
	"2026-02-20": true,
	"2026-02-23": true,
	"2026-04-06": true,
	"2026-05-01": true,
	"2026-05-04": true,
	"2026-05-05": true,
	"2026-06-19": true,
	"2026-09-25": true,
	"2026-10-01": true,
	"2026-10-02": true,
	"2026-10-05": true,
	"2026-10-06": true,
	"2026-10-07": true,
}

// aShareClosureYears 标记休市表覆盖到的年份，用于提示「按星期推断」的情况。
var aShareClosureYears = map[int]bool{2025: true, 2026: true}

var extraClosures = struct {
	sync.RWMutex
	days  map[string]bool
	years map[int]bool
}{
	days:  map[string]bool{},
	years: map[int]bool{},
}

// AddClosures 追加自定义休市日（YYYY-MM-DD），启动时调用一次。
// 用户显式给出某年的日期后，该年不再被视为「未知年份」。
func AddClosures(dates []string) {
	extraClosures.Lock()
	defer extraClosures.Unlock()

	for _, d := range dates {
		d = strings.TrimSpace(d)
		parsed, err := time.Parse("2006-01-02", d)
		if err != nil {
			continue
		}
		extraClosures.days[d] = true
		extraClosures.years[parsed.Year()] = true
	}
}

func isClosure(t time.Time) bool {
	if aShareClosures[t.Format("2006-01-02")] {
		return true
	}
	extraClosures.RLock()
	defer extraClosures.RUnlock()
	return extraClosures.days[t.Format("2006-01-02")]
}

// CalendarKnown 报告该年份的 A 股休市表是否已收录。
func CalendarKnown(t time.Time) bool {
	year := t.Year()
	if aShareClosureYears[year] {
		return true
	}
	extraClosures.RLock()
	defer extraClosures.RUnlock()
	return extraClosures.years[year]
}
