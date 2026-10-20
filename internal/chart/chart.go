// Package chart loads and schedules one-beat patterns without graphics dependencies.
package chart

import (
	"fmt"
	"math"
	"math/big"
	"slices"
	"sort"
	"time"
)

const (
	LaneCount          = 4
	TicksPerBeat int64 = 960
	TicksPerBar  int64 = 4 * TicksPerBeat
)

// Note uses an original tick relative to the beginning of its pattern.
// Load converts the external JSON position fraction into this integer tick.
type Note struct {
	Lane int
	Tick int64
}

type Pattern struct {
	Name string `json:"name"`
	// TimeScale is the speed multiplier minus one: 0 = normal, 1 = double density.
	TimeScale int64  `json:"time_scale"`
	Notes     []Note `json:"notes"`
}

// ScheduledNote is independent of the pattern definition and has no judgement state.
// Step 1 schedules a single pattern at absolute tick zero.
type ScheduledNote struct {
	ID           int64
	Lane         int
	AbsoluteTick int64
	Time         time.Duration
}

// TickToTime converts an absolute tick directly to nanoseconds. Rounding is to
// the nearest nanosecond, with exact halves rounded up. BPM is interpreted as
// its exact float64 value; rational arithmetic avoids intermediate overflow and
// preserves integer tick precision for future long charts.
func TickToTime(tick int64, bpm float64) (time.Duration, error) {
	if tick < 0 {
		return 0, fmt.Errorf("tick must be nonnegative: %d", tick)
	}
	if err := validateBPM(bpm); err != nil {
		return 0, err
	}
	nanoseconds := new(big.Rat).SetInt64(tick)
	nanoseconds.Mul(nanoseconds, new(big.Rat).SetInt64(int64(time.Minute)))
	denominator := new(big.Rat).SetFloat64(bpm)
	denominator.Mul(denominator, new(big.Rat).SetInt64(TicksPerBeat))
	nanoseconds.Quo(nanoseconds, denominator)
	whole, remainder := new(big.Int), new(big.Int)
	whole.QuoRem(nanoseconds.Num(), nanoseconds.Denom(), remainder)
	if remainder.Lsh(remainder, 1).Cmp(nanoseconds.Denom()) >= 0 {
		whole.Add(whole, big.NewInt(1))
	}
	if !whole.IsInt64() {
		return 0, fmt.Errorf("tick %d at BPM %g exceeds time.Duration range", tick, bpm)
	}
	return time.Duration(whole.Int64()), nil
}

func validateBPM(bpm float64) error {
	if bpm <= 0 || math.IsNaN(bpm) || math.IsInf(bpm, 0) {
		return fmt.Errorf("BPM must be positive and finite: %g", bpm)
	}
	return nil
}

// Validate also applies to patterns constructed directly in Go.
func Validate(pattern Pattern) error {
	_, err := validatedNotes(pattern)
	return err
}

func validatedNotes(pattern Pattern) ([]Note, error) {
	divisor, err := speedMultiplier(pattern.TimeScale)
	if err != nil {
		return nil, err
	}
	for i, note := range pattern.Notes {
		if note.Lane < 0 || note.Lane >= LaneCount {
			return nil, fmt.Errorf("note %d: lane must be 0..3: %d", i, note.Lane)
		}
		if note.Tick < 0 || note.Tick >= TicksPerBeat {
			return nil, fmt.Errorf("note %d: tick must be 0..%d: %d", i, TicksPerBeat-1, note.Tick)
		}
		if _, err := effectiveTick(note.Tick, divisor); err != nil {
			return nil, fmt.Errorf("note %d: %w", i, err)
		}
	}
	notes := slices.Clone(pattern.Notes)
	sort.Slice(notes, func(i, j int) bool {
		if notes[i].Tick != notes[j].Tick {
			return notes[i].Tick < notes[j].Tick
		}
		return notes[i].Lane < notes[j].Lane
	})
	for i := 1; i < len(notes); i++ {
		if notes[i] == notes[i-1] {
			return nil, fmt.Errorf("duplicate note: lane %d, tick %d", notes[i].Lane, notes[i].Tick)
		}
	}
	return notes, nil
}

// speedMultiplier keeps the supported scale policy separate from tick conversion.
func speedMultiplier(scale int64) (int64, error) {
	if scale < 0 || scale > 1 {
		return 0, fmt.Errorf("time_scale must be 0 (normal) or 1 (double density): %d", scale)
	}
	return scale + 1, nil
}

func effectiveTick(tick, divisor int64) (int64, error) {
	if tick%divisor != 0 {
		return 0, fmt.Errorf("tick %d must be divisible by time_scale + 1 (%d)", tick, divisor)
	}
	return tick / divisor, nil
}

// EffectiveLengthTicks returns the validated pattern's exclusive end tick when
// scheduled at tick zero, including for an empty pattern. Use this length, not
// the last note, as the next pattern's start offset in future concatenation.
func EffectiveLengthTicks(pattern Pattern) (int64, error) {
	if err := Validate(pattern); err != nil {
		return 0, err
	}
	divisor, err := speedMultiplier(pattern.TimeScale)
	if err != nil {
		return 0, err
	}
	return effectiveTick(TicksPerBeat, divisor)
}

// EndTime converts the exclusive end tick directly with TickToTime's rounding.
// It exists even when the pattern has no notes and does not end gameplay.
func EndTime(pattern Pattern, bpm float64) (time.Duration, error) {
	length, err := EffectiveLengthTicks(pattern)
	if err != nil {
		return 0, err
	}
	return TickToTime(length, bpm)
}

// Schedule assigns zero-based IDs in tick/lane order without modifying pattern.
func Schedule(pattern Pattern, bpm float64) ([]ScheduledNote, error) {
	if err := validateBPM(bpm); err != nil {
		return nil, err
	}
	notes, err := validatedNotes(pattern)
	if err != nil {
		return nil, err
	}
	divisor, err := speedMultiplier(pattern.TimeScale)
	if err != nil {
		return nil, err
	}
	length, err := effectiveTick(TicksPerBeat, divisor)
	if err != nil {
		return nil, err
	}
	if _, err := TickToTime(length, bpm); err != nil {
		return nil, fmt.Errorf("pattern end: %w", err)
	}
	result := make([]ScheduledNote, len(notes))
	for i, note := range notes {
		tick, err := effectiveTick(note.Tick, divisor)
		if err != nil {
			return nil, fmt.Errorf("note %d: %w", i, err)
		}
		at, err := TickToTime(tick, bpm)
		if err != nil {
			return nil, fmt.Errorf("note %d: %w", i, err)
		}
		result[i] = ScheduledNote{ID: int64(i), Lane: note.Lane, AbsoluteTick: tick, Time: at}
	}
	return result, nil
}
