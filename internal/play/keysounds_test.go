package play

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	sound "github.com/usausamikann/4k_game/internal/audio"
	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/judge"
)

// Uses the real Session and Tracker, so press sharing covers empty presses,
// COUNT/SILENT/PLAY, held/repeated samples, focus restoration and offsets.
func TestSessionKeysoundTransitions(t *testing.T) {
	for _, bpm := range []float64{180, 220} {
		s, err := NewSession(nil, bpm, Scroll{Height: 480, JudgeY: 400, Speed: 240, NoteHeight: 12}, Offsets{A: 10 * time.Millisecond, B: 20 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		settings := sound.DefaultSettings()
		settings.KeySoundMode = "unified"
		pcm := make([]float32, 48000)
		for i := range pcm {
			pcm[i] = .5
		}
		keyStream, err := sound.NewKeyStream(settings, func(string) (*sound.Clip, error) { return &sound.Clip{PCM: pcm, Peak: .5}, nil })
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		s.OnInput = func(events []judge.Event) {
			keyStream.HandleEvents(events)
			for _, e := range events {
				if e.Kind == judge.Press {
					count++
				}
			}
		}
		wall := time.Unix(100, 0)
		tick := func(focused bool, down [4]bool) {
			t.Helper()
			if err := s.UpdateAt(wall, focused, down); err != nil {
				t.Fatal(err)
			}
			wall = wall.Add(time.Millisecond)
		}
		tick(true, [4]bool{})
		tick(true, [4]bool{true, true, true, true})
		tick(true, [4]bool{true, true, true, true})
		tick(true, [4]bool{})
		if count != 4 {
			t.Fatal("COUNT/hold/release", count)
		}
		wall = wall.Add(s.Timeline.Preparation / 2)
		tick(true, [4]bool{true})
		tick(true, [4]bool{})
		if phase, _ := s.Timeline.Phase(s.Now); phase != "SILENT" {
			t.Fatal(phase)
		}
		if count != 5 {
			t.Fatal("SILENT")
		}
		tick(false, [4]bool{true})
		tick(true, [4]bool{true})
		tick(true, [4]bool{true})
		if count != 5 {
			t.Fatal("focus held key")
		}
		tick(true, [4]bool{})
		tick(true, [4]bool{true})
		tick(true, [4]bool{})
		if count != 6 {
			t.Fatal("focus fresh press")
		}
		wall = wall.Add(s.Timeline.Preparation)
		tick(true, [4]bool{})
		if phase, _ := s.Timeline.Phase(s.Now); phase != "PLAY" {
			t.Fatal(phase)
		}
		step := time.Duration(float64(time.Minute) / bpm / 4)
		for i := 0; i < 200; i++ {
			var down [4]bool
			down[i%4] = true
			tick(true, down)
			tick(true, [4]bool{})
			wall = wall.Add(step - 2*time.Millisecond)
		}
		if count != 206 || s.Statistics().EmptyPresses != 206 {
			t.Fatal("staircase event mismatch", count, s.Statistics())
		}
	}
}

func TestKeysoundsIndependentOfJudgement(t *testing.T) {
	s := makeSession(t, []chart.ScheduledNote{{ID: 1, Lane: 0, Time: time.Second}, {ID: 2, Lane: 1, Time: 2 * time.Second}, {ID: 3, Lane: 2, Time: 3 * time.Second}}, Offsets{})
	presses := 0
	s.OnInput = func(events []judge.Event) {
		for _, e := range events {
			if e.Kind == judge.Press {
				presses++
			}
		}
	}
	start := time.Now()
	updateSession(t, s, start, true, [4]bool{})
	updateSession(t, s, start.Add(s.Timeline.Preparation+time.Second), true, [4]bool{true})
	if presses != 1 || s.Last.State != judge.Hit {
		t.Fatal("hit press", s.Last)
	}
	updateSession(t, s, start.Add(s.Timeline.Preparation+1500*time.Millisecond), true, [4]bool{})
	updateSession(t, s, start.Add(s.Timeline.Preparation+2070*time.Millisecond), true, [4]bool{false, true})
	if presses != 2 || s.Last.Rank != judge.Bad {
		t.Fatal("bad press", s.Last)
	}
	updateSession(t, s, start.Add(s.Timeline.Preparation+4*time.Second), true, [4]bool{})
	if presses != 2 || s.Last.State != judge.Miss {
		t.Fatal("automatic miss emitted press", s.Last)
	}
}

// Exercise the real Tracker -> Session callback -> KeyStream path. Two presses
// in one Update must select a chord, while a later press replaces that chord.
func TestSessionChordBatchAndCut(t *testing.T) {
	s := makeSession(t, nil, Offsets{})
	settings := sound.DefaultSettings()
	settings.KeySoundMode = "unified"
	settings.KeySoundVolume = 100
	stream, err := sound.NewKeyStream(settings, func(string) (*sound.Clip, error) { return &sound.Clip{PCM: []float32{.25, .25, .5, .5, .75, .75}}, nil })
	if err != nil {
		t.Fatal(err)
	}
	s.OnInput = stream.HandleEvents
	start := time.Now()
	updateSession(t, s, start, true, [4]bool{})
	updateSession(t, s, start.Add(time.Millisecond), true, [4]bool{true, false, true})
	read := func(want float64) {
		t.Helper()
		var p [8]byte
		if n, e := stream.Read(p[:]); n != 8 || e != nil {
			t.Fatal(n, e)
		}
		got := float64(math.Float32frombits(binary.LittleEndian.Uint32(p[:4])))
		if math.Abs(got-want) > 1e-7 {
			t.Fatal("wrong chord/sample", got, want)
		}
	}
	read(.7 * math.Tanh(.5*.5/.7))
	// Held keys and releases do not restart the already-playing chord.
	updateSession(t, s, start.Add(2*time.Millisecond), true, [4]bool{true, false, true})
	updateSession(t, s, start.Add(3*time.Millisecond), true, [4]bool{})
	read(.7 * math.Tanh(1*.5/.7))
	updateSession(t, s, start.Add(4*time.Millisecond), true, [4]bool{false, true})
	read(.7 * math.Tanh(.25*.5/.7))
}
