package chart

import (
	"encoding/json"
	"fmt"
	"io"
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
		Name        *string         `json:"name"`
		LengthTicks *int64          `json:"length_ticks"`
		TimeScale   json.RawMessage `json:"time_scale"`
		Notes       *[]struct {
			Lane *int   `json:"lane"`
			Tick *int64 `json:"tick"`
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
	if raw.Name == nil || raw.LengthTicks == nil || raw.Notes == nil {
		return Pattern{}, fmt.Errorf("name, length_ticks, and notes are required and must not be null")
	}
	pattern := Pattern{Name: *raw.Name, LengthTicks: *raw.LengthTicks, Notes: make([]Note, len(*raw.Notes))}
	if raw.TimeScale != nil {
		if string(raw.TimeScale) == "null" {
			return Pattern{}, fmt.Errorf("time_scale must not be null")
		}
		if err := json.Unmarshal(raw.TimeScale, &pattern.TimeScale); err != nil {
			return Pattern{}, fmt.Errorf("decode time_scale: %w", err)
		}
	}
	for i, note := range *raw.Notes {
		if note.Lane == nil || note.Tick == nil {
			return Pattern{}, fmt.Errorf("note %d: lane and tick are required and must not be null", i)
		}
		pattern.Notes[i] = Note{Lane: *note.Lane, Tick: *note.Tick}
	}
	notes, err := validatedNotes(pattern)
	if err != nil {
		return Pattern{}, err
	}
	pattern.Notes = notes
	return pattern, nil
}
