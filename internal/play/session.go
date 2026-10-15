package play

import (
	"fmt"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/input"
	"github.com/usausamikann/4k_game/internal/judge"
)

// Session connects sampled transitions to judgement and ID-based render updates.
// Audio uses Now, rendering VisualNow, and input/expiry use JudgeNow.
type Session struct {
	Chart                    *PlayChart
	View                     *View
	Clock                    *Clock
	Timeline                 Timeline
	Offsets                  Offsets
	States                   []judge.State
	Last                     judge.Result
	LastHit                  judge.Result
	HasLastHit               bool
	HasLast                  bool
	Now, VisualNow, JudgeNow time.Duration
	engine                   *judge.Engine
	tracker                  input.Tracker
	events                   []judge.Event
	changes                  []judge.Result
}

func NewSession(notes []chart.ScheduledNote, bpm float64, scroll Scroll, offsets Offsets) (*Session, error) {
	timeline, err := NewTimeline(bpm)
	if err != nil {
		return nil, err
	}
	p, err := NewChart(notes)
	if err != nil {
		return nil, err
	}
	view, err := NewView(p, scroll)
	if err != nil {
		return nil, err
	}
	engine, err := judge.New(notes, judge.DefaultWindows())
	if err != nil {
		return nil, err
	}
	s := &Session{Chart: p, View: view, Timeline: timeline, Offsets: offsets, engine: engine, Now: -timeline.Preparation, events: make([]judge.Event, 0, chart.LaneCount), States: make([]judge.State, len(notes))}
	for i := range s.States {
		s.States[i] = judge.Pending
	}
	s.VisualNow, s.JudgeNow, err = offsets.Times(s.Now)
	if err != nil {
		return nil, err
	}
	view.Update(s.VisualNow)
	return s, nil
}
func (s *Session) UpdateAt(wall time.Time, focused bool, down [chart.LaneCount]bool) error {
	if s.Clock == nil {
		c, err := NewClock(wall, s.Timeline.Preparation)
		if err != nil {
			return err
		}
		s.Clock = c
	}
	if err := s.Clock.SetPaused(wall, !focused); err != nil {
		return err
	}
	now := s.Clock.At(wall)
	visual, judgement, err := s.Offsets.Times(now)
	if err != nil {
		return err
	}
	s.events = s.tracker.Sample(focused, down, judgement, s.events)
	if err := s.engine.Process(s.events, judgement); err != nil {
		return err
	}
	s.changes = s.engine.DrainChanges(s.changes)
	for _, r := range s.changes {
		index, ok := s.Chart.IndexForID(r.Note.ID)
		if !ok {
			return fmt.Errorf("unknown resolved note ID %d", r.Note.ID)
		}
		s.States[index] = r.State
		s.Last = r
		s.HasLast = true
		if r.State == judge.Hit {
			s.LastHit = r
			s.HasLastHit = true
		}
	}
	s.Now, s.VisualNow, s.JudgeNow = now, visual, judgement
	s.View.Update(visual)
	return nil
}
func (s *Session) Statistics() judge.Stats { return s.engine.Statistics() }
