package play

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type Offsets struct{ A, B time.Duration }

func (o Offsets) Times(now time.Duration) (visual, judgement time.Duration, err error) {
	visual, err = subTime(now, o.A)
	if err != nil {
		return
	}
	judgement, err = subTime(visual, o.B)
	return
}
func subTime(a, b time.Duration) (time.Duration, error) {
	if (b > 0 && a < time.Duration(math.MinInt64)+b) || (b < 0 && a > time.Duration(math.MaxInt64)+b) {
		return 0, fmt.Errorf("offset time overflows time.Duration")
	}
	return a - b, nil
}
func OffsetMilliseconds(value float64) (time.Duration, error) {
	ns := math.Round(value * float64(time.Millisecond))
	if math.IsNaN(ns) || math.IsInf(ns, 0) || ns >= float64(math.MaxInt64) || ns <= float64(math.MinInt64) {
		return 0, fmt.Errorf("offset must be finite and fit in time.Duration")
	}
	return time.Duration(ns), nil
}

// Sudden uses integer tenths: 0..400. Height is independent of speed and time.
type Sudden int

func ParseSudden(text string) (Sudden, error) {
	if strings.HasPrefix(text, "-") {
		return 0, fmt.Errorf("sudden must be nonnegative")
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("sudden must be 0.0..40.0 in steps of 0.1")
	}
	whole, err := strconv.Atoi(parts[0])
	if err != nil || whole < 0 || whole > 40 {
		return 0, fmt.Errorf("sudden must be 0.0..40.0")
	}
	tenth := 0
	if len(parts) == 2 {
		if len(parts[1]) != 1 || parts[1][0] < '0' || parts[1][0] > '9' {
			return 0, fmt.Errorf("sudden must use at most one decimal digit")
		}
		tenth = int(parts[1][0] - '0')
	}
	value := Sudden(whole*10 + tenth)
	if value > 400 {
		return 0, fmt.Errorf("sudden exceeds 40.0")
	}
	return value, nil
}
func (s Sudden) Height(judgeY float64) float64 { return judgeY * float64(s) / 400 }
