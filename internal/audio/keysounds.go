package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"

	audioassets "github.com/usausamikann/4k_game/assets/audio"
	"github.com/usausamikann/4k_game/internal/judge"
)

// KeyStream plays at most one precomposed chord. PCM is immutable stereo F32 LE.
// Construction is the only synthesis step; rebuilding this stream prepares a new
// fixed lane assignment. Sources are shared while building, then released.
type KeyStream struct {
	mu       sync.Mutex
	chords   [16][]byte
	current  []byte
	cursor   int
	position int64 // bytes returned by Read since rewind, independent of Press
	scratch  [8]byte
	pending  int
}

func NewKeyStream(s Settings, load func(string) (*Clip, error)) (*KeyStream, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	m := &KeyStream{}
	if s.KeySoundMode == "off" || s.KeySoundVolume == 0 {
		return m, nil
	}
	var lanes [4]*Clip
	cache := map[string]*Clip{}
	for lane := range lanes {
		id := s.SoundForLane(lane)
		clip := cache[id]
		if clip == nil {
			var err error
			clip, err = load(id)
			if err != nil {
				return nil, err
			}
			if clip == nil || len(clip.PCM) == 0 || len(clip.PCM)%2 != 0 || len(clip.PCM) > maxClipSeconds*SampleRate*2 {
				return nil, fmt.Errorf("invalid keysound PCM %s", id)
			}
			for _, sample := range clip.PCM {
				if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) {
					return nil, fmt.Errorf("nonfinite keysound PCM %s", id)
				}
			}
			cache[id] = clip
		}
		lanes[lane] = clip
	}
	volume := float64(s.KeySoundVolume) / 100
	for mask := 1; mask < len(m.chords); mask++ {
		samples := 0
		for lane, clip := range lanes {
			if mask&(1<<lane) != 0 {
				samples = max(samples, len(clip.PCM))
			}
		}
		pcm := make([]byte, samples*4)
		for sample := 0; sample < samples; sample++ {
			sum := 0.0
			for lane, clip := range lanes {
				if mask&(1<<lane) != 0 && sample < len(clip.PCM) {
					sum += float64(clip.PCM[sample])
				}
			}
			value := float32(.70 * math.Tanh(sum*.5/.70) * volume)
			binary.LittleEndian.PutUint32(pcm[sample*4:], math.Float32bits(value))
		}
		m.chords[mask] = pcm
	}
	return m, nil
}

func LoadKeySound(id string) (*Clip, error) {
	if !validKeySound(id) {
		return nil, fmt.Errorf("unknown keysound %q", id)
	}
	f, err := audioassets.Files.Open(id + ".ogg")
	if err != nil {
		return nil, fmt.Errorf("place processed keysound assets/audio/%s.ogg: %w", id, err)
	}
	defer f.Close()
	clip, err := decode(f, false)
	if err != nil {
		return nil, fmt.Errorf("decode keysound %s: %w", id, err)
	}
	return clip, nil
}

// HandleEvents receives exactly one Session.UpdateAt batch. Duplicate lane
// events collapse into one bit; releases and empty batches leave playback alone.
func (m *KeyStream) HandleEvents(events []judge.Event) {
	mask := 0
	for _, e := range events {
		if e.Kind == judge.Press && e.Lane >= 0 && e.Lane < 4 {
			mask |= 1 << e.Lane
		}
	}
	if mask == 0 {
		return
	}
	m.mu.Lock()
	m.current = m.chords[mask]
	m.cursor = 0
	// Finish at most the already-started stereo frame, avoiding malformed float32
	// bytes on a partial Read. All later old PCM is discarded immediately.
	m.mu.Unlock()
}

// Read is infinite (silence after a chord). Copy at most 256 bytes per lock so
// Press never waits for an entire backend buffer, and never synthesizes samples.
func (m *KeyStream) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		m.mu.Lock()
		size := min(len(p)-n, 256)
		size = int(min(int64(size), math.MaxInt64-m.position))
		if size == 0 {
			m.mu.Unlock()
			return n, fmt.Errorf("keysound stream position overflow")
		}
		m.read(p[n : n+size])
		m.position += int64(size)
		m.mu.Unlock()
		n += size
	}
	return n, nil
}

// read requires mu. cursor always points to a whole stereo frame. Only scratch
// holds a partly delivered frame, so a new chord cannot shift channel alignment.
func (m *KeyStream) read(p []byte) {
	for len(p) > 0 {
		if m.pending > 0 {
			n := copy(p, m.scratch[8-m.pending:])
			m.pending -= n
			p = p[n:]
			continue
		}
		if m.cursor == len(m.current) {
			m.current = nil
			m.cursor = 0
		}
		whole := len(p) / 8 * 8
		if whole > 0 {
			if m.current == nil {
				clear(p[:whole])
				p = p[whole:]
			} else {
				n := min(whole, len(m.current)-m.cursor)
				copy(p[:n], m.current[m.cursor:m.cursor+n])
				m.cursor += n
				p = p[n:]
			}
			continue
		}
		clear(m.scratch[:])
		if m.current != nil {
			copy(m.scratch[:], m.current[m.cursor:m.cursor+8])
			m.cursor += 8
		}
		m.pending = 8
	}
}

// Ebitengine v2.10.5 probes SeekCurrent when first using a NewPlayerF32 source;
// Player.Rewind seeks to zero and drops its own buffered PCM. A live stream has
// no finite end or replayable history: other seeks return ErrUnsupported.
func (m *KeyStream) Seek(offset int64, whence int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if offset == 0 && whence == io.SeekCurrent {
		return m.position, nil
	}
	if offset == 0 && whence == io.SeekStart {
		m.current = nil
		m.cursor = 0
		m.pending = 0
		m.position = 0
		return 0, nil
	}
	return m.position, fmt.Errorf("keysound live stream supports only current-position query and rewind (offset=%d, whence=%d): %w", offset, whence, errors.ErrUnsupported)
}

type KeyPlayer interface {
	Play()
	PauseAndStopReading()
	Rewind() error
	Close() error
}

// KeyPlayback drops tails and buffered audio on focus loss; resume starts silence.
type KeyPlayback struct {
	Player  KeyPlayer
	running bool
}

func (p *KeyPlayback) SetActive(active bool) error {
	if active == p.running {
		return nil
	}
	if active {
		p.Player.Play()
		p.running = true
		return nil
	}
	p.Player.PauseAndStopReading()
	p.running = false
	return p.Player.Rewind()
}
func (p *KeyPlayback) Close() error {
	p.Player.PauseAndStopReading()
	p.running = false
	return p.Player.Close()
}
