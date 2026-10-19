package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"

	audioassets "github.com/usausamikann/4k_game/assets/audio"
	"github.com/usausamikann/4k_game/internal/judge"
)

const KeyVoiceLimit = 32

type keyVoice struct {
	clip   *Clip
	sample int
	serial uint64
}

// KeyMixer owns fixed voice slots and one immutable PCM copy per selected ID.
// Only a short, single-frame critical section is shared with the audio reader.
type KeyMixer struct {
	mu      sync.Mutex
	lanes   [4]*Clip
	voices  [KeyVoiceLimit]keyVoice
	serial  uint64
	volume  float64
	scratch [8]byte
	pending int
}

func NewKeyMixer(s Settings, load func(string) (*Clip, error)) (*KeyMixer, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	m := &KeyMixer{volume: float64(s.KeySoundVolume) / 100}
	cache := map[string]*Clip{}
	for lane := range m.lanes {
		id := s.SoundForLane(lane)
		if id == "" {
			continue
		}
		clip := cache[id]
		if clip == nil {
			var err error
			clip, err = load(id)
			if err != nil {
				return nil, err
			}
			if clip == nil || len(clip.PCM) == 0 || len(clip.PCM)%2 != 0 {
				return nil, fmt.Errorf("invalid keysound PCM %s", id)
			}
			cache[id] = clip
		}
		m.lanes[lane] = clip
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

// HandleEvents consumes the existing Tracker transitions once, independently of judgement.
func (m *KeyMixer) HandleEvents(events []judge.Event) {
	for _, e := range events {
		if e.Kind == judge.Press {
			m.Press(e.Lane)
		}
	}
}
func (m *KeyMixer) Press(lane int) {
	if lane < 0 || lane >= 4 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clip := m.lanes[lane]
	if clip == nil || m.volume == 0 {
		return
	}
	slot := 0
	for i, v := range m.voices {
		if v.clip == nil {
			slot = i
			break
		}
		if v.serial < m.voices[slot].serial {
			slot = i
		}
	}
	m.serial++
	m.voices[slot] = keyVoice{clip: clip, serial: m.serial}
}
func (m *KeyMixer) frame() {
	var sum [2]float64
	for i := range m.voices {
		v := &m.voices[i]
		if v.clip == nil {
			continue
		}
		sum[0] += float64(v.clip.PCM[v.sample])
		sum[1] += float64(v.clip.PCM[v.sample+1])
		v.sample += 2
		if v.sample == len(v.clip.PCM) {
			v.clip = nil
		}
	}
	// Smooth saturation bounds keysounds to +/-0.70 even at voice exhaustion.
	// Fixed common gain preserves the supplied relative asset balance; no per-lane gain.
	for c := 0; c < 2; c++ {
		value := float32(.70 * math.Tanh(sum[c]*.5/.70) * m.volume)
		binary.LittleEndian.PutUint32(m.scratch[c*4:], math.Float32bits(value))
	}
	m.pending = 8
}
func (m *KeyMixer) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		m.mu.Lock()
		if m.pending == 0 {
			m.frame()
		}
		count := copy(p[n:], m.scratch[8-m.pending:])
		m.pending -= count
		m.mu.Unlock()
		n += count
	}
	return n, nil
}

// Seek exists only to reset the dedicated Player: Ebitengine Rewind also discards
// its queued PCM. The metronome stream is never sought or reset.
func (m *KeyMixer) Seek(offset int64, whence int) (int64, error) {
	if offset != 0 || whence != io.SeekStart {
		return 0, fmt.Errorf("keysound mixer supports only rewind")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.voices = [KeyVoiceLimit]keyVoice{}
	m.pending = 0
	return 0, nil
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
