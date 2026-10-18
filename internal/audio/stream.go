package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const maxVoices = 32

type voice struct{ frame int }

// Stream is a single-consumer, infinite stereo F32 LE pull reader. Only its clip
// and a fixed voice array are retained, independent of session length.
type Stream struct {
	clip                            *Clip
	schedule                        *Schedule
	voices                          [maxVoices]voice
	count                           int
	frame, beat, next, silenceStart int64
	gain                            float64
	fade                            int64
	scratch                         [8]byte
	pending                         int
	err                             error
}

func NewStream(clip *Clip, bpm float64) (*Stream, error) {
	if clip == nil || len(clip.PCM) == 0 || len(clip.PCM)%2 != 0 || clip.Peak <= 0 || math.IsNaN(clip.Peak) || math.IsInf(clip.Peak, 0) {
		return nil, fmt.Errorf("invalid PCM clip")
	}
	schedule, err := NewSchedule(bpm)
	if err != nil {
		return nil, err
	}
	// Avoid converting a huge beat gap to int or adding it to a sample count.
	gap, err := schedule.minimumGap()
	if err != nil {
		return nil, err
	}
	if gap <= 0 {
		return nil, fmt.Errorf("audio beat interval out of range")
	}
	frames := int64(len(clip.PCM) / 2)
	overlap := 1 + (frames-1)/gap
	if overlap > maxVoices {
		return nil, fmt.Errorf("audio BPM causes more than %d overlapping clicks", maxVoices)
	}
	silent, err := schedule.Frame(4)
	if err != nil {
		return nil, err
	}
	if _, err := schedule.Frame(8); err != nil {
		return nil, err
	}
	// Fixed gain for the entire stream, not a per-beat accent or a dynamic limiter.
	gain := math.Min(1, .8/(clip.Peak*float64(overlap)))
	return &Stream{clip: clip, schedule: schedule, silenceStart: silent, gain: gain, fade: min(int64(SampleRate/500), gap/4)}, nil
}

func (s *Stream) render() {
	if s.frame == s.silenceStart {
		s.count = 0
	} // No COUNT tails may reappear in PLAY.
	if s.frame == s.next {
		s.voices[s.count] = voice{}
		s.count++
		s.beat++
		if s.beat == 4 {
			s.beat = 8
		} // Four deliberately silent beat onsets.
		s.next, s.err = s.schedule.Frame(s.beat)
	}
	var left, right float64
	alive := 0
	for i := 0; i < s.count; i++ {
		v := s.voices[i]
		left += float64(s.clip.PCM[v.frame*2])
		right += float64(s.clip.PCM[v.frame*2+1])
		v.frame++
		if v.frame < len(s.clip.PCM)/2 {
			s.voices[alive] = v
			alive++
		}
	}
	s.count = alive
	gain := s.gain
	// If a COUNT tail extends into SILENT at unusually high BPM, fade its last
	// 2ms before the boundary; the continuous stream itself is never restarted.
	fade := s.fade
	if s.frame < s.silenceStart && s.frame > s.silenceStart-fade {
		gain *= float64(s.silenceStart-s.frame) / float64(fade)
	}
	left = math.Max(-1, math.Min(1, left*gain))
	right = math.Max(-1, math.Min(1, right*gain))
	binary.LittleEndian.PutUint32(s.scratch[:4], math.Float32bits(float32(left)))
	binary.LittleEndian.PutUint32(s.scratch[4:], math.Float32bits(float32(right)))
	if s.frame == math.MaxInt64 {
		s.err = io.EOF
	} else {
		s.frame++
	}
}

// Read supports arbitrary byte lengths, including reads smaller than one frame.
func (s *Stream) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		if s.pending == 0 {
			if s.err != nil {
				return written, s.err
			}
			s.render()
			s.pending = 8
		}
		n := copy(p[written:], s.scratch[8-s.pending:])
		s.pending -= n
		written += n
	}
	return written, nil
}
