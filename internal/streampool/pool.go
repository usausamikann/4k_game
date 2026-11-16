// Package streampool enumerates valid one-beat, 16th-note stream patterns.
// It does not choose patterns, concatenate them, or generate full charts.
package streampool

import (
	"fmt"
	"math/bits"

	"github.com/usausamikann/4k_game/internal/chart"
)

const Slots = 4

// Mask holds a nonempty subset of D/F/J/K in bits 0/1/2/3.
type Mask uint8

type DensityTier uint8

const (
	Light DensityTier = iota
	Middle
	Heavy
)

type TrillKind string

const (
	NoTrill         TrillKind = ""
	JumpTrill       TrillKind = "jump"       // DF <-> JK
	SymmetricTrill  TrillKind = "symmetric"  // FJ <-> DK
	AsymmetricTrill TrillKind = "asymmetric" // DJ <-> FK
)

// Pattern is one beat with notes at 0, 1/4, 1/2, and 3/4 beats.
// All note counts are exact integers; densities are notes per 16th-note slot.
// IDs pack the four masks into four nibbles, earliest slot first.
type Pattern struct {
	Masks        [Slots]Mask
	FirstMask    Mask
	LastMask     Mask
	FirstTwo     [2]Mask
	LastTwo      [2]Mask
	ID           uint16
	MirrorID     uint16
	CanonicalID  uint16
	TotalNotes   uint8
	LeftNotes    uint8
	RightNotes   uint8
	Density      float64
	LeftDensity  float64
	RightDensity float64
	BalanceDelta int8 // LeftNotes - RightNotes: exact queue-friendly balance
	DensityTier  DensityTier
	Is2222       bool
	Period2      bool // Masks[0:2] == Masks[2:4]
	TrillKind    TrillKind
}

// Pool holds every playable orientation, not just mirror representatives.
// The ByTier slices contain value copies of the entries in All. All fields
// are mutable by the caller; Build constructs a fresh independent pool.
type Pool struct {
	All    []Pattern
	ByTier [3][]Pattern
}

// CanConnect checks the only constraint necessary to prohibit jack at a
// one-beat boundary. Trill length and hand-balance are future generator state.
func CanConnect(a, b Pattern) bool {
	return a.LastMask&b.FirstMask == 0
}

// ToChartPattern provides a single beat in the existing tick-based chart
// format. This is not concatenation or chart generation.
func (p Pattern) ToChartPattern() chart.Pattern {
	notes := make([]chart.Note, 0, p.TotalNotes)
	for slot, mask := range p.Masks {
		for lane := 0; lane < 4; lane++ {
			if mask&(1<<lane) != 0 {
				notes = append(notes, chart.Note{Lane: lane, Tick: int64(slot) * chart.TicksPerBeat / Slots})
			}
		}
	}
	return chart.Pattern{
		Name:      fmt.Sprintf("stream-%04x", p.ID),
		TimeScale: 0,
		Notes:     notes,
	}
}

// Build enumerates all 15^4 nonempty-mask sequences and filters them:
// - every slot has notes and adjacent masks are disjoint (no jack);
// - normally only slots 0 and 2 may have chords, of size at most three;
// - as one explicit exception, every slot may have exactly two notes (2222).
// The latter has precisely six actual orientations without adjacent overlap.
// No sampling, weighting, pattern ordering, or multi-beat generation occurs.
func Build() Pool {
	pool := Pool{All: make([]Pattern, 0, 454)}
	for a := Mask(1); a <= 15; a++ {
		for b := Mask(1); b <= 15; b++ {
			if a&b != 0 {
				continue
			}
			for c := Mask(1); c <= 15; c++ {
				if b&c != 0 {
					continue
				}
				for d := Mask(1); d <= 15; d++ {
					if c&d != 0 {
						continue
					}
					masks := [Slots]Mask{a, b, c, d}
					if !allowedCounts(masks) {
						continue
					}
					p := describe(masks)
					pool.All = append(pool.All, p)
					pool.ByTier[p.DensityTier] = append(pool.ByTier[p.DensityTier], p)
				}
			}
		}
	}
	return pool
}

func allowedCounts(m [Slots]Mask) bool {
	a, b, c, d := count(m[0]), count(m[1]), count(m[2]), count(m[3])
	regular := a >= 1 && a <= 3 && b == 1 && c >= 1 && c <= 3 && d == 1
	special := a == 2 && b == 2 && c == 2 && d == 2
	return regular || special
}

func count(m Mask) int { return bits.OnesCount8(uint8(m)) }

func encode(m [Slots]Mask) uint16 {
	return uint16(m[0]) | uint16(m[1])<<4 | uint16(m[2])<<8 | uint16(m[3])<<12
}

// Mirror flips only the lane axis (D <-> K, F <-> J), not time.
func Mirror(m [Slots]Mask) [Slots]Mask {
	var out [Slots]Mask
	for i, x := range m {
		out[i] = (x&1)<<3 | (x&2)<<1 | (x&4)>>1 | (x&8)>>3
	}
	return out
}

func describe(m [Slots]Mask) Pattern {
	p := Pattern{
		Masks: m, FirstMask: m[0], LastMask: m[3],
		FirstTwo: [2]Mask{m[0], m[1]}, LastTwo: [2]Mask{m[2], m[3]},
		ID: encode(m), MirrorID: encode(Mirror(m)),
		Is2222:  count(m[0]) == 2 && count(m[1]) == 2 && count(m[2]) == 2 && count(m[3]) == 2,
		Period2: m[0] == m[2] && m[1] == m[3],
	}
	p.CanonicalID = min(p.ID, p.MirrorID)
	for _, mask := range m {
		p.TotalNotes += uint8(count(mask))
		p.LeftNotes += uint8(count(mask & 0b0011))
		p.RightNotes += uint8(count(mask & 0b1100))
	}
	p.Density = float64(p.TotalNotes) / Slots
	p.LeftDensity = float64(p.LeftNotes) / Slots
	p.RightDensity = float64(p.RightNotes) / Slots
	p.BalanceDelta = int8(p.LeftNotes) - int8(p.RightNotes)
	switch {
	case p.TotalNotes < 6:
		p.DensityTier = Light
	case p.TotalNotes == 6:
		p.DensityTier = Middle
	default:
		p.DensityTier = Heavy
	}
	if p.Is2222 {
		switch m[0] {
		case 0b0011, 0b1100:
			p.TrillKind = JumpTrill
		case 0b0110, 0b1001:
			p.TrillKind = SymmetricTrill
		case 0b0101, 0b1010:
			p.TrillKind = AsymmetricTrill
		}
	}
	return p
}
