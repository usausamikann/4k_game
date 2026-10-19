package audio

import (
	"encoding/binary"
	"io"
	"math"
	"sync"
	"testing"
)

func constantClip(value float32, frames int) *Clip {
	p := make([]float32, frames*2)
	for i := range p {
		p[i] = value
	}
	return &Clip{PCM: p, Peak: float64(value)}
}
func sampleKey(t *testing.T, m *KeyMixer) float64 {
	t.Helper()
	var p [8]byte
	if n, e := m.Read(p[:]); n != 8 || e != nil {
		t.Fatal(n, e)
	}
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(p[:4])))
}
func activeKeys(m *KeyMixer) int {
	n := 0
	for _, v := range m.voices {
		if v.clip != nil {
			n++
		}
	}
	return n
}
func TestKeyModesAndSharedPCM(t *testing.T) {
	for _, mode := range []string{"off", "unified", "per-lane"} {
		t.Run(mode, func(t *testing.T) {
			s := DefaultSettings()
			s.KeySoundMode = mode
			s.KeySoundLanes = [4]string{"bong_001", "drop_002", "bong_001", "toggle_001"}
			calls := map[string]int{}
			clips := map[string]*Clip{}
			m, err := NewKeyMixer(s, func(id string) (*Clip, error) { calls[id]++; c := constantClip(.5, 100); clips[id] = c; return c, nil })
			if err != nil {
				t.Fatal(err)
			}
			for lane := 0; lane < 4; lane++ {
				id := s.SoundForLane(lane)
				if id == "" {
					if m.lanes[lane] != nil {
						t.Fatal("off assigned PCM")
					}
				} else if m.lanes[lane] != clips[id] {
					t.Fatal("wrong lane PCM", lane)
				}
				m.Press(lane)
			}
			if mode == "off" {
				if len(calls) != 0 || sampleKey(t, m) != 0 {
					t.Fatal("off decoded or played")
				}
			} else {
				if activeKeys(m) != 4 || sampleKey(t, m) <= 0 {
					t.Fatal("simultaneous presses missing")
				}
				for id, n := range calls {
					if n != 1 {
						t.Fatal("duplicate decode", id, n)
					}
				}
			}
		})
	}
}
func TestKeyVolumeAndHeadroom(t *testing.T) {
	for _, volume := range []int{0, 50, 100} {
		s := DefaultSettings()
		s.KeySoundMode = "unified"
		s.KeySoundVolume = volume
		m, _ := NewKeyMixer(s, func(string) (*Clip, error) { return constantClip(1, 100), nil })
		m.Press(0)
		got := sampleKey(t, m)
		want := .7 * math.Tanh(.5/.7) * float64(volume) / 100
		if math.Abs(got-want) > 1e-7 {
			t.Fatal(volume, got, want)
		}
		before := s.MetronomeGain()
		s.KeySoundVolume = 100 - volume
		if before != s.MetronomeGain() {
			t.Fatal("coupled gains")
		}
		s.MetronomeVolume = volume
		if s.MetronomeGain() != .25*float64(volume)/100 {
			t.Fatal("metronome percentage")
		}
	}
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	s.KeySoundVolume = 100
	m, _ := NewKeyMixer(s, func(string) (*Clip, error) { return constantClip(1, 100), nil })
	for i := 0; i < KeyVoiceLimit; i++ {
		m.Press(i % 4)
	}
	if got := sampleKey(t, m); got > .700001 || got+.25 >= 1 {
		t.Fatal("headroom", got)
	}
}
func TestKeyBoundReuseAndReset(t *testing.T) {
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	m, _ := NewKeyMixer(s, func(string) (*Clip, error) { return constantClip(.5, 100), nil })
	for i := 0; i < KeyVoiceLimit; i++ {
		m.Press(i % 4)
	}
	m.Press(2)
	if activeKeys(m) != KeyVoiceLimit || m.voices[0].serial != KeyVoiceLimit+1 {
		t.Fatal("oldest not reused")
	}
	for i := 0; i < 10000; i++ {
		m.Press(i % 4)
	}
	if activeKeys(m) != KeyVoiceLimit {
		t.Fatal("unbounded voices")
	}
	if alloc := testing.AllocsPerRun(100, func() { m.Press(0) }); alloc != 0 {
		t.Fatal("per-press allocation", alloc)
	}
	var partial [3]byte
	m.Read(partial[:])
	if _, err := m.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if activeKeys(m) != 0 || sampleKey(t, m) != 0 {
		t.Fatal("stale tail")
	}
	if _, err := m.Seek(1, io.SeekStart); err == nil {
		t.Fatal("invalid seek")
	}
	m.Press(0)
	for i := 0; i < 100; i++ {
		sampleKey(t, m)
	}
	if activeKeys(m) != 0 {
		t.Fatal("finished voice retained")
	}
}
func TestKeySettingsValidation(t *testing.T) {
	good := "bong_001,drop_002,glass_005,toggle_001"
	for _, mode := range []string{"off", "unified", "per-lane"} {
		for _, v := range []int{0, 50, 100} {
			if _, err := ParseSettings(mode, "drop_002", good, v, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, args := range []struct {
		mode, sound, lanes string
		k, m               int
	}{
		{"oops", "drop_002", good, 50, 50}, {"off", "glass_002", good, 50, 50}, {"off", "drop_002", "bong_001", 50, 50}, {"off", "drop_002", "bad,drop_002,drop_002,drop_002", 50, 50}, {"off", "drop_002", good, -1, 50}, {"off", "drop_002", good, 101, 50}, {"off", "drop_002", good, 50, -1}, {"off", "drop_002", good, 50, 101},
	} {
		if _, err := ParseSettings(args.mode, args.sound, args.lanes, args.k, args.m); err == nil {
			t.Fatal("invalid accepted", args)
		}
	}
}
func TestProvidedKeyAssets(t *testing.T) {
	for _, id := range KeySoundIDs() {
		c, err := LoadKeySound(id)
		if err != nil {
			t.Fatal(err)
		}
		if c.TrimFrames != 0 || len(c.PCM) == 0 || c.SourceRate <= 0 {
			t.Fatal("processed attack altered", id)
		}
	}
	if _, err := LoadKeySound(DefaultSound); err == nil {
		t.Fatal("metronome list leaked")
	}
	if _, err := Load("drop_002"); err == nil {
		t.Fatal("keysound list leaked")
	}
}
func TestKeyConcurrentReader(t *testing.T) {
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	m, _ := NewKeyMixer(s, func(string) (*Clip, error) { return constantClip(.5, 100), nil })
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var p [103]byte
		for i := 0; i < 1000; i++ {
			m.Read(p[:])
		}
	}()
	for i := 0; i < 1000; i++ {
		m.Press(i % 4)
		if i%50 == 0 {
			m.Seek(0, io.SeekStart)
		}
	}
	wg.Wait()
}

type fakeKeyPlayer struct{ plays, pauses, rewinds, closed int }

func (p *fakeKeyPlayer) Play()                { p.plays++ }
func (p *fakeKeyPlayer) PauseAndStopReading() { p.pauses++ }
func (p *fakeKeyPlayer) Rewind() error        { p.rewinds++; return nil }
func (p *fakeKeyPlayer) Close() error         { p.closed++; return nil }
func TestKeyPlaybackFocus(t *testing.T) {
	f := &fakeKeyPlayer{}
	p := &KeyPlayback{Player: f}
	for _, a := range []bool{false, true, true, false, false, true} {
		if err := p.SetActive(a); err != nil {
			t.Fatal(err)
		}
	}
	if f.plays != 2 || f.pauses != 1 || f.rewinds != 1 {
		t.Fatal(f)
	}
	p.Close()
	if f.closed != 1 {
		t.Fatal("not closed")
	}
}
