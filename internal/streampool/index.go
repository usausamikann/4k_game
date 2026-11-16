package streampool

// Index stores indices into Pool.All, never copies of Pattern objects.
// G[first][tier] contains patterns whose FirstMask equals first.
// H[allowed][tier] contains patterns whose FirstMask is a nonempty submask
// of allowed. A generator can use H[15 &^ previousLast][tier] directly.
type Index struct {
	G [16][3][]int
	H [16][3][]int
}

// BuildIndex builds both lookup tables once for the given pool.
// The caller must not reorder or replace Pool.All afterward.
func BuildIndex(pool Pool) Index {
	var index Index
	for id, p := range pool.All {
		index.G[p.FirstMask][p.DensityTier] = append(index.G[p.FirstMask][p.DensityTier], id)
	}
	for allowed := 0; allowed < 16; allowed++ {
		for first := allowed; first != 0; first = (first - 1) & allowed {
			for tier := 0; tier < 3; tier++ {
				index.H[allowed][tier] = append(index.H[allowed][tier], index.G[first][tier]...)
			}
		}
	}
	return index
}
