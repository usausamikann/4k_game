package play

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/judge"
)

func makeSession(t *testing.T, notes []chart.ScheduledNote, o Offsets) *Session {
	t.Helper()
	s, err := NewSession(notes, 180, Scroll{Height: 480, JudgeY: 400, Speed: 240, NoteHeight: 12}, o)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func updateSession(t *testing.T, s *Session, at time.Time, focus bool, down [4]bool) {
	t.Helper()
	if err := s.UpdateAt(at, focus, down); err != nil {
		t.Fatal(err)
	}
}
func TestOffsetsHitsAndIDMapping(t *testing.T) {
	for _, o := range []Offsets{{}, {A: 20 * time.Millisecond}, {B: 30 * time.Millisecond}, {A: 20 * time.Millisecond, B: 30 * time.Millisecond}, {A: 20 * time.Millisecond, B: -20 * time.Millisecond}, {A: -20 * time.Millisecond, B: -30 * time.Millisecond}} {
		t.Run(o.A.String()+"/"+o.B.String(), func(t *testing.T) {
			notes := []chart.ScheduledNote{{ID: 90, Lane: 1, AbsoluteTick: 960, Time: time.Second}, {ID: 12, Lane: 0, AbsoluteTick: 960, Time: time.Second}}
			original := append([]chart.ScheduledNote(nil), notes...)
			s := makeSession(t, notes, o)
			start := time.Now()
			updateSession(t, s, start, true, [4]bool{})
			raw := time.Second + o.A + o.B
			updateSession(t, s, start.Add(s.Timeline.Preparation+raw), true, [4]bool{true})
			if s.JudgeNow != time.Second || s.VisualNow != time.Second+o.B || s.Now != raw {
				t.Fatalf("axes %v %v %v", s.Now, s.VisualNow, s.JudgeNow)
			}
			if s.States[0] != judge.Pending || s.States[1] != judge.Hit || s.Last.Note.ID != 12 || s.Last.Offset != 0 {
				t.Fatalf("ID mapping %+v %+v", s.States, s.Last)
			}
			if !reflect.DeepEqual(notes, original) || !reflect.DeepEqual(s.Chart.Notes, original) {
				t.Fatal("offset mutated notes")
			}
			updateSession(t, s, start.Add(s.Timeline.Preparation+raw+90*time.Millisecond), true, [4]bool{true})
			if s.States[0] != judge.Pending {
				t.Fatal("corrected +90ms expired inclusively")
			}
			updateSession(t, s, start.Add(s.Timeline.Preparation+raw+91*time.Millisecond), true, [4]bool{true})
			if s.States[0] != judge.Miss || s.Statistics().Miss != 1 {
				t.Fatal("automatic MISS correction")
			}
		})
	}
}
func TestVisualAndJudgementAxesIndependent(t *testing.T) {
	for _, o := range []Offsets{{A: 30 * time.Millisecond}, {B: 30 * time.Millisecond}, {A: 30 * time.Millisecond, B: -30 * time.Millisecond}, {A: -30 * time.Millisecond}} {
		visual, judge, err := o.Times(time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if visual != time.Second-o.A || judge != time.Second-o.A-o.B {
			t.Fatal("offset axes")
		}
		if y := (Scroll{JudgeY: 400, Speed: 240}).Y(time.Second, visual); math.Abs(y-(400-240*o.A.Seconds())) > 1e-9 {
			t.Fatal("visual offset sign")
		}
	}
}
func TestFocusReleaseResumeAndNoInputMiss(t *testing.T) {
	notes := []chart.ScheduledNote{{ID: 0, Lane: 0, Time: 0}, {ID: 1, Lane: 0, AbsoluteTick: 960, Time: time.Second}}
	s := makeSession(t, notes, Offsets{})
	start := time.Now()
	updateSession(t, s, start, true, [4]bool{})
	zero := start.Add(s.Timeline.Preparation)
	updateSession(t, s, zero, true, [4]bool{true})
	if s.Statistics().Combo != 1 {
		t.Fatal("first hit")
	}
	updateSession(t, s, zero, false, [4]bool{true})
	updateSession(t, s, zero.Add(time.Hour), false, [4]bool{true})
	if s.Now != 0 || s.Statistics().Miss != 0 || s.States[1] != judge.Pending {
		t.Fatal("paused real time caused MISS")
	}
	updateSession(t, s, zero.Add(time.Hour), true, [4]bool{true})
	if s.Statistics().EmptyPresses != 0 || s.Statistics().Combo != 1 {
		t.Fatal("held resume press")
	}
	updateSession(t, s, zero.Add(time.Hour+500*time.Millisecond), true, [4]bool{})
	updateSession(t, s, zero.Add(time.Hour+time.Second), true, [4]bool{true})
	if s.Statistics().Combo != 2 {
		t.Fatal("engine retained pressed state on loss")
	}
	idle := makeSession(t, notes, Offsets{})
	updateSession(t, idle, start, true, [4]bool{})
	updateSession(t, idle, start.Add(idle.Timeline.Preparation+1100*time.Millisecond), true, [4]bool{})
	if idle.Statistics().Miss != 2 {
		t.Fatal("no-input Update failed to advance")
	}
}
func TestEightBeatTimeline(t *testing.T) {
	for _, bpm := range []float64{120, 180, 240} {
		timeline, err := NewTimeline(bpm)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := chart.TickToTime(8*chart.TicksPerBeat, bpm)
		if timeline.Preparation != want {
			t.Fatal("8 beat preparation")
		}
		for i, at := range timeline.CountTimes {
			absolute, _ := chart.TickToTime(int64(8-i)*chart.TicksPerBeat, bpm)
			if at != -absolute {
				t.Fatal("count tick rounding")
			}
			phase, count := timeline.Phase(at)
			if phase != "COUNT" || count != i+1 {
				t.Fatal("count phase")
			}
		}
		if phase, _ := timeline.Phase(timeline.SilenceStart); phase != "SILENT" {
			t.Fatal("silent boundary")
		}
		if phase, _ := timeline.Phase(0); phase != "PLAY" {
			t.Fatal("play boundary")
		}
		beat, err := timeline.BeatTime(3)
		want, _ = chart.TickToTime(3*chart.TicksPerBeat, bpm)
		if err != nil || beat != want {
			t.Fatal("audio beat time")
		}
		if _, err := timeline.BeatTime(-1); err == nil {
			t.Fatal("negative beat")
		}
	}
	timeline, _ := NewTimeline(180)
	if timeline.Preparation != 2666666667 || timeline.SilenceStart != -1333333333 || timeline.CountTimes != [4]time.Duration{-2666666667, -2333333333, -2000000000, -1666666667} {
		t.Fatalf("BPM180 timeline %+v", timeline)
	}
	c, _ := NewClock(time.Now(), timeline.Preparation)
	start := c.epochs[0].wall
	c.SetPaused(start.Add(time.Second), true)
	c.SetPaused(start.Add(11*time.Second), false)
	if c.At(start.Add(10*time.Second+timeline.Preparation)) != 0 {
		t.Fatal("timeline pause drift")
	}
}
func TestSuddenAndOffsetValidation(t *testing.T) {
	for text, want := range map[string]Sudden{"0": 0, "0.0": 0, "0.1": 1, "20.0": 200, "30.0": 300, "40.0": 400} {
		s, err := ParseSudden(text)
		if err != nil || s != want || s.Height(400) != float64(want) {
			t.Fatalf("sudden %s: %v %v", text, s, err)
		}
		if s.Height(800) != 2*s.Height(400) {
			t.Fatal("cover is not ratio")
		}
	}
	for _, text := range []string{"-0.1", "-1", "40.1", "41", "NaN", "1.01", "", "1.x"} {
		if _, err := ParseSudden(text); err == nil {
			t.Fatalf("accepted sudden %q", text)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.MaxFloat64} {
		if _, err := OffsetMilliseconds(value); err == nil {
			t.Fatal("invalid ms")
		}
	}
	if offset, err := OffsetMilliseconds(-1.5); err != nil || offset != -1500*time.Microsecond {
		t.Fatal("fractional ms")
	}
	for _, o := range []Offsets{{A: time.Duration(math.MinInt64)}, {B: time.Duration(math.MinInt64)}} {
		if _, _, err := o.Times(time.Second); err == nil {
			t.Fatal("offset overflow")
		}
	}
	if _, err := NewTimeline(0); err == nil {
		t.Fatal("invalid bpm")
	}
}

func TestIdleUpdateReusesBuffers(t *testing.T) {
	s := makeSession(t, []chart.ScheduledNote{{ID: 1, Time: time.Second}}, Offsets{})
	start := time.Now()
	updateSession(t, s, start, true, [4]bool{})
	at := start.Add(time.Millisecond)
	if allocations := testing.AllocsPerRun(100, func() {
		if err := s.UpdateAt(at, true, [4]bool{}); err != nil {
			t.Fatal(err)
		}
	}); allocations != 0 {
		t.Fatalf("idle Update allocated %g times", allocations)
	}
}

func TestSessionChordsAndReleaseRepress(t *testing.T) {
	notes := []chart.ScheduledNote{
		{ID: 11, Lane: 1, AbsoluteTick: 960, Time: time.Second},
		{ID: 42, Lane: 0, AbsoluteTick: 960, Time: time.Second},
		{ID: 77, Lane: 0, AbsoluteTick: 1056, Time: 1100 * time.Millisecond},
	}
	s := makeSession(t, notes, Offsets{})
	start := time.Now()
	updateSession(t, s, start, true, [4]bool{})
	zero := start.Add(s.Timeline.Preparation)
	updateSession(t, s, zero.Add(time.Second), true, [4]bool{true, true})
	if s.Statistics().Combo != 2 || s.States[0] != judge.Hit || s.States[1] != judge.Hit {
		t.Fatal("session chord failed")
	}
	updateSession(t, s, zero.Add(1100*time.Millisecond), true, [4]bool{true, true})
	if s.States[2] != judge.Pending || s.Statistics().EmptyPresses != 0 {
		t.Fatal("held key judged/repeated")
	}
	updateSession(t, s, zero.Add(1100*time.Millisecond), true, [4]bool{})
	updateSession(t, s, zero.Add(1100*time.Millisecond), true, [4]bool{true})
	if s.States[2] != judge.Hit || s.Statistics().Combo != 3 {
		t.Fatal("release/repress failed")
	}
}

func TestLastHitSurvivesLaterMiss(t *testing.T) {
	s := makeSession(t, []chart.ScheduledNote{
		{ID: 0, Lane: 0, Time: 520 * time.Millisecond, AbsoluteTick: 500},
		{ID: 1, Lane: 1, Time: 500 * time.Millisecond, AbsoluteTick: 480},
	}, Offsets{A: 20 * time.Millisecond, B: 30 * time.Millisecond})
	start := time.Now()
	updateSession(t, s, start, true, [4]bool{})
	updateSession(t, s, start.Add(s.Timeline.Preparation+560*time.Millisecond), true, [4]bool{true})
	if !s.HasLastHit || s.LastHit.Offset != -10*time.Millisecond {
		t.Fatalf("not corrected last Hit: %+v", s.LastHit)
	}
	updateSession(t, s, start.Add(s.Timeline.Preparation+700*time.Millisecond), true, [4]bool{true})
	if s.Last.State != judge.Miss || s.LastHit.Note.ID != 0 || s.LastHit.Offset != -10*time.Millisecond {
		t.Fatal("MISS replaced last Hit")
	}
}
