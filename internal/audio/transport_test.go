package audio

import (
	"bytes"
	"github.com/usausamikann/4k_game/internal/play"
	"io"
	"testing"
	"time"
)

type fakePlayer struct {
	stream        *Stream
	playing       bool
	plays, pauses int
	position      time.Duration
}

func (p *fakePlayer) Play()                   { p.playing = true; p.plays++ }
func (p *fakePlayer) PauseAndStopReading()    { p.playing = false; p.pauses++ }
func (p *fakePlayer) Position() time.Duration { return p.position }
func (p *fakePlayer) advance(frames int) []byte {
	if !p.playing {
		return nil
	}
	data := make([]byte, frames*8)
	io.ReadFull(p.stream, data)
	p.position += time.Duration(frames) * time.Second / SampleRate
	return data
}
func TestTransportClockStartupPauseResumeNoDuplicate(t *testing.T) {
	clip := &Clip{PCM: []float32{.5, .5}, Peak: .5}
	stream, _ := NewStream(clip, 180)
	p := &fakePlayer{stream: stream}
	timeline, _ := play.NewTimeline(180)
	ready := false
	transport := NewTransport(p, func() bool { return ready }, timeline.Preparation)
	if transport.Update(true) || p.plays != 0 {
		t.Fatal("started before device ready")
	}
	ready = true
	if transport.Update(false) {
		t.Fatal("started without focus")
	}
	start := time.Now()
	if !transport.Update(true) {
		t.Fatal("did not start")
	}
	clock, _ := play.NewClock(start, timeline.Preparation)
	before := p.advance(SampleRate)
	if transport.Position() != clock.At(start.Add(time.Second)) {
		t.Fatal("initial clock mapping")
	}
	transport.Update(false)
	clock.SetPaused(start.Add(time.Second), true)
	frame := stream.frame
	p.advance(SampleRate * 10)
	if stream.frame != frame || clock.At(start.Add(11*time.Second)) != transport.Position() {
		t.Fatal("unfocused audio/clock advanced")
	}
	transport.Update(false)
	transport.Update(true)
	clock.SetPaused(start.Add(11*time.Second), false)
	after := p.advance(SampleRate)
	if p.plays != 2 || p.pauses != 1 {
		t.Fatal("per-Update restarts")
	}
	if transport.Position() != clock.At(start.Add(12*time.Second)) {
		t.Fatal("resume mapping")
	}
	reference, _ := NewStream(clip, 180)
	all := make([]byte, 2*SampleRate*8)
	io.ReadFull(reference, all)
	if !bytes.Equal(append(before, after...), all) {
		t.Fatal("resume duplicated or lost a beat")
	}
	transport.Close()
	if p.playing {
		t.Fatal("did not stop")
	}
}
