// Package judge evaluates timestamped input independently of rendering and devices.
package judge

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
)

type State string

const (
	Pending State = "Pending"
	Hit     State = "Hit"
	Miss    State = "Miss"
)

type Rank string

const (
	NoRank   Rank = ""
	PClean   Rank = "P-CLEAN"
	Clean    Rank = "CLEAN"
	Bad      Rank = "BAD"
	MissRank Rank = "MISS"
)

type Windows struct{ PClean, Clean, Bad time.Duration }

func DefaultWindows() Windows {
	return Windows{30 * time.Millisecond, 60 * time.Millisecond, 90 * time.Millisecond}
}

type EventKind uint8

const (
	Press EventKind = iota
	Release
)

type Event struct {
	Lane int
	Time time.Duration
	Kind EventKind
}

// InputTime and Offset are populated only for Hit. ResolvedAt is the input time
// for Hit or the first integer nanosecond at which a Pending note becomes Miss.
type Result struct {
	Note                          chart.ScheduledNote
	State                         State
	Rank                          Rank
	InputTime, Offset, ResolvedAt time.Duration
}
type Stats struct{ PClean, Clean, Bad, Miss, EmptyPresses, Combo, MaxCombo int }
type expiry struct {
	index int
	at    time.Duration
}

type Engine struct {
	windows    Windows
	results    []Result
	lanes      [chart.LaneCount][]int
	boundaries [chart.LaneCount][]time.Duration
	expiries   []expiry
	nextExpiry int
	pressed    [chart.LaneCount]bool
	stats      Stats
	empty      []Event
	started    bool
	now        time.Duration
	changes    []Result
}

// New accepts a complete timeline, including notes beyond a single pattern.
// It copies all definitions and fixes lane boundaries once, before play.
func New(notes []chart.ScheduledNote, windows Windows) (*Engine, error) {
	if windows.PClean <= 0 || windows.PClean >= windows.Clean || windows.Clean >= windows.Bad {
		return nil, fmt.Errorf("windows must satisfy 0 < P-CLEAN < CLEAN < BAD")
	}
	e := &Engine{windows: windows}
	definitions := slices.Clone(notes)
	sort.Slice(definitions, func(i, j int) bool {
		a, b := definitions[i], definitions[j]
		if a.Time != b.Time {
			return a.Time < b.Time
		}
		if a.Lane != b.Lane {
			return a.Lane < b.Lane
		}
		return a.ID < b.ID
	})
	ids := make(map[int64]bool)
	ticks := make(map[struct {
		lane int
		tick int64
	}]bool)
	for _, note := range definitions {
		if note.Lane < 0 || note.Lane >= chart.LaneCount {
			return nil, fmt.Errorf("note %d: invalid lane %d", note.ID, note.Lane)
		}
		if note.AbsoluteTick < 0 || note.Time < 0 || windows.Bad == time.Duration(math.MaxInt64) || note.Time > time.Duration(math.MaxInt64)-windows.Bad-1 {
			return nil, fmt.Errorf("note %d: negative coordinate or time too large for MISS deadline", note.ID)
		}
		if ids[note.ID] {
			return nil, fmt.Errorf("duplicate note ID %d", note.ID)
		}
		ids[note.ID] = true
		position := struct {
			lane int
			tick int64
		}{note.Lane, note.AbsoluteTick}
		if ticks[position] {
			return nil, fmt.Errorf("duplicate tick in lane %d: %d", note.Lane, note.AbsoluteTick)
		}
		ticks[position] = true
		lane := e.lanes[note.Lane]
		if len(lane) > 0 && e.results[lane[len(lane)-1]].Note.Time == note.Time {
			return nil, fmt.Errorf("duplicate time in lane %d: %v", note.Lane, note.Time)
		}
		i := len(e.results)
		e.results = append(e.results, Result{Note: note, State: Pending})
		e.lanes[note.Lane] = append(lane, i)
	}
	for lane, indices := range e.lanes {
		for i, index := range indices {
			at := e.results[index].Note.Time
			deadline := at + windows.Bad + 1 // +BAD itself is still hittable.
			if i+1 < len(indices) {
				next := e.results[indices[i+1]].Note.Time
				delta := next - at
				// Ceiling preserves the exact midpoint ownership for odd ns gaps.
				boundary := at + delta/2 + delta%2
				e.boundaries[lane] = append(e.boundaries[lane], boundary)
				if boundary < deadline {
					deadline = boundary
				}
			}
			e.expiries = append(e.expiries, expiry{index, deadline})
		}
	}
	sort.Slice(e.expiries, func(i, j int) bool {
		a, b := e.expiries[i], e.expiries[j]
		if a.at != b.at {
			return a.at < b.at
		}
		return a.index < b.index
	})
	return e, nil
}

// Process validates the entire batch before changing state, stable-sorts input
// by time, expires notes before each event, then expires through now. Equal-time
// events retain caller order. Late/future events and a backwards clock are errors.
// All times must use the same clock; no offsets or inferred timestamps are added.
func (e *Engine) Process(events []Event, now time.Duration) error {
	if e.started && now < e.now {
		return fmt.Errorf("clock moved backwards")
	}
	for i, event := range events {
		if event.Lane < 0 || event.Lane >= chart.LaneCount || (event.Kind != Press && event.Kind != Release) {
			return fmt.Errorf("event %d: invalid lane or kind", i)
		}
		if event.Time > now || (e.started && event.Time < e.now) {
			return fmt.Errorf("event %d: time outside current batch interval", i)
		}
	}
	ordered := events
	compare := func(a, b Event) int { return cmp.Compare(a.Time, b.Time) }
	if !slices.IsSortedFunc(events, compare) {
		ordered = slices.Clone(events)
		slices.SortStableFunc(ordered, compare)
	}
	for _, event := range ordered {
		e.advance(event.Time)
		if event.Kind == Release {
			e.pressed[event.Lane] = false
			continue
		}
		if e.pressed[event.Lane] {
			continue
		} // OS repeat is not another press transition.
		e.pressed[event.Lane] = true
		e.press(event)
	}
	e.advance(now)
	e.now = now
	e.started = true
	return nil
}

func (e *Engine) advance(now time.Duration) {
	for e.nextExpiry < len(e.expiries) && e.expiries[e.nextExpiry].at <= now {
		ex := e.expiries[e.nextExpiry]
		e.nextExpiry++
		r := &e.results[ex.index]
		if r.State != Pending {
			continue
		}
		r.State, r.Rank, r.ResolvedAt = Miss, MissRank, ex.at
		e.changes = append(e.changes, *r)
		e.stats.Miss++
		e.stats.Combo = 0
	}
}

func (e *Engine) press(event Event) {
	indices := e.lanes[event.Lane]
	owner := sort.Search(len(e.boundaries[event.Lane]), func(i int) bool { return e.boundaries[event.Lane][i] > event.Time })
	if len(indices) > 0 {
		r := &e.results[indices[owner]]
		// Check bounds before subtracting, so extreme negative input cannot overflow.
		if r.State == Pending && event.Time >= r.Note.Time-e.windows.Bad && event.Time <= r.Note.Time+e.windows.Bad {
			offset := event.Time - r.Note.Time
			absolute := offset
			if absolute < 0 {
				absolute = -absolute
			}
			r.State, r.InputTime, r.Offset, r.ResolvedAt = Hit, event.Time, offset, event.Time
			switch {
			case absolute <= e.windows.PClean:
				r.Rank = PClean
				e.stats.PClean++
			case absolute <= e.windows.Clean:
				r.Rank = Clean
				e.stats.Clean++
			default:
				r.Rank = Bad
				e.stats.Bad++
			}
			e.changes = append(e.changes, *r)
			e.stats.Combo++
			if e.stats.Combo > e.stats.MaxCombo {
				e.stats.MaxCombo = e.stats.Combo
			}
			return
		}
	}
	e.stats.EmptyPresses++
	e.empty = append(e.empty, event)
}

// Snapshots never expose the engine's mutable storage.
func (e *Engine) Results() []Result     { return slices.Clone(e.results) }
func (e *Engine) Statistics() Stats     { return e.stats }
func (e *Engine) EmptyPresses() []Event { return slices.Clone(e.empty) }

// DrainChanges copies only newly resolved notes into reusable caller storage.
// Each transition is delivered exactly once, in processing order.
func (e *Engine) DrainChanges(dst []Result) []Result {
	dst = append(dst[:0], e.changes...)
	e.changes = e.changes[:0]
	return dst
}
