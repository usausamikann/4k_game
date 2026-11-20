//go:build !windows

package input

import (
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
)

type RawReader struct{}

func NewRawReader([chart.LaneCount]uint16) (*RawReader, error) {
	return nil, ErrRawInputUnsupported
}

func (*RawReader) DrainUpTo(time.Time, []RawEvent) ([]RawEvent, uint64, [chart.LaneCount]bool) {
	return nil, 0, [chart.LaneCount]bool{}
}
func (*RawReader) FailedReads() uint64 { return 0 }
func (*RawReader) Close() error        { return nil }
