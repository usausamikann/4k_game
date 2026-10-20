package chart

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
)

// Load reads exactly one JSON pattern, validates it, and sorts its notes.
func Load(path string) (Pattern, error) {
	f, err := os.Open(path)
	if err != nil {
		return Pattern{}, fmt.Errorf("open pattern %q: %w", path, err)
	}
	defer f.Close()
	pattern, err := decode(f)
	if err != nil {
		return Pattern{}, fmt.Errorf("load pattern %q: %w", path, err)
	}
	return pattern, nil
}

func decode(r io.Reader) (Pattern, error) {
	// Pointers distinguish omitted/null fields from valid zero values.
	var raw struct {
		Name      *string         `json:"name"`
		TimeScale json.RawMessage `json:"time_scale"`
		Notes     *[]struct {
			Lane     *int     `json:"lane"`
			Position []*int64 `json:"position"`
		} `json:"notes"`
	}
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return Pattern{}, fmt.Errorf("decode JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Pattern{}, fmt.Errorf("expected exactly one JSON pattern")
		}
		return Pattern{}, fmt.Errorf("trailing JSON: %w", err)
	}
	if raw.Name == nil || raw.Notes == nil {
		return Pattern{}, fmt.Errorf("name and notes are required and must not be null")
	}
	pattern := Pattern{Name: *raw.Name, Notes: make([]Note, len(*raw.Notes))}
	if raw.TimeScale != nil {
		if string(raw.TimeScale) == "null" {
			return Pattern{}, fmt.Errorf("time_scale must not be null")
		}
		if err := json.Unmarshal(raw.TimeScale, &pattern.TimeScale); err != nil {
			return Pattern{}, fmt.Errorf("decode time_scale: %w", err)
		}
	}
	for i, note := range *raw.Notes {
		if note.Lane == nil || note.Position == nil {
			return Pattern{}, fmt.Errorf("note %d: lane and position are required and must not be null", i)
		}
		if len(note.Position) != 2 {
			return Pattern{}, fmt.Errorf("note %d: position must contain exactly two integers", i)
		}
		if note.Position[0] == nil || note.Position[1] == nil {
			return Pattern{}, fmt.Errorf("note %d: position elements must be integers and must not be null", i)
		}
		tick, err := positionToTick(*note.Position[0], *note.Position[1])
		if err != nil {
			return Pattern{}, fmt.Errorf("note %d: %w", i, err)
		}
		pattern.Notes[i] = Note{Lane: *note.Lane, Tick: tick}
	}
	notes, err := validatedNotes(pattern)
	if err != nil {
		return Pattern{}, err
	}
	pattern.Notes = notes
	return pattern, nil
}

// positionToTick converts a beat fraction to an original tick, independently of
// time_scale. Big integers keep 960*numerator exact even near int64 limits.
func positionToTick(numerator, denominator int64) (int64, error) {
	if denominator <= 0 {
		return 0, fmt.Errorf("position denominator must be positive: %d", denominator)
	}
	if numerator < 0 || numerator >= denominator {
		return 0, fmt.Errorf("position must satisfy 0 <= numerator < denominator: [%d, %d]", numerator, denominator)
	}
	product := new(big.Int).Mul(big.NewInt(TicksPerBeat), big.NewInt(numerator))
	tick, remainder := new(big.Int), new(big.Int)
	tick.QuoRem(product, big.NewInt(denominator), remainder)
	if remainder.Sign() != 0 {
		return 0, fmt.Errorf("position [%d, %d] must yield an integer original tick", numerator, denominator)
	}
	// The checked fraction is in [0, 1), so the quotient is in [0, 960).
	return tick.Int64(), nil
}
