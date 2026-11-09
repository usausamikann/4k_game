package streampool

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/usausamikann/4k_game/internal/chart"
)

func TestPoolAllPatternsAndDensity(t *testing.T) {
	p := Build()
	if len(p.All) != 454 {
		t.Fatalf("got %d patterns, want 454", len(p.All))
	}
	wantTiers := [3]int{288, 120, 46}
	for i, want := range wantTiers {
		if got := len(p.ByTier[i]); got != want {
			t.Errorf("tier %d: got %d, want %d", i, got, want)
		}
	}
	seen := make(map[uint16]bool)
	mirrors := make(map[uint16]bool)
	special, period2, selfMirror := 0, 0, 0
	for _, x := range p.All {
		if seen[x.ID] {
			t.Errorf("duplicate ID %04x", x.ID)
		}
		seen[x.ID] = true
		mirrors[x.CanonicalID] = true
		if x.MirrorID == x.ID {
			selfMirror++
		}
		if x.Is2222 {
			special++
		}
		if x.Period2 {
			period2++
		}
		if x.FirstMask != x.Masks[0] || x.LastMask != x.Masks[3] ||
			x.FirstTwo != [2]Mask{x.Masks[0], x.Masks[1]} || x.LastTwo != [2]Mask{x.Masks[2], x.Masks[3]} {
			t.Errorf("bad prefix/suffix for %04x", x.ID)
		}
		if !allowedCounts(x.Masks) {
			t.Errorf("bad counts for %04x", x.ID)
		}
		for i, mask := range x.Masks {
			if mask == 0 || mask > 15 {
				t.Errorf("invalid mask %d at %d", mask, i)
			}
			if i > 0 && (x.Masks[i-1]&mask) != 0 {
				t.Errorf("jack at slot %d for %04x", i, x.ID)
			}
		}
		if x.TotalNotes != x.LeftNotes+x.RightNotes {
			t.Errorf("bad total for %04x", x.ID)
		}
		if x.Density != x.LeftDensity+x.RightDensity {
			t.Errorf("bad densities for %04x", x.ID)
		}
		if x.BalanceDelta != int8(x.LeftNotes)-int8(x.RightNotes) {
			t.Errorf("bad balance for %04x", x.ID)
		}
		if x.MirrorID != encode(Mirror(x.Masks)) || x.CanonicalID != min(x.ID, x.MirrorID) {
			t.Errorf("bad mirror ID for %04x", x.ID)
		}
		if x.Period2 != (x.Masks[0] == x.Masks[2] && x.Masks[1] == x.Masks[3]) {
			t.Errorf("bad period2 for %04x", x.ID)
		}
		if x.Is2222 && x.TrillKind == NoTrill {
			t.Errorf("missing trill kind for %04x", x.ID)
		}
		if !x.Is2222 && x.TrillKind != NoTrill {
			t.Errorf("unexpected trill kind for %04x", x.ID)
		}
		if x.Is2222 && x.DensityTier != Heavy {
			t.Errorf("special not heavy for %04x", x.ID)
		}
	}
	if special != 6 || period2 != 34 || selfMirror != 2 || len(mirrors) != 228 {
		t.Errorf("special=%d period2=%d selfmirror=%d classes=%d, want 6/34/2/228", special, period2, selfMirror, len(mirrors))
	}
	for _, x := range p.All {
		if !seen[x.MirrorID] {
			t.Errorf("mirror %04x missing for %04x", x.MirrorID, x.ID)
		}
	}
	if !reflect.DeepEqual(p.All, Build().All) {
		t.Error("Build not deterministic")
	}
}

func TestNoChordsOnWeakPositionsExcept2222(t *testing.T) {
	for _, p := range Build().All {
		if !p.Is2222 && (count(p.Masks[1]) != 1 || count(p.Masks[3]) != 1) {
			t.Fatalf("chords on offbeats: %04x", p.ID)
		}
		if p.DensityTier == Light && p.TotalNotes >= 6 ||
			p.DensityTier == Middle && p.TotalNotes != 6 ||
			p.DensityTier == Heavy && p.TotalNotes < 7 {
			t.Fatalf("bad tier %04x", p.ID)
		}
	}
}

func TestSpecialTrillsAndHandBalance(t *testing.T) {
	pool := Build()
	countKind := map[TrillKind]int{}
	for _, p := range pool.All {
		if p.Is2222 {
			countKind[p.TrillKind]++
		}
	}
	for _, kind := range []TrillKind{JumpTrill, SymmetricTrill, AsymmetricTrill} {
		if countKind[kind] != 2 {
			t.Fatalf("%s count=%d", kind, countKind[kind])
		}
	}
	samples := map[[4]Mask]struct {
		notes   int
		balance int8
		tier    DensityTier
		period2 bool
	}{
		{7, 8, 7, 8}:   {8, 0, Heavy, true},
		{1, 2, 4, 8}:   {4, 0, Light, false},
		{1, 2, 1, 2}:   {4, 4, Light, true},
		{3, 12, 3, 12}: {8, 0, Heavy, true},
	}
	for masks, want := range samples {
		id := encode(masks)
		found := false
		for _, p := range pool.All {
			if p.ID == id {
				found = true
				if int(p.TotalNotes) != want.notes || p.BalanceDelta != want.balance || p.DensityTier != want.tier || p.Period2 != want.period2 {
					t.Errorf("pattern %04x got %+v, want %+v", id, p, want)
				}
				break
			}
		}
		if !found {
			t.Errorf("missing example %04x", id)
		}
	}
}

func TestCanConnectAndChartAdapter(t *testing.T) {
	p := Build()
	for _, candidate := range p.All {
		chartPattern := candidate.ToChartPattern()
		if err := chart.Validate(chartPattern); err != nil {
			t.Fatalf("%04x: chart.Validate: %v", candidate.ID, err)
		}
		if chartPattern.TimeScale != 0 || len(chartPattern.Notes) != int(candidate.TotalNotes) {
			t.Fatalf("%04x bad chart", candidate.ID)
		}
		for _, note := range chartPattern.Notes {
			slot := int(note.Tick / (chart.TicksPerBeat / 4))
			if note.Tick != int64(slot)*chart.TicksPerBeat/4 || slot < 0 || slot >= Slots ||
				candidate.Masks[slot]&(1<<note.Lane) == 0 {
				t.Fatalf("bad note for %04x: %+v", candidate.ID, note)
			}
		}
		expectedName := fmt.Sprintf("stream-%04x", candidate.ID)
		if chartPattern.Name != expectedName {
			t.Errorf("got name %q want %q", chartPattern.Name, expectedName)
		}
	}
	// Valid boundary / invalid boundary independent of density.
	a := describe([4]Mask{1, 2, 4, 8})
	b := describe([4]Mask{1, 2, 4, 8})
	c := describe([4]Mask{8, 4, 2, 1})
	if !CanConnect(a, b) {
		t.Fatal("D after K should connect")
	}
	if CanConnect(a, c) {
		t.Fatal("K after K should be forbidden")
	}
}
