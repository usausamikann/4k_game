// Package audio implements bounded metronome PCM generation and playback control.
// It does not handle judgement, keysounds, or visual offsets.
package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"math"

	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	audioassets "github.com/usausamikann/4k_game/assets/audio"
)

const SampleRate = 48000
const DefaultSound = "glass_002"
const maxClipSeconds = 2

var sounds = [...]string{"switch8", "switch19", "switch29", "glass_002", "pluck_001"}

// Clip contains interleaved stereo float32 samples at SampleRate. Treat PCM as
// immutable. TrimFrames records non-destructive source trimming at output rate.
type Clip struct {
	PCM                                     []float32
	OriginalFrames, TrimFrames, AttackFrame int
	SourceRate                              int
	Peak                                    float64
}

func SoundIDs() []string            { return append([]string(nil), sounds[:]...) }
func Load(id string) (*Clip, error) { return load(audioassets.Files, id) }
func load(files fs.FS, id string) (*Clip, error) {
	valid := false
	for _, name := range sounds {
		if name == id {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("unknown metronome sound %q; choose switch8, switch19, switch29, glass_002, or pluck_001", id)
	}
	f, err := files.Open(id + ".ogg")
	if err != nil {
		return nil, fmt.Errorf("open metronome %s: %w", id, err)
	}
	defer f.Close()
	clip, err := Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode metronome %s: %w", id, err)
	}
	return clip, nil
}

// Decode accepts mono/stereo Ogg Vorbis and normalizes once to 48kHz stereo F32.
// Decoder output is capped, so a malformed or unexpectedly huge file is rejected.
func Decode(src io.Reader) (*Clip, error) {
	decoded, err := vorbis.DecodeF32(src)
	if err != nil {
		return nil, err
	}
	reader := ebaudio.ResampleReaderF32(decoded, decoded.Length(), decoded.SampleRate(), SampleRate)
	limit := int64((maxClipSeconds*SampleRate + 1) * 8)
	raw, err := io.ReadAll(io.LimitReader(reader, limit))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw)%8 != 0 || int64(len(raw)) >= limit {
		return nil, fmt.Errorf("clip must contain stereo frames and be at most %ds", maxClipSeconds)
	}
	samples := make([]float32, len(raw)/4)
	peak := float64(0)
	for i := range samples {
		value := math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("nonfinite decoded sample")
		}
		samples[i] = value
		peak = math.Max(peak, math.Abs(float64(value)))
	}
	if peak == 0 {
		return nil, fmt.Errorf("sound is silent")
	}
	// First sample reaching 10% of peak defines significant attack. Keep 1ms
	// before it to preserve attack transients, rather than cutting at the peak.
	attack := 0
	for i := 0; i < len(samples); i += 2 {
		if math.Max(math.Abs(float64(samples[i])), math.Abs(float64(samples[i+1]))) >= peak*.1 {
			attack = i / 2
			break
		}
	}
	trim := max(0, attack-SampleRate/1000)
	pcm := append([]float32(nil), samples[trim*2:]...)
	// Only trimmed clips need a short edge fade; it ends before significant attack.
	if trim > 0 {
		fade := SampleRate / 2000
		for frame := 0; frame < fade; frame++ {
			gain := float32(frame) / float32(fade)
			pcm[2*frame] *= gain
			pcm[2*frame+1] *= gain
		}
	}
	return &Clip{PCM: pcm, OriginalFrames: len(samples) / 2, TrimFrames: trim, AttackFrame: attack - trim, SourceRate: decoded.SampleRate(), Peak: peak}, nil
}
