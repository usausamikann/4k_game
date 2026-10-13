package play

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
)

type NoteGroup struct {
	Time        time.Duration
	NoteIndices []int
}

// Notes and Groups are owned copies, read-only after construction. Indices refer
// to Notes, never to note IDs or judge's internal ordering.
type PlayChart struct {
	Notes     []chart.ScheduledNote
	Groups    []NoteGroup
	indexByID map[int64]int
}

func NewChart(notes []chart.ScheduledNote) (*PlayChart, error) {
	p := &PlayChart{Notes: slices.Clone(notes), indexByID: make(map[int64]int)}
	indices := make([]int, len(notes))
	for i, n := range p.Notes {
		if n.Lane < 0 || n.Lane >= chart.LaneCount || n.Time < 0 || n.AbsoluteTick < 0 {
			return nil, fmt.Errorf("note %d: invalid lane or coordinate", n.ID)
		}
		if _, ok := p.indexByID[n.ID]; ok {
			return nil, fmt.Errorf("duplicate note ID %d", n.ID)
		}
		p.indexByID[n.ID] = i
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool { return p.Notes[indices[i]].Time < p.Notes[indices[j]].Time })
	for _, i := range indices {
		at := p.Notes[i].Time
		if len(p.Groups) == 0 || p.Groups[len(p.Groups)-1].Time != at {
			p.Groups = append(p.Groups, NoteGroup{Time: at})
		}
		group := &p.Groups[len(p.Groups)-1]
		group.NoteIndices = append(group.NoteIndices, i)
	}
	return p, nil
}

// IndexForID supports future per-note updates without copying all judge results.
func (p *PlayChart) IndexForID(id int64) (int, bool) { i, ok := p.indexByID[id]; return i, ok }
