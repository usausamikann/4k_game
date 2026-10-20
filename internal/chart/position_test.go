package chart

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestFractionPositions(t *testing.T) {
	for _, scale := range []int64{0, 1} {
		// Include reducible fractions and int64-sized fractions whose intermediate
		// numerator product (and scaled denominator) would overflow int64.
		for _, tc := range []struct {
			position string
			tick     int64
		}{
			{`[0,1]`, 0}, {`[1,4]`, 240}, {`[1,6]`, 160}, {`[1,8]`, 120},
			{`[1,3]`, 320}, {`[2,3]`, 640}, {`[2,4]`, 480}, {`[3,4]`, 720},
			{`[1,5]`, 192}, {`[1,2]`, 480}, {`[4,8]`, 480},
			{`[0,9223372036854775807]`, 0},
			{`[4611686018427387903,9223372036854775806]`, 480},
		} {
			t.Run(fmt.Sprintf("scale%d/%s", scale, tc.position), func(t *testing.T) {
				input := fmt.Sprintf(`{"name":"fraction","time_scale":%d,"notes":[{"lane":2,"position":%s}]}`, scale, tc.position)
				p, err := decode(strings.NewReader(input))
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(p.Notes, []Note{{Lane: 2, Tick: tc.tick}}) {
					t.Fatalf("original notes = %+v", p.Notes)
				}
				original := append([]Note(nil), p.Notes...)
				scheduled, err := Schedule(p, 180)
				if err != nil {
					t.Fatal(err)
				}
				at, err := TickToTime(tc.tick/(scale+1), 180)
				if err != nil {
					t.Fatal(err)
				}
				if len(scheduled) != 1 || scheduled[0].AbsoluteTick != tc.tick/(scale+1) || scheduled[0].Time != at {
					t.Fatalf("schedule = %+v", scheduled)
				}
				if !reflect.DeepEqual(original, p.Notes) {
					t.Fatal("Schedule changed original ticks")
				}
			})
		}
	}
	// Every position in 16th/24th/32nd grids, not just the first interval.
	for _, denominator := range []int64{4, 6, 8} {
		for numerator := int64(0); numerator < denominator; numerator++ {
			tick, err := positionToTick(numerator, denominator)
			if err != nil || tick != 960*numerator/denominator {
				t.Fatalf("%d/%d = %d, %v", numerator, denominator, tick, err)
			}
		}
	}
}

func TestInvalidFractionPositions(t *testing.T) {
	for _, tc := range []struct {
		position, errText string
		scale             int64
	}{
		{`[1,0]`, "denominator must be positive", 0},
		{`[-1,4]`, "0 <= numerator < denominator", 0},
		{`[1,-4]`, "denominator must be positive", 0},
		{`[4,4]`, "0 <= numerator < denominator", 0},
		{`[5,4]`, "0 <= numerator < denominator", 0},
		{`[1,7]`, "integer original tick", 0},
		{`[1,7]`, "integer original tick", 1},
		{`[1,64]`, "divisible", 1},
		{`[959,960]`, "divisible", 1},
		{`[9223372036854775806,9223372036854775807]`, "integer original tick", 0},
		{`[9223372036854775807,9223372036854775807]`, "0 <= numerator < denominator", 0},
		{`[1,-9223372036854775808]`, "denominator must be positive", 0},
		{`[-9223372036854775808,4]`, "0 <= numerator < denominator", 0},
		{`null`, "required", 0}, {`[]`, "exactly two", 0}, {`[1]`, "exactly two", 0},
		{`[0,1,2]`, "exactly two", 0}, {`[0,1,null]`, "exactly two", 0},
		{`[null,4]`, "must not be null", 0}, {`[0,null]`, "must not be null", 0},
		{`[0.5,4]`, "decode JSON", 0}, {`[1,4.0]`, "decode JSON", 0}, {`[1e0,4]`, "decode JSON", 0},
		{`["1",4]`, "decode JSON", 0}, {`[1,true]`, "decode JSON", 0},
		{`[1,{}]`, "decode JSON", 0}, {`[[],4]`, "decode JSON", 0},
		{`"1/4"`, "decode JSON", 0}, {`0.25`, "decode JSON", 0}, {`{}`, "decode JSON", 0},
		{`[9223372036854775808,4]`, "decode JSON", 0},
		{`[1,9223372036854775808]`, "decode JSON", 0},
		{`[-9223372036854775809,4]`, "decode JSON", 0},
	} {
		t.Run(fmt.Sprintf("scale%d/%s", tc.scale, tc.position), func(t *testing.T) {
			input := fmt.Sprintf(`{"name":"x","time_scale":%d,"notes":[{"lane":0,"position":%s}]}`, tc.scale, tc.position)
			_, err := decode(strings.NewReader(input))
			if err == nil || !strings.Contains(err.Error(), tc.errText) {
				t.Fatalf("error = %v, want %q", err, tc.errText)
			}
		})
	}
	// Odd original ticks remain valid when no compression is requested.
	p, err := decode(strings.NewReader(`{"name":"odd","notes":[{"lane":0,"position":[1,64]},{"lane":1,"position":[959,960]}]}`))
	if err != nil || !reflect.DeepEqual(p.Notes, []Note{{0, 15}, {1, 959}}) {
		t.Fatalf("odd original ticks = %+v, %v", p.Notes, err)
	}
}

func TestFractionDuplicatesAndChords(t *testing.T) {
	for _, scale := range []int64{0, 1} {
		input := fmt.Sprintf(`{"name":"chord","time_scale":%d,"notes":[{"lane":3,"position":[1,2]},{"lane":0,"position":[2,4]},{"lane":1,"position":[1,4]}]}`, scale)
		p, err := decode(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p.Notes, []Note{{1, 240}, {0, 480}, {3, 480}}) {
			t.Fatalf("sorted chord = %+v", p.Notes)
		}
		for _, position := range []string{`[2,4]`, `[4,8]`} {
			input := fmt.Sprintf(`{"name":"duplicate","time_scale":%d,"notes":[{"lane":0,"position":[1,2]},{"lane":0,"position":%s}]}`, scale, position)
			if _, err := decode(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "duplicate note") {
				t.Fatalf("duplicate error = %v", err)
			}
		}
	}
}

func TestRetiredJSONFields(t *testing.T) {
	for _, input := range []string{
		`{"name":"x","length_ticks":960,"notes":[]}`,
		`{"name":"x","length_ticks":null,"notes":[]}`,
		`{"name":"x","notes":[{"lane":0,"tick":0}]}`,
		`{"name":"x","notes":[{"lane":0,"tick":0,"position":[0,1]}]}`,
	} {
		if _, err := decode(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("retired field error = %v", err)
		}
	}
}
