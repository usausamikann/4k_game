// Command export-stream writes a generated stream as a standalone multi-beat
// JSON chart. It does not require the game UI or change game runtime flags.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/usausamikann/4k_game/internal/streampool"
)

type note struct {
	Lane     int      `json:"lane"`
	Position [2]int64 `json:"position"`
}

type chartJSON struct {
	Name        string `json:"name"`
	TimeScale   int64  `json:"time_scale"`
	LengthBeats int    `json:"length_beats"`
	Notes       []note `json:"notes"`
}

func main() {
	bpm := flag.Float64("bpm", 120, "chart BPM")
	minutes := flag.Float64("minutes", streampool.DurationShort, "target minutes, e.g. 1.5/3/5/10")
	seed := flag.Int64("seed", 42, "deterministic random seed")
	output := flag.String("out", "patterns/stream-preview.json", "destination JSON path")
	flag.Parse()
	if err := export(*bpm, *minutes, *seed, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func export(bpm, minutes float64, seed int64, output string) error {
	pool := streampool.Build()
	index := streampool.BuildIndex(pool)
	cfg := streampool.DefaultGeneratorConfig()
	cfg.BPM, cfg.TargetMinutes, cfg.Seed = bpm, minutes, seed
	sequence, err := streampool.Generate(pool, index, cfg)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	result := chartJSON{
		Name:        fmt.Sprintf("stream-bpm%g-min%g-seed%d", bpm, minutes, seed),
		TimeScale:   0,
		LengthBeats: sequence.BeatCount,
		Notes:       make([]note, 0, sequence.BeatCount*6),
	}
	for beat, id := range sequence.PatternIndices {
		if id < 0 || id >= len(pool.All) {
			return fmt.Errorf("invalid pattern index %d", id)
		}
		p := pool.All[id]
		for slot, mask := range p.Masks {
			for lane := 0; lane < 4; lane++ {
				if mask&(1<<lane) != 0 {
					result.Notes = append(result.Notes, note{
						Lane:     lane,
						Position: [2]int64{int64(beat*4 + slot), 4},
					})
				}
			}
		}
	}
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create %q: %w", output, err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	writeErr := encoder.Encode(result)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("encode %q: %w", output, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %q: %w", output, closeErr)
	}
	fmt.Printf("Wrote %s: %d beats (%.3f min at BPM %g), %d notes; tier counts %v, target %v, seed %d\n",
		output, sequence.BeatCount, sequence.ActualMinutes, bpm, len(result.Notes),
		sequence.TierCounts, sequence.TierTargets, seed)
	return nil
}
