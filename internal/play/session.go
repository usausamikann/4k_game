package play

import (
	"fmt"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/input"
	"github.com/usausamikann/4k_game/internal/judge"
)

// Session connects input transitions to judgement and ID-based render updates.
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
	// OnInput receives borrowed Tracker events once per Update; do not retain them.
	OnInput     func([]judge.Event)
	engine      *judge.Engine
	tracker     input.Tracker
	events      []judge.Event
	changes     []judge.Result
	hasJudgeNow bool
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
	s := &Session{
		Chart:    p,
		View:     view,
		Timeline: timeline,
		Offsets:  offsets,
		engine:   engine,
		Now:      -timeline.Preparation,
		events:   make([]judge.Event, 0, 16),
		States:   make([]judge.State, len(notes)),
	}
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

func (s *Session) Pressed() [chart.LaneCount]bool { return s.tracker.Pressed() }
func (s *Session) SyncRaw(raw []input.RawEvent)   { s.tracker.SyncRaw(raw) }

func (s *Session) prepareUpdate(wall time.Time, focused bool) (now, visual, judgement time.Duration, err error) {
	if s.Clock == nil {
		c, err := NewClock(wall, s.Timeline.Preparation)
		if err != nil {
			return 0, 0, 0, err
		}
		s.Clock = c
	}
	if err := s.Clock.SetPaused(wall, !focused); err != nil {
		return 0, 0, 0, err
	}
	now = s.Clock.At(wall)
	visual, judgement, err = s.Offsets.Times(now)
	return now, visual, judgement, err
}

func (s *Session) finishUpdate(now, visual, judgement time.Duration) error {
	if s.OnInput != nil {
		s.OnInput(s.events)
	}
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
	s.hasJudgeNow = true
	s.View.Update(visual)
	return nil
}

func (s *Session) UpdateAt(wall time.Time, focused bool, down [chart.LaneCount]bool) error {
	now, visual, judgement, err := s.prepareUpdate(wall, focused)
	if err != nil {
		return err
	}
	s.events = s.tracker.Sample(focused, down, judgement, s.events)
	return s.finishUpdate(now, visual, judgement)
}

func (s *Session) UpdateWithEdgesAt(wall time.Time, focused bool, samples [chart.LaneCount]input.KeySample) error {
	now, visual, judgement, err := s.prepareUpdate(wall, focused)
	if err != nil {
		return err
	}
	s.events = s.tracker.SampleEdges(focused, samples, judgement, s.events)
	return s.finishUpdate(now, visual, judgement)
}

func (s *Session) UpdateWithRawAt(wall time.Time, focused bool, raw []input.RawEvent) error {
	now, visual, judgement, err := s.prepareUpdate(wall, focused)
	if err != nil {
		return err
	}
	convert := func(at time.Time) (time.Duration, error) {
		_, j, err := s.Offsets.Times(s.Clock.At(at))
		return j, err
	}
	s.events, err = s.tracker.ConsumeRaw(focused, raw, convert, s.JudgeNow, judgement, s.hasJudgeNow, s.events)
	if err != nil {
		return err
	}
	return s.finishUpdate(now, visual, judgement)
}

// UpdateWithRawResyncAt releases held judge keys after a lost Raw Input batch.
func (s *Session) UpdateWithRawResyncAt(wall time.Time, focused bool, down [chart.LaneCount]bool) error {
	now, visual, judgement, err := s.prepareUpdate(wall, focused)
	if err != nil {
		return err
	}
	s.events = s.tracker.ResyncRaw(focused, down, judgement, s.events)
	return s.finishUpdate(now, visual, judgement)
}

// SyncRawState records the physical snapshot while the audio device is not ready.
func (s *Session) SyncRawState(down [chart.LaneCount]bool) {
	s.tracker.SyncRawState(down)
}

func (s *Session) Statistics() judge.Stats { return s.engine.Statistics() }
