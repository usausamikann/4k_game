// Package inputdebug formats optional diagnostics without a graphics dependency.
package inputdebug

import (
	"fmt"
	"time"

	"github.com/usausamikann/4k_game/internal/judge"
)

const DefaultTPS = 240
const RefreshInterval = 250 * time.Millisecond

func ValidateTPS(tps int) error {
	switch tps {
	case 60, 120, 240:
		return nil
	}
	return fmt.Errorf("tps must be 60, 120, or 240: %d", tps)
}

type State struct {
	LastJudge     judge.Rank
	LastHitOffset time.Duration
	HasHit        bool
	EmptyPresses  int
}

type HUD struct {
	target int
	next   time.Time
	text   string
}

func NewHUD(tps int) (*HUD, error) {
	if err := ValidateTPS(tps); err != nil {
		return nil, err
	}
	return &HUD{target: tps}, nil
}

// Due uses wall time, so diagnostics still refresh when the game clock is paused.
func (h *HUD) Due(at time.Time) bool { return h.next.IsZero() || !at.Before(h.next) }
func (h *HUD) Refresh(at time.Time, actualTPS, actualFPS float64, state State) {
	if !h.Due(at) {
		return
	}
	rank := string(state.LastJudge)
	if rank == "" {
		rank = "-"
	}
	offset := "- (no Hit yet)"
	if state.HasHit {
		offset = FormatOffset(state.LastHitOffset)
	}
	h.text = fmt.Sprintf("INPUT DEBUG\nTarget TPS %d\nTPS %.1f\nFPS %.1f\nLast judge %s\nLast HIT offset:\n%s\n(A/B adjusted)\nEmpty presses %d", h.target, actualTPS, actualFPS, rank, offset, state.EmptyPresses)
	h.next = at.Add(RefreshInterval)
}
func (h *HUD) Text() string { return h.text }

// Negative is FAST, positive is LATE. Only call this for an actual Hit offset.
func FormatOffset(offset time.Duration) string {
	timing := "ON TIME"
	if offset < 0 {
		timing = "FAST"
	} else if offset > 0 {
		timing = "LATE"
	}
	return fmt.Sprintf("%+.1fms %s", float64(offset)/float64(time.Millisecond), timing)
}
