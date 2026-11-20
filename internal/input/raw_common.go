package input

import (
	"errors"
	"sync"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/judge"
)

// ErrRawInputUnsupported allows the caller to use the Ebitengine fallback.
var ErrRawInputUnsupported = errors.New("raw keyboard input unsupported on this OS")

const rawQueueCapacity = 1024

type rawQueue struct {
	mu          sync.Mutex
	buf         [rawQueueCapacity]RawEvent
	head, count int
	dropped     uint64
	physDown    [chart.LaneCount]bool
}

func (q *rawQueue) push(ev RawEvent) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if ev.Lane >= 0 && ev.Lane < chart.LaneCount {
		q.physDown[ev.Lane] = ev.Kind == judge.Press
	}
	if q.count == len(q.buf) {
		q.head = (q.head + 1) % len(q.buf)
		q.count--
		q.dropped++
	}
	q.buf[(q.head+q.count)%len(q.buf)] = ev
	q.count++
}

// A lost transition invalidates the batch. Return a physical-key snapshot so
// the game can release stale presses and block keys still physically held.
// A complete batch only drains events received by the supplied wall time.
func (q *rawQueue) drainUpTo(limit time.Time, dst []RawEvent) ([]RawEvent, uint64, [chart.LaneCount]bool) {
	dst = dst[:0]
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.dropped != 0 {
		dropped, down := q.dropped, q.physDown
		q.head, q.count, q.dropped = 0, 0, 0
		return dst, dropped, down
	}
	for q.count > 0 {
		ev := q.buf[q.head]
		if ev.Wall.After(limit) {
			break
		}
		dst = append(dst, ev)
		q.head = (q.head + 1) % len(q.buf)
		q.count--
	}
	return dst, 0, q.physDown
}
