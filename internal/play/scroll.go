package play

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"
)

// Y is the note center; Speed is pixels/second. A note is visible while its
// rectangle touches [0, Height], including the two exact edge-touch instants.
type Scroll struct{ Height, JudgeY, Speed, NoteHeight float64 }

func (s Scroll) Y(noteTime, now time.Duration) float64 {
	return s.JudgeY - s.Speed*((float64(noteTime)-float64(now))/float64(time.Second))
}

// View maintains [first,end). It stores only a chart reference and cursors;
// selection does not allocate or scan every note on each frame.
type View struct {
	chart         *PlayChart
	ahead, behind time.Duration
	first, end    int
	last          time.Duration
	initialized   bool
}

func NewView(p *PlayChart, s Scroll) (*View, error) {
	for _, v := range []float64{s.Height, s.JudgeY, s.Speed, s.NoteHeight} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("scroll values must be finite")
		}
	}
	if p == nil || s.Height <= 0 || s.Speed <= 0 || s.NoteHeight <= 0 || s.JudgeY < 0 || s.JudgeY > s.Height {
		return nil, fmt.Errorf("invalid chart or scroll geometry")
	}
	half := new(big.Rat).SetFloat64(s.NoteHeight)
	half.Quo(half, big.NewRat(2, 1))
	before := new(big.Rat).SetFloat64(s.JudgeY)
	before.Add(before, half)
	after := new(big.Rat).SetFloat64(s.Height)
	after.Sub(after, new(big.Rat).SetFloat64(s.JudgeY))
	after.Add(after, half)
	ahead, err := travelNanos(before, s.Speed)
	if err != nil {
		return nil, err
	}
	behind, err := travelNanos(after, s.Speed)
	if err != nil {
		return nil, err
	}
	v := &View{chart: p, ahead: ahead, behind: behind}
	for _, g := range p.Groups {
		if g.Time > time.Duration(math.MaxInt64)-v.behind {
			return nil, fmt.Errorf("visibility end overflows time.Duration")
		}
	}
	return v, nil
}

// Interval gives the inclusive first/last integer nanosecond for intersection
// with the viewport. Entry uses ceil(noteTime - travel), exit uses floor.
func (v *View) Interval(noteTime time.Duration) (time.Duration, time.Duration) {
	return noteTime - v.ahead, noteTime + v.behind
}

func (v *View) Reset()            { v.first, v.end = 0, 0; v.initialized = false }
func (v *View) Range() (int, int) { return v.first, v.end }

func (v *View) Update(now time.Duration) {
	groups := v.chart.Groups
	if !v.initialized || now < v.last {
		v.first = sort.Search(len(groups), func(i int) bool { return groups[i].Time+v.behind >= now })
		v.end = sort.Search(len(groups), func(i int) bool { return groups[i].Time-v.ahead > now })
		v.initialized = true
	} else {
		for v.first < len(groups) && groups[v.first].Time+v.behind < now {
			v.first++
		}
		if v.end < v.first {
			v.end = v.first
		}
		for v.end < len(groups) && groups[v.end].Time-v.ahead <= now {
			v.end++
		}
	}
	v.last = now
}

// Compute bounds once with exact float64 rationals. A float floor can otherwise
// drop an exact edge by 1ns (for example 410px / 200px/s).
func travelNanos(distance *big.Rat, speed float64) (time.Duration, error) {
	distance.Quo(distance, new(big.Rat).SetFloat64(speed))
	distance.Mul(distance, new(big.Rat).SetInt64(int64(time.Second)))
	whole := new(big.Int).Quo(distance.Num(), distance.Denom())
	if !whole.IsInt64() {
		return 0, fmt.Errorf("visibility duration out of range")
	}
	return time.Duration(whole.Int64()), nil
}
