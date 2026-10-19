package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sync"
	"testing"

	"github.com/usausamikann/4k_game/internal/judge"
)

func constantClip(value float32, frames int) *Clip {
	p := make([]float32, frames*2)
	for i := range p {
		p[i] = value
	}
	return &Clip{PCM: p, Peak: math.Abs(float64(value))}
}
func pressEvents(lanes ...int) []judge.Event {
	var events []judge.Event
	for _, lane := range lanes {
		events = append(events, judge.Event{Lane: lane, Kind: judge.Press})
	}
	return events
}
func readKey(t *testing.T, m *KeyStream, n int) []byte {
	t.Helper()
	p := make([]byte, n)
	if got, e := m.Read(p); got != n || e != nil {
		t.Fatal(got, e)
	}
	return p
}
func keyPosition(t *testing.T, m *KeyStream) int64 {
	t.Helper()
	p, e := m.Seek(0, io.SeekCurrent)
	if e != nil {
		t.Fatal(e)
	}
	return p
}

// This reference matches the old mixer's simultaneous-voice arithmetic, including
// its sample order, float64 summation, nonlinear gain and final F32 conversion.
func referenceChord(lanes [4]*Clip, mask, volume int) []byte {
	n := 0
	for lane, c := range lanes {
		if mask&(1<<lane) != 0 {
			n = max(n, len(c.PCM))
		}
	}
	pcm := make([]byte, n*4)
	for i := 0; i < n; i++ {
		sum := 0.0
		for lane, c := range lanes {
			if mask&(1<<lane) != 0 && i < len(c.PCM) {
				sum += float64(c.PCM[i])
			}
		}
		binary.LittleEndian.PutUint32(pcm[i*4:], math.Float32bits(float32(.7*math.Tanh(sum*.5/.7)*float64(volume)/100)))
	}
	return pcm
}
func TestKeyAllMasksAndSharedDecode(t *testing.T) {
	for _, mode := range []string{"off", "unified", "per-lane"} {
		s := DefaultSettings()
		s.KeySoundMode = mode
		s.KeySoundLanes = [4]string{"bong_001", "drop_002", "bong_001", "toggle_001"}
		sources := map[string]*Clip{"bong_001": {PCM: []float32{.1, -.2, .3, -.4}}, "drop_002": {PCM: []float32{.5, -.6}}, "toggle_001": {PCM: []float32{-.7, .8, .1, -.1, .2, -.3}}}
		calls := map[string]int{}
		m, err := NewKeyStream(s, func(id string) (*Clip, error) { calls[id]++; return sources[id], nil })
		if err != nil {
			t.Fatal(err)
		}
		var lanes [4]*Clip
		for lane := range lanes {
			if mode != "off" {
				lanes[lane] = sources[s.SoundForLane(lane)]
			}
		}
		for mask := 0; mask < 16; mask++ {
			m.Seek(0, io.SeekStart)
			var events []judge.Event
			for lane := 0; lane < 4; lane++ {
				if mask&(1<<lane) != 0 {
					events = append(events, judge.Event{Lane: lane, Kind: judge.Press})
				}
			}
			m.HandleEvents(events)
			var want []byte
			if mode != "off" {
				want = referenceChord(lanes, mask, s.KeySoundVolume)
			}
			if !bytes.Equal(m.chords[mask], want) {
				t.Fatal("precomposition mismatch", mode, mask)
			}
			got := readKey(t, m, len(want)+17)
			if !bytes.Equal(got[:len(want)], want) || !bytes.Equal(got[len(want):], make([]byte, 17)) {
				t.Fatal("play/padding mismatch", mode, mask)
			}
		}
		wantCalls := 3
		if mode == "off" {
			wantCalls = 0
		}
		if mode == "unified" {
			wantCalls = 1
		}
		if len(calls) != wantCalls {
			t.Fatal("unneeded decode", calls)
		}
		for _, n := range calls {
			if n != 1 {
				t.Fatal("duplicate decode")
			}
		}
	}
}
func TestKeySwitchAndNoPress(t *testing.T) {
	s := DefaultSettings()
	s.KeySoundMode = "per-lane"
	s.KeySoundLanes = [4]string{"bong_001", "drop_002", "bong_001", "drop_002"}
	m, err := NewKeyStream(s, func(id string) (*Clip, error) {
		if id == "bong_001" {
			return constantClip(.2, 10), nil
		}
		return constantClip(.8, 10), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	m.HandleEvents(pressEvents(0, 2))
	if !bytes.Equal(readKey(t, m, 8), m.chords[5][:8]) {
		t.Fatal("batch not combined")
	}
	m.HandleEvents([]judge.Event{{Lane: 0, Kind: judge.Release}})
	m.HandleEvents(nil)
	if !bytes.Equal(readKey(t, m, 8), m.chords[5][8:16]) {
		t.Fatal("release/empty changed playback")
	}
	position := keyPosition(t, m)
	m.HandleEvents(pressEvents(1))
	if keyPosition(t, m) != position {
		t.Fatal("Press reset stream position")
	}
	if !bytes.Equal(readKey(t, m, len(m.chords[2])), m.chords[2]) {
		t.Fatal("new chord did not cut/restart")
	}
	if !bytes.Equal(readKey(t, m, 24), make([]byte, 24)) {
		t.Fatal("old tail survived")
	}
	m.HandleEvents(pressEvents(-1, 4))
	if !bytes.Equal(readKey(t, m, 8), make([]byte, 8)) {
		t.Fatal("invalid lane played")
	}
	m.HandleEvents(pressEvents(1, 1))
	if !bytes.Equal(readKey(t, m, 8), m.chords[2][:8]) {
		t.Fatal("duplicate lane doubled")
	}
}
func TestKeySeekAndPartialReads(t *testing.T) {
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	m, err := NewKeyStream(s, func(string) (*Clip, error) { return &Clip{PCM: []float32{.1, -.2, .3, -.4, .5, -.6}}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if keyPosition(t, m) != 0 {
		t.Fatal("initial position")
	}
	m.HandleEvents(pressEvents(0))
	want := append(append([]byte{}, m.chords[1]...), make([]byte, 40)...)
	var got []byte
	for _, n := range []int{0, 1, 2, 4, 1, 9, 3, 8, 36} {
		before := keyPosition(t, m)
		p := readKey(t, m, n)
		got = append(got, p...)
		if keyPosition(t, m) != before+int64(n) {
			t.Fatal("incorrect byte position")
		}
	}
	if !bytes.Equal(got, want) {
		t.Fatal("partial Read or current query changed PCM")
	}
	for _, v := range []struct {
		o int64
		w int
	}{{1, io.SeekStart}, {-1, io.SeekStart}, {1, io.SeekCurrent}, {-1, io.SeekCurrent}, {0, io.SeekEnd}, {0, 99}} {
		before := keyPosition(t, m)
		pos, e := m.Seek(v.o, v.w)
		if !errors.Is(e, errors.ErrUnsupported) || pos != before || keyPosition(t, m) != before {
			t.Fatal("unsupported seek mutated position", v, pos, e)
		}
	}
	m.HandleEvents(pressEvents(0))
	readKey(t, m, 3)
	m.Seek(0, io.SeekStart)
	if keyPosition(t, m) != 0 || !bytes.Equal(readKey(t, m, 40), make([]byte, 40)) {
		t.Fatal("rewind retained audio")
	}
	// A Press halfway through a stereo frame finishes just that frame, then starts
	// the new chord at its first complete frame without corrupting F32/channel bytes.
	m.Seek(0, io.SeekStart)
	m.HandleEvents(pressEvents(0))
	first := readKey(t, m, 3)
	m.HandleEvents(pressEvents(0, 1))
	actual := append(first, readKey(t, m, 5+len(m.chords[3]))...)
	expected := append(append([]byte{}, m.chords[1][:8]...), m.chords[3]...)
	if !bytes.Equal(actual, expected) {
		t.Fatal("partial frame switch")
	}
	m.position = math.MaxInt64 - 2
	p := make([]byte, 3)
	n, e := m.Read(p)
	if n != 2 || e == nil || keyPosition(t, m) != math.MaxInt64 {
		t.Fatal("overflow", n, e)
	}
	if n, e := m.Read(nil); n != 0 || e != nil {
		t.Fatal("zero read at limit")
	}
}
func TestKeyVolumeFiniteAndInvalidPCM(t *testing.T) {
	for _, v := range []int{0, 50, 100} {
		s := DefaultSettings()
		s.KeySoundMode = "unified"
		s.KeySoundVolume = v
		calls := 0
		m, err := NewKeyStream(s, func(string) (*Clip, error) { calls++; return constantClip(math.MaxFloat32, 4), nil })
		if err != nil {
			t.Fatal(err)
		}
		for mask := 1; mask < 16; mask++ {
			for i := 0; i < len(m.chords[mask]); i += 4 {
				f := float64(math.Float32frombits(binary.LittleEndian.Uint32(m.chords[mask][i:])))
				if math.IsNaN(f) || math.IsInf(f, 0) || math.Abs(f) > .700001*float64(v)/100 {
					t.Fatal("nonfinite/headroom")
				}
			}
		}
		if v == 0 && calls != 0 {
			t.Fatal("muted source decoded")
		}
		before := s.MetronomeGain()
		s.KeySoundVolume = 100 - v
		if s.MetronomeGain() != before {
			t.Fatal("coupled gains")
		}
		s.MetronomeVolume = v
		if s.MetronomeGain() != .25*float64(v)/100 {
			t.Fatal("met volume")
		}
	}
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		s := DefaultSettings()
		s.KeySoundMode = "unified"
		if _, e := NewKeyStream(s, func(string) (*Clip, error) { return constantClip(value, 1), nil }); e == nil {
			t.Fatal("nonfinite accepted")
		}
	}
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	for _, clip := range []*Clip{nil, {}, {PCM: []float32{1}}} {
		if _, e := NewKeyStream(s, func(string) (*Clip, error) { return clip, nil }); e == nil {
			t.Fatal("invalid PCM accepted")
		}
	}
	if _, e := NewKeyStream(s, func(string) (*Clip, error) { return nil, io.ErrUnexpectedEOF }); !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Fatal("load error lost")
	}
}
func TestKeyNoRuntimeAllocations(t *testing.T) {
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	m, _ := NewKeyStream(s, func(string) (*Clip, error) { return constantClip(.2, 100), nil })
	events := pressEvents(0, 1, 2, 3)
	p := make([]byte, 103)
	if n := testing.AllocsPerRun(10000, func() { m.HandleEvents(events); m.Read(p) }); n != 0 {
		t.Fatal("runtime allocation", n)
	}
	total := 0
	for _, chord := range m.chords {
		total += len(chord)
	}
	if total != 15*100*8 {
		t.Fatal("unexpected retained PCM", total)
	}
}
func TestKeyConcurrentReader(t *testing.T) {
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	m, _ := NewKeyStream(s, func(string) (*Clip, error) { return constantClip(.5, 100), nil })
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var p [4093]byte
		for i := 0; i < 1000; i++ {
			m.Read(p[:])
		}
	}()
	events := pressEvents(0, 2)
	for i := 0; i < 1000; i++ {
		m.HandleEvents(events)
		m.Seek(0, io.SeekCurrent)
		if i%50 == 0 {
			m.Seek(0, io.SeekStart)
		}
	}
	wg.Wait()
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

func TestKeyRealAssetsMatchOldChordOutput(t *testing.T) {
	ids := KeySoundIDs()
	sources := map[string]*Clip{}
	for _, id := range ids {
		clip, e := LoadKeySound(id)
		if e != nil {
			t.Fatal(e)
		}
		sources[id] = clip
	}
	for shift := 0; shift < 2; shift++ {
		s := DefaultSettings()
		s.KeySoundMode = "per-lane"
		s.KeySoundVolume = 100
		var lanes [4]*Clip
		for lane := range lanes {
			s.KeySoundLanes[lane] = ids[(shift+lane)%len(ids)]
			lanes[lane] = sources[s.KeySoundLanes[lane]]
		}
		m, e := NewKeyStream(s, func(id string) (*Clip, error) { return sources[id], nil })
		if e != nil {
			t.Fatal(e)
		}
		total := 0
		for mask := 1; mask < 16; mask++ {
			want := referenceChord(lanes, mask, 100)
			if !bytes.Equal(m.chords[mask], want) {
				t.Fatal("real asset mismatch", shift, mask)
			}
			total += len(want)
		}
		t.Logf("layout %d: retained precomposed PCM %d bytes", shift, total)
	}
}
func TestKeySilenceAndVolumeIndependence(t *testing.T) {
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	loader := func(string) (*Clip, error) { return constantClip(0, 10), nil }
	m, e := NewKeyStream(s, loader)
	if e != nil {
		t.Fatal(e)
	}
	m.HandleEvents(pressEvents(0, 1, 2, 3))
	if !bytes.Equal(readKey(t, m, 100), make([]byte, 100)) {
		t.Fatal("silent source audible")
	}
	loader = func(string) (*Clip, error) { return constantClip(.5, 10), nil }
	a, _ := NewKeyStream(s, loader)
	s.MetronomeVolume = 100
	b, _ := NewKeyStream(s, loader)
	for mask := 0; mask < 16; mask++ {
		if !bytes.Equal(a.chords[mask], b.chords[mask]) {
			t.Fatal("metronome volume altered keysound")
		}
	}
}

type resettingKeyPlayer struct {
	fakeKeyPlayer
	stream *KeyStream
}

func (p *resettingKeyPlayer) Rewind() error {
	p.rewinds++
	_, e := p.stream.Seek(0, io.SeekStart)
	return e
}
func TestKeyPlaybackDropsPCM(t *testing.T) {
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	m, _ := NewKeyStream(s, func(string) (*Clip, error) { return constantClip(.5, 100), nil })
	player := &resettingKeyPlayer{stream: m}
	p := KeyPlayback{Player: player}
	p.SetActive(true)
	m.HandleEvents(pressEvents(0))
	readKey(t, m, 3)
	if e := p.SetActive(false); e != nil {
		t.Fatal(e)
	}
	p.SetActive(true)
	if !bytes.Equal(readKey(t, m, 100), make([]byte, 100)) {
		t.Fatal("stale focus tail")
	}
}

// A CPU microbenchmark, not an output-latency or device-load measurement.
func BenchmarkKeyStreamRead(b *testing.B) {
	s := DefaultSettings()
	s.KeySoundMode = "unified"
	m, _ := NewKeyStream(s, func(string) (*Clip, error) { return constantClip(.5, 480), nil })
	events := pressEvents(0, 1, 2, 3)
	p := make([]byte, 480*8)
	b.ReportAllocs()
	b.SetBytes(int64(len(p)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.HandleEvents(events)
		m.Read(p)
	}
}
