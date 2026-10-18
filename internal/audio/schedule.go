package audio

import (
	"fmt"
	"math"
	"math/big"
	"time"
)

// Schedule keeps the exact rational value of the supplied float64 BPM. Every
// index is computed independently; nearest-sample rounding uses ties upward.
// Frame zero is mapped to chart time -Preparation by Transport.
type Schedule struct{ numerator, denominator big.Int }

func NewSchedule(bpm float64) (*Schedule, error) {
	if bpm <= 0 || math.IsNaN(bpm) || math.IsInf(bpm, 0) {
		return nil, fmt.Errorf("audio BPM must be positive and finite")
	}
	ratio := new(big.Rat).SetInt64(60 * SampleRate)
	ratio.Quo(ratio, new(big.Rat).SetFloat64(bpm))
	s := &Schedule{}
	s.numerator.Set(ratio.Num())
	s.denominator.Set(ratio.Denom())
	if s.numerator.Cmp(&s.denominator) < 0 {
		return nil, fmt.Errorf("audio BPM requires more than one beat per sample")
	}
	return s, nil
}
func (s *Schedule) Frame(beat int64) (int64, error) {
	if beat < 0 {
		return 0, fmt.Errorf("negative beat index")
	}
	n := new(big.Int).Mul(big.NewInt(beat), &s.numerator)
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, &s.denominator, r)
	if r.Lsh(r, 1).Cmp(&s.denominator) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("beat sample index overflow")
	}
	return q.Int64(), nil
}
func (s *Schedule) minimumGap() (int64, error) {
	gap := new(big.Int).Quo(&s.numerator, &s.denominator)
	if !gap.IsInt64() {
		return 0, fmt.Errorf("audio beat interval overflows sample position")
	}
	return gap.Int64(), nil
}

// ChartTime maps a stream position to the same unadjusted clock as the chart.
// The eighth beat is quantized within half a sample of chart time zero.
func ChartTime(position, preparation time.Duration) time.Duration { return position - preparation }
