package streampool

import "testing"

func TestIndexExactAndSubmask(t *testing.T) {
	pool := Build()
	idx := BuildIndex(pool)
	for first := 0; first < 16; first++ {
		for tier := 0; tier < 3; tier++ {
			seen := map[int]bool{}
			for _, id := range idx.G[first][tier] {
				if id < 0 || id >= len(pool.All) || seen[id] {
					t.Fatalf("invalid/duplicate G[%d][%d] id %d", first, tier, id)
				}
				seen[id] = true
				p := pool.All[id]
				if int(p.FirstMask) != first || int(p.DensityTier) != tier {
					t.Fatalf("wrong exact-first entry: %+v", p)
				}
			}
			for id, p := range pool.All {
				want := int(p.FirstMask) == first && int(p.DensityTier) == tier
				if seen[id] != want {
					t.Fatalf("G[%d][%d] membership mismatch for id %d", first, tier, id)
				}
			}
		}
	}
	for allowed := 0; allowed < 16; allowed++ {
		for tier := 0; tier < 3; tier++ {
			seen := map[int]bool{}
			for _, id := range idx.H[allowed][tier] {
				if id < 0 || id >= len(pool.All) || seen[id] {
					t.Fatalf("invalid/duplicate H[%d][%d] id %d", allowed, tier, id)
				}
				seen[id] = true
				p := pool.All[id]
				if int(p.FirstMask)&allowed != int(p.FirstMask) || int(p.DensityTier) != tier {
					t.Fatalf("wrong allowed-mask entry: %+v", p)
				}
			}
			for id, p := range pool.All {
				want := int(p.DensityTier) == tier && int(p.FirstMask)&allowed == int(p.FirstMask)
				if seen[id] != want {
					t.Fatalf("H[%d][%d] membership mismatch for id %d", allowed, tier, id)
				}
			}
		}
	}
}
