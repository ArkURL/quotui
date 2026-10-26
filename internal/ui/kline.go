package ui

import (
	"math"
	"strings"

	"github.com/arkurl/quotui/internal/quote"
)

// klineWindow 计算能放几根 K 线、每根占几列、左侧留白多少。
// 右侧对齐，最新一根贴住右边缘。
func klineWindow(n, w int) (take, step, offset int) {
	step = 2
	if w/step < 12 {
		step = 1
	}
	take = w / step
	if take > n {
		take = n
	}
	offset = w - take*step
	return take, step, offset
}

// klineLines 画蜡烛图：实体用实心块，影线用竖线，涨绿跌红按主题走。
func klineLines(th themeStyles, candles []quote.Candle, w, h int) []string {
	if len(candles) < 2 || w < 6 || h < 3 {
		return blankLines(w, h)
	}

	take, step, offset := klineWindow(len(candles), w)
	candles = candles[len(candles)-take:]

	lo, hi := candles[0].Low, candles[0].High
	for _, c := range candles {
		lo = math.Min(lo, c.Low)
		hi = math.Max(hi, c.High)
	}
	if hi-lo < 1e-9 {
		hi += 0.01
		lo -= 0.01
	}
	pad := (hi - lo) * 0.06
	lo -= pad
	hi += pad

	yFor := func(v float64) int {
		y := int(math.Round((hi - v) / (hi - lo) * float64(h-1)))
		if y < 0 {
			y = 0
		}
		if y > h-1 {
			y = h - 1
		}
		return y
	}

	usedW := take * step
	grid := newGrid(h, usedW)
	rising := make([]bool, take)

	for i, c := range candles {
		x := i * step
		rising[i] = c.Close >= c.Open

		// 影线
		top, bottom := yFor(c.High), yFor(c.Low)
		for y := top; y <= bottom; y++ {
			grid[y][x] = 1
		}
		// 实体
		bTop, bBottom := yFor(c.Open), yFor(c.Close)
		if bTop > bBottom {
			bTop, bBottom = bBottom, bTop
		}
		for y := bTop; y <= bBottom; y++ {
			grid[y][x] = 2
		}
	}

	lines := make([]string, 0, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		sb.WriteString(strings.Repeat(" ", offset))
		for x := 0; x < usedW; x++ {
			if grid[y][x] == 0 {
				sb.WriteString(" ")
				continue
			}
			st := th.down
			if rising[x/step] {
				st = th.up
			}
			if grid[y][x] == 2 {
				sb.WriteString(st.Render("█"))
			} else {
				sb.WriteString(st.Render("│"))
			}
		}
		lines = append(lines, sb.String())
	}
	return lines
}

// klineAxis 画底部时间轴，与图表的左右边界对齐。
func klineAxis(th themeStyles, candles []quote.Candle, period string, w int) string {
	if len(candles) < 2 {
		return strings.Repeat(" ", w)
	}
	take, step, offset := klineWindow(len(candles), w)
	layout := "01-02"
	switch period {
	case quote.PeriodMonth:
		layout = "2006-01"
	case quote.Period60Min:
		layout = "01-02 15:04"
	}
	first := candles[len(candles)-take].At.Format(layout)
	last := candles[len(candles)-1].At.Format(layout)
	return strings.Repeat(" ", offset) + spread(th.dim.Render(first), th.dim.Render(last), take*step)
}
