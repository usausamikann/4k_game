package streampool

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

const (
	DurationShort    = 1.5
	DurationNormal   = 3.0
	DurationLong     = 5.0
	DurationMarathon = 10.0
	maxBeats         = 1_000_000
)

// GeneratorConfig describes a finite fixed-BPM stream. TargetMinutes may be
// a duration preset or any other positive finite duration in minutes.
// TierWeights specify a desired overall ratio, not independent probabilities.
type GeneratorConfig struct {
	BPM           float64
	TargetMinutes float64
	TierWeights   [3]float64
	BalanceWindow int // recent one-beat patterns in the hand-balance window
	MaxBalance    int // maximum absolute difference of L and R note counts
	Seed          int64
}

func DefaultGeneratorConfig() GeneratorConfig {
	return GeneratorConfig{
		BPM:           180,
		TargetMinutes: DurationShort,
		TierWeights:   [3]float64{1, 1, 1},
		BalanceWindow: 8,
		MaxBalance:    8,
		Seed:          0,
	}
}

// Sequence indexes Pool.All in playback order. It does not schedule notes.
// TierTargets are the quotas computed from weights; TierCounts are the real
// counts. They may differ if no tier with an unmet quota has legal candidates.
type Sequence struct {
	BPM            float64
	TargetMinutes  float64
	ActualMinutes  float64
	BeatCount      int
	TierTargets    [3]int
	TierCounts     [3]int
	PatternIndices []int
}

// BeatCount rounds a nominal duration to the nearest multiple of THREE
// one-beat patterns, not twelve. Exact halfway cases round upward.
func BeatCount(bpm, minutes float64) (int, error) {
	if !positiveFinite(bpm) || !positiveFinite(minutes) {
		return 0, fmt.Errorf("BPM and TargetMinutes must be positive and finite")
	}
	beats := math.Round(bpm*minutes/3) * 3
	if beats < 3 || beats > maxBeats || math.IsInf(beats, 0) || math.IsNaN(beats) {
		return 0, fmt.Errorf("beat count %.4g is outside supported range [3,%d]", beats, maxBeats)
	}
	return int(beats), nil
}

func positiveFinite(x float64) bool {
	return x > 0 && !math.IsNaN(x) && !math.IsInf(x, 0)
}

// TierTargets uses largest remainders to assign integer quotas that sum to n.
// Changing TierWeights only changes the quotas, not the generator algorithm.
func TierTargets(n int, weights [3]float64) ([3]int, error) {
	var result [3]int
	if n <= 0 || n > maxBeats {
		return result, fmt.Errorf("invalid beat count: %d", n)
	}
	sum := 0.0
	for _, w := range weights {
		if w < 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			return result, fmt.Errorf("tier weights must be finite and nonnegative")
		}
		sum += w
	}
	if !positiveFinite(sum) {
		return result, fmt.Errorf("sum of tier weights must be positive and finite")
	}
	var remainder [3]float64
	assigned := 0
	for i, w := range weights {
		amount := float64(n) * (w / sum)
		result[i] = int(math.Floor(amount))
		remainder[i] = amount - float64(result[i])
		assigned += result[i]
	}
	order := []int{0, 1, 2}
	sort.SliceStable(order, func(i, j int) bool {
		return remainder[order[i]] > remainder[order[j]]
	})
	for i := 0; i < n-assigned; i++ {
		result[order[i]]++
	}
	return result, nil
}

// Generate selects fixed-length pattern indices. It uses remaining tier
// quotas as draw weights (sampling without replacement), then draws a random
// playable pattern from Index.H[allowedFirstMask][tier] subject to a rolling
// left/right balance bound. The tier fallback does not violate hard constraints
// but may deviate from quotas. If no pattern is legal, an error is returned.
// In particular this function does NOT suppress period-2 or trill runs.
func Generate(pool Pool, index Index, cfg GeneratorConfig) (Sequence, error) {
	var sequence Sequence
	n, err := BeatCount(cfg.BPM, cfg.TargetMinutes)
	if err != nil {
		return sequence, err
	}
	if cfg.BalanceWindow <= 0 || cfg.BalanceWindow > maxBeats || cfg.MaxBalance < 0 {
		return sequence, fmt.Errorf("BalanceWindow must be in [1,%d] and MaxBalance nonnegative", maxBeats)
	}
	targets, err := TierTargets(n, cfg.TierWeights)
	if err != nil {
		return sequence, err
	}
	if len(pool.All) == 0 {
		return sequence, fmt.Errorf("pattern pool is empty")
	}
	sequence = Sequence{
		BPM: cfg.BPM, TargetMinutes: cfg.TargetMinutes,
		ActualMinutes: float64(n)/cfg.BPM, BeatCount: n,
		TierTargets: targets, PatternIndices: make([]int, 0, n),
	}

	random := rand.New(rand.NewSource(cfg.Seed))
	window := make([]int, cfg.BalanceWindow)
	windowLen, head, handSum := 0, 0, 0
	var prev Mask // zero means the sequence has no previous pattern
	var feasible [3][]int
	for beat := 0; beat < n; beat++ {
		allowed := Mask(15) &^ prev
		baseSum := handSum
		if windowLen == cfg.BalanceWindow {
			baseSum -= window[head]
		}
		var remaining [3]int
		var available [3]bool
		for tier := 0; tier < 3; tier++ {
			feasible[tier] = feasible[tier][:0]
			for _, id := range index.H[allowed][tier] {
				if id < 0 || id >= len(pool.All) {
					return Sequence{}, fmt.Errorf("invalid pool index: %d", id)
				}
				candidate := pool.All[id]
				if absInt(baseSum+int(candidate.BalanceDelta)) <= cfg.MaxBalance {
					feasible[tier] = append(feasible[tier], id)
				}
			}
			available[tier] = len(feasible[tier]) > 0
			remaining[tier] = max(0, targets[tier]-sequence.TierCounts[tier])
		}
		tier, ok := drawTier(random, remaining, available)
		if !ok {
			return Sequence{}, fmt.Errorf("no legal pattern at beat %d: prev=%04b handSum=%d", beat, prev, handSum)
		}
		choices := feasible[tier]
		id := choices[random.Intn(len(choices))]
		p := pool.All[id]
		sequence.PatternIndices = append(sequence.PatternIndices, id)
		sequence.TierCounts[tier]++
		prev = p.LastMask

		if windowLen == cfg.BalanceWindow {
			handSum -= window[head]
			head = (head + 1) % cfg.BalanceWindow
			windowLen--
		}
		window[(head+windowLen)%cfg.BalanceWindow] = int(p.BalanceDelta)
		windowLen++
		handSum += int(p.BalanceDelta)
	}
	return sequence, nil
}

// drawTier first considers available tiers with unmet targets, weighting each
// by its remaining quota. When all unmet tiers are blocked, any available tier
// can be drawn (even if its target was already met).
func drawTier(r *rand.Rand, remaining [3]int, available [3]bool) (int, bool) {
	var weights [3]int
	total := 0
	for i := 0; i < 3; i++ {
		if available[i] {
			weights[i] = remaining[i]
			total += weights[i]
		}
	}
	if total == 0 {
		for i := 0; i < 3; i++ {
			if available[i] {
				weights[i] = 1
				total++
			}
		}
	}
	if total == 0 {
		return 0, false
	}
	roll := r.Intn(total)
	for i := 0; i < 3; i++ {
		if roll < weights[i] {
			return i, true
		}
		roll -= weights[i]
	}
	panic("unreachable: tier weights must sum to total")
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
