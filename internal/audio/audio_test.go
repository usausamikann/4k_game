package audio

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/usausamikann/4k_game/internal/play"
)

func TestFiveSoundsDecodeResampleAndAlign(t *testing.T) {
	ids := SoundIDs()
	if len(ids) != 5 {
		t.Fatal("five candidates required")
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			clip, err := Load(id)
			if err != nil {
				t.Fatal(err)
			}
			if clip.SourceRate != 44100 || len(clip.PCM)%2 != 0 || len(clip.PCM) == 0 {
				t.Fatal("source/output format")
			}
			if clip.AttackFrame < 0 || clip.AttackFrame > SampleRate/1000 {
				t.Fatalf("attack alignment %d frames", clip.AttackFrame)
			}
			if strings.HasPrefix(id, "switch") && clip.TrimFrames < SampleRate/20 {
				t.Fatal("switch lead-in was not compensated")
			}
			if !strings.HasPrefix(id, "switch") && clip.TrimFrames != 0 {
				t.Fatal("removed a prompt attack")
			}
			peak := 0.0
			for _, sample := range clip.PCM {
				if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) {
					t.Fatal("invalid PCM")
				}
				peak = math.Max(peak, math.Abs(float64(sample)))
			}
			if math.Abs(peak-clip.Peak) > 1e-5 {
				t.Fatal("attack peak removed")
			}
			if id == "glass_002" {
				for i := 0; i < len(clip.PCM); i += 2 {
					if clip.PCM[i] != clip.PCM[i+1] {
						t.Fatal("mono not duplicated to stereo")
					}
				}
			}
			stream, err := NewStream(clip, 220)
			if err != nil {
				t.Fatal(err)
			}
			rendered := make([]byte, SampleRate*8*3)
			if _, err := io.ReadFull(stream, rendered); err != nil {
				t.Fatal(err)
			}
			waveBytes := len(clip.PCM) * 4
			for _, beat := range []int64{1, 2, 3, 8, 9, 10} {
				onset, err := stream.schedule.Frame(beat)
				if err != nil {
					t.Fatal(err)
				}
				start := int(onset) * 8
				if start+waveBytes <= len(rendered) && !bytes.Equal(rendered[:waveBytes], rendered[start:start+waveBytes]) {
					t.Fatalf("beat %d changed click volume/timbre", beat)
				}
			}
			for i := 0; i < len(rendered); i += 4 {
				v := math.Float32frombits(binary.LittleEndian.Uint32(rendered[i:]))
				if math.Abs(float64(v)) > .80001 {
					t.Fatalf("clipped sample %g", v)
				}
			}
			t.Logf("source=44100Hz output=%dHz original=%d frames trim=%.3fms significant attack=%.3fms", SampleRate, clip.OriginalFrames, float64(clip.TrimFrames)*1000/SampleRate, float64(clip.AttackFrame)*1000/SampleRate)
		})
	}
	ids[0] = "changed"
	if SoundIDs()[0] != "switch8" {
		t.Fatal("sound list aliases global storage")
	}
	if _, err := Load("missing"); err == nil {
		t.Fatal("unknown sound accepted")
	}
	if _, err := load(fstest.MapFS{}, DefaultSound); err == nil || !strings.Contains(err.Error(), "open metronome glass_002") {
		t.Fatalf("missing asset: %v", err)
	}
	if _, err := load(fstest.MapFS{"glass_002.ogg": &fstest.MapFile{Data: []byte("broken")}}, DefaultSound); err == nil || !strings.Contains(err.Error(), "decode metronome glass_002") {
		t.Fatalf("bad asset: %v", err)
	}
	if _, err := Decode(bytes.NewReader(nil)); err == nil {
		t.Fatal("invalid OGG accepted")
	}
}

func TestAbsolutePositionsNoRoundingDrift(t *testing.T) {
	for _, bpm := range []float64{120, 180, 220, 137.35, 123.4567} {
		s, err := NewSchedule(bpm)
		if err != nil {
			t.Fatal(err)
		}
		for _, beat := range []int64{0, 1, 3, 4, 7, 8, 9, 100, 1000, 10000, 1000000} {
			frame, err := s.Frame(beat)
			if err != nil {
				t.Fatal(err)
			}
			ideal := float64(beat) * 60 * SampleRate / bpm
			if math.Abs(float64(frame)-ideal) > .50001 {
				t.Fatalf("BPM%g beat%d drift %g samples", bpm, beat, float64(frame)-ideal)
			}
		}
	}
	half, _ := NewSchedule(1920000)
	for beat, want := range []int64{0, 2, 3, 5, 6} {
		got, _ := half.Frame(int64(beat))
		if got != want {
			t.Fatal("half-sample rounding")
		}
	}
	for _, bpm := range []float64{0, -1, math.NaN(), math.Inf(1), math.MaxFloat64} {
		if _, err := NewSchedule(bpm); err == nil {
			t.Fatalf("invalid BPM %g", bpm)
		}
	}
	s, _ := NewSchedule(120)
	if _, err := s.Frame(-1); err == nil {
		t.Fatal("negative beat")
	}
	if _, err := s.Frame(math.MaxInt64); err == nil {
		t.Fatal("overflowing sample position")
	}
}

func TestCountSilencePlayUniformAndMapping(t *testing.T) {
	clip := &Clip{PCM: []float32{.5, -.5}, Peak: .5}
	s, err := NewStream(clip, 220)
	if err != nil {
		t.Fatal(err)
	}
	last, _ := s.schedule.Frame(16)
	raw := make([]byte, (last+1)*8)
	io.ReadFull(s, raw)
	var onsets []int64
	for frame := int64(0); frame <= last; frame++ {
		left := math.Float32frombits(binary.LittleEndian.Uint32(raw[frame*8:]))
		right := math.Float32frombits(binary.LittleEndian.Uint32(raw[frame*8+4:]))
		if left != 0 {
			if left != .5 || right != -.5 {
				t.Fatal("unexpected accent/gain")
			}
			onsets = append(onsets, frame)
		}
	}
	var want []int64
	for beat := int64(0); beat <= 16; beat++ {
		if beat < 4 || beat >= 8 {
			at, _ := s.schedule.Frame(beat)
			want = append(want, at)
		}
	}
	if !reflect.DeepEqual(onsets, want) {
		t.Fatalf("onsets %v want %v", onsets, want)
	}
	timeline, _ := play.NewTimeline(220)
	for beat := int64(0); beat <= 8; beat++ {
		frame, _ := s.schedule.Frame(beat)
		elapsed := time.Duration(float64(frame) * float64(time.Second) / SampleRate)
		at := ChartTime(elapsed, timeline.Preparation)
		if beat < 4 && math.Abs(float64(at-timeline.CountTimes[beat])) > float64(time.Second)/SampleRate+1 {
			t.Fatal("COUNT/clock origin differs")
		}
		if beat == 8 && math.Abs(float64(at)) > float64(time.Second)/SampleRate/2+1 {
			t.Fatal("PLAY onset not at chart zero")
		}
	}
}

func TestChunkSizesOverlapAndSilenceAreSafe(t *testing.T) {
	clip := &Clip{PCM: make([]float32, SampleRate/2*2), Peak: 1.2}
	for i := range clip.PCM {
		clip.PCM[i] = 1.2
	}
	a, err := NewStream(clip, 600)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewStream(clip, 600)
	length := SampleRate * 8
	want := make([]byte, length)
	io.ReadFull(a, want)
	got := make([]byte, length)
	for i := 0; i < len(got); {
		end := min(len(got), i+7)
		if _, err := io.ReadFull(b, got[i:end]); err != nil {
			t.Fatal(err)
		}
		i = end
	}
	if !bytes.Equal(got, want) {
		t.Fatal("chunk size affected PCM")
	}
	silent, _ := a.schedule.Frame(4)
	playStart, _ := a.schedule.Frame(8)
	for frame := int64(0); frame < int64(length/8); frame++ {
		sample := math.Float32frombits(binary.LittleEndian.Uint32(got[frame*8:]))
		if sample > .80001 {
			t.Fatal("overlap clipping")
		}
		if frame >= silent && frame < playStart && sample != 0 {
			t.Fatal("COUNT tails leaked into SILENT")
		}
	}
	if a.frame != b.frame || a.pending != b.pending {
		t.Fatal("reader state differs")
	}
}

func TestBoundedReaderAndInvalidPreparation(t *testing.T) {
	for _, clip := range []*Clip{nil, {}, {PCM: []float32{1}, Peak: 1}} {
		if _, err := NewStream(clip, 180); err == nil {
			t.Fatal("invalid clip")
		}
	}
	clip := &Clip{PCM: []float32{.5, .5}, Peak: .5}
	s, _ := NewStream(clip, 120)
	buffer := make([]byte, 8)
	s.Read(buffer)
	if allocations := testing.AllocsPerRun(100, func() { s.Read(buffer) }); allocations != 0 {
		t.Fatalf("per-frame PCM allocations %g", allocations)
	}
	if n, err := s.Read(nil); n != 0 || err != nil {
		t.Fatal("empty read")
	}
	if _, err := NewStream(&Clip{PCM: make([]float32, SampleRate*2), Peak: 1}, 60*SampleRate); err == nil {
		t.Fatal("unbounded overlapping voices")
	}
}
