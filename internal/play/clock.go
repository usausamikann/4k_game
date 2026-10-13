// Package play contains GUI-independent playback time and rendering calculations.
package play

import (
	"fmt"
	"sort"
	"time"
)

type epoch struct {
	wall   time.Time
	game   time.Duration
	paused bool
}

// Clock converts timestamps from Go's monotonic time axis to game time.
// Keep time.Time values obtained from time.Now; serialization loses monotonic data.
// Pause history also permits conversion of queued input from earlier intervals.
// Methods must be called from the same goroutine.
type Clock struct {
	preparation time.Duration
	epochs      []epoch
}

func NewClock(start time.Time, preparation time.Duration) (*Clock, error) {
	if preparation < 0 {
		return nil, fmt.Errorf("preparation must be nonnegative")
	}
	c := &Clock{preparation: preparation}
	c.Reset(start)
	return c, nil
}

func (c *Clock) Reset(start time.Time) {
	c.epochs = append(c.epochs[:0], epoch{wall: start, game: -c.preparation})
}

// At performs a direct elapsed-time calculation; no frame count is involved.
func (c *Clock) At(wall time.Time) time.Duration {
	i := sort.Search(len(c.epochs), func(i int) bool { return c.epochs[i].wall.After(wall) }) - 1
	if i < 0 {
		i = 0
	}
	e := c.epochs[i]
	if e.paused {
		return e.game
	}
	return e.game + wall.Sub(e.wall)
}

func (c *Clock) SetPaused(wall time.Time, paused bool) error {
	last := c.epochs[len(c.epochs)-1]
	if wall.Before(last.wall) {
		return fmt.Errorf("clock transition moved backwards")
	}
	if last.paused == paused {
		return nil
	}
	at := c.At(wall)
	c.epochs = append(c.epochs, epoch{wall: wall, game: at, paused: paused})
	return nil
}
func (c *Clock) Paused() bool { return c.epochs[len(c.epochs)-1].paused }
