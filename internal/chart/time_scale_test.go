package chart

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDecodeTimeScale(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    int64
		errText string
	}{
		{"", 0, ""}, {",\"time_scale\":0", 0, ""}, {",\"time_scale\":1", 1, ""},
		{",\"time_scale\":null", 0, "time_scale must not be null"},
		{",\"time_scale\":-1", 0, "time_scale must"},
		{",\"time_scale\":2", 0, "time_scale must"},
		{",\"time_scale\":9223372036854775807", 0, "time_scale must"},
		{",\"time_scale\":9223372036854775808", 0, "time_scale"},
		{",\"time_scale\":0.5", 0, "time_scale"},
		{",\"time_scale\":\"1\"", 0, "time_scale"},
		{",\"time_scale\":true", 0, "time_scale"},
		{",\"time_scale\":[]", 0, "time_scale"},
		{",\"time_scale\":{}", 0, "time_scale"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			p, err := decode(strings.NewReader(`{"name":"x","length_ticks":960,"notes":[]` + tc.value + `}`))
			if tc.errText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errText) {
					t.Fatalf("error = %v, want %q", err, tc.errText)
				}
				return
			}
			if err != nil || p.TimeScale != tc.want {
				t.Fatalf("pattern = %+v, error = %v", p, err)
			}
		})
	}
}

func TestTimeScaleScheduleAndEnd(t *testing.T) {
	for _, scale := range []int64{0, 1} {
		for _, bpm := range []float64{120, 180, 120.5} {
			t.Run(fmt.Sprintf("scale%d/bpm%g", scale, bpm), func(t *testing.T) {
				p := Pattern{LengthTicks: 960, TimeScale: scale, Notes: []Note{{0, 720}, {0, 240}, {0, 0}, {1, 480}, {0, 480}}}
				original := append([]Note(nil), p.Notes...)
				notes, err := Schedule(p, bpm)
				if err != nil {
					t.Fatal(err)
				}
				ticks := []int64{0, 240, 480, 480, 720}
				lanes := []int{0, 0, 0, 1, 0}
				for i, n := range notes {
					tick := ticks[i] / (scale + 1)
					at, err := TickToTime(tick, bpm)
					if err != nil {
						t.Fatal(err)
					}
					if n != (ScheduledNote{ID: int64(i), Lane: lanes[i], AbsoluteTick: tick, Time: at}) {
						t.Fatalf("note %d = %+v", i, n)
					}
				}
				if !reflect.DeepEqual(original, p.Notes) {
					t.Fatal("modified source ticks")
				}
				for _, empty := range []bool{false, true} {
					if empty {
						p.Notes = nil
					}
					length, err := EffectiveLengthTicks(p)
					if err != nil || length != 960/(scale+1) {
						t.Fatalf("length = %d, %v", length, err)
					}
					end, err := EndTime(p, bpm)
					want, _ := TickToTime(length, bpm)
					if err != nil || end != want || end <= notes[len(notes)-1].Time {
						t.Fatalf("end = %v, %v", end, err)
					}
					if empty {
						ns, err := Schedule(p, bpm)
						if err != nil || len(ns) != 0 {
							t.Fatalf("empty = %+v, %v", ns, err)
						}
					}
				}
			})
		}
	}
}

func TestTimeScaleValidationAcrossAPIs(t *testing.T) {
	for _, tc := range []struct {
		scale, tick int64
		errText     string
	}{
		{0, 959, ""}, {1, 958, ""}, {1, 959, "divisible"}, {1, 1, "divisible"},
		{0, 960, "tick must"}, {1, 960, "tick must"}, {1, -2, "tick must"},
		{-1, 0, "time_scale"}, {2, 0, "time_scale"}, {math.MaxInt64, 0, "time_scale"},
	} {
		t.Run(fmt.Sprintf("scale%d/tick%d", tc.scale, tc.tick), func(t *testing.T) {
			p := Pattern{LengthTicks: 960, TimeScale: tc.scale, Notes: []Note{{0, tc.tick}}}
			_, scheduleErr := Schedule(p, 180)
			_, lengthErr := EffectiveLengthTicks(p)
			_, endErr := EndTime(p, 180)
			_, loadErr := decode(strings.NewReader(fmt.Sprintf(`{"name":"x","length_ticks":960,"time_scale":%d,"notes":[{"lane":0,"tick":%d}]}`, tc.scale, tc.tick)))
			for _, err := range []error{Validate(p), scheduleErr, lengthErr, endErr, loadErr} {
				if tc.errText == "" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || !strings.Contains(err.Error(), tc.errText) {
					t.Fatalf("error = %v, want %q", err, tc.errText)
				}
			}
		})
	}
	// Compression must retain chord legality and reject repeated lane/tick pairs.
	for _, scale := range []int64{0, 1} {
		p := Pattern{LengthTicks: 960, TimeScale: scale, Notes: []Note{{0, 240}, {0, 240}}}
		if err := Validate(p); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("duplicate error = %v", err)
		}
	}
}

func TestEndTimeConversionErrors(t *testing.T) {
	p := Pattern{LengthTicks: 960}
	for _, bpm := range []float64{0, -1, math.NaN(), math.Inf(1), math.SmallestNonzeroFloat64} {
		if _, err := EndTime(p, bpm); err == nil {
			t.Fatalf("accepted BPM %g", bpm)
		}
		if _, err := Schedule(p, bpm); err == nil {
			t.Fatalf("empty schedule accepted BPM %g", bpm)
		}
	}
	// End overflow must be rejected even when the only note at tick zero fits.
	p.Notes = []Note{{0, 0}}
	if _, err := Schedule(p, 1e-9); err == nil || !strings.Contains(err.Error(), "pattern end") {
		t.Fatalf("end overflow = %v", err)
	}
	end, err := EndTime(Pattern{LengthTicks: 960, TimeScale: 1}, 120)
	if err != nil || end != 250*time.Millisecond {
		t.Fatalf("end = %v, %v", end, err)
	}
}
