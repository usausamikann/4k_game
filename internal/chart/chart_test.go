package chart

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadSample(t *testing.T) {
	p, err := Load("../../patterns/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	want := Pattern{Name: "sample", LengthTicks: 960, Notes: []Note{{0, 0}, {1, 240}, {2, 480}, {3, 720}}}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("pattern = %#v, want %#v", p, want)
	}
	notes, err := Schedule(p, 180)
	if err != nil {
		t.Fatal(err)
	}
	wantTimes := []time.Duration{0, 83333333, 166666667, 250000000}
	for i, n := range notes {
		want := ScheduledNote{ID: int64(i), Lane: p.Notes[i].Lane, AbsoluteTick: p.Notes[i].Tick, Time: wantTimes[i]}
		if n != want {
			t.Fatalf("note %d = %+v, want %+v", i, n, want)
		}
	}
}

func TestDecodeValidation(t *testing.T) {
	tests := []struct{ name, input, errText string }{
		{"simultaneous", `{"name":"chord","length_ticks":960,"notes":[{"lane":3,"tick":480},{"lane":0,"tick":480}]}`, ""},
		{"empty", `{"name":"empty","length_ticks":960,"notes":[]}`, ""},
		{"last tick", `{"name":"last","length_ticks":960,"notes":[{"lane":3,"tick":959}]}`, ""},
		{"duplicate", `{"name":"x","length_ticks":960,"notes":[{"lane":0,"tick":0},{"lane":1,"tick":480},{"lane":0,"tick":0}]}`, "duplicate note"},
		{"negative lane", `{"name":"x","length_ticks":960,"notes":[{"lane":-1,"tick":0}]}`, "lane must"},
		{"large lane", `{"name":"x","length_ticks":960,"notes":[{"lane":4,"tick":0}]}`, "lane must"},
		{"negative tick", `{"name":"x","length_ticks":960,"notes":[{"lane":0,"tick":-1}]}`, "tick must"},
		{"end tick", `{"name":"x","length_ticks":960,"notes":[{"lane":0,"tick":960}]}`, "tick must"},
		{"wrong length", `{"name":"x","length_ticks":1920,"notes":[]}`, "length_ticks must"},
		{"zero length", `{"name":"x","length_ticks":0,"notes":[]}`, "length_ticks must"},
		{"malformed", `{`, "decode JSON"},
		{"fractional tick", `{"name":"x","length_ticks":960,"notes":[{"lane":0,"tick":0.5}]}`, "decode JSON"},
		{"overflow tick", `{"name":"x","length_ticks":960,"notes":[{"lane":0,"tick":9223372036854775808}]}`, "decode JSON"},
		{"missing tick", `{"name":"x","length_ticks":960,"notes":[{"lane":0}]}`, "lane and tick are required"},
		{"missing lane", `{"name":"x","length_ticks":960,"notes":[{"tick":0}]}`, "lane and tick are required"},
		{"null note", `{"name":"x","length_ticks":960,"notes":[null]}`, "lane and tick are required"},
		{"missing field", `{"name":"x","notes":[]}`, "are required"},
		{"null notes", `{"name":"x","length_ticks":960,"notes":null}`, "are required"},
		{"null pattern", `null`, "are required"},
		{"unknown field", `{"name":"x","length_ticks":960,"notes":[],"typo":0}`, "unknown field"},
		{"second object", `{"name":"x","length_ticks":960,"notes":[]} {}`, "exactly one"},
		{"trailing garbage", `{"name":"x","length_ticks":960,"notes":[]} broken`, "trailing JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := decode(strings.NewReader(tt.input))
			if tt.errText != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errText) {
					t.Fatalf("error = %v, want %q", err, tt.errText)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.name == "simultaneous" && !reflect.DeepEqual(p.Notes, []Note{{0, 480}, {3, 480}}) {
				t.Fatalf("notes not sorted: %+v", p.Notes)
			}
		})
	}
}

func TestLoadFileErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil || !strings.Contains(err.Error(), "open pattern") {
		t.Fatalf("missing file error: %v", err)
	}
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"name":"x","length_ticks":1,"notes":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "length_ticks") {
		t.Fatalf("invalid file error: %v", err)
	}
}

func TestTickToTime(t *testing.T) {
	tests := []struct {
		name string
		tick int64
		bpm  float64
		want time.Duration
	}{
		{"origin", 0, 120, 0},
		{"120 eighth", 480, 120, 250 * time.Millisecond},
		{"120 beat", 960, 120, 500 * time.Millisecond},
		{"120 bar", 3840, 120, 2 * time.Second},
		{"180 eighth", 480, 180, 166666667},
		{"180 beat", 960, 180, 333333333},
		{"180 bar", 3840, 180, 1333333333},
		{"half rounds up", 1, 4000000, 16},
		{"below half", 1, 8000000, 8},
		{"fractional BPM", 960, 120.5, 497925311},
		{"long chart", 1000000000000, 120, 520833333333333333},
		{"max duration exactly", math.MaxInt64, 62500000, time.Duration(math.MaxInt64)},
		{"tiny result", 1, math.MaxFloat64, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TickToTime(tt.tick, tt.bpm)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("time = %d ns, want %d ns", got, tt.want)
			}
		})
	}
	// A rounded 1-tick duration multiplied by 960 would be wrong at BPM 180.
	one, _ := TickToTime(1, 180)
	beat, _ := TickToTime(960, 180)
	if beat == one*960 {
		t.Fatal("conversion accumulated rounded tick duration")
	}
}

func TestInvalidConversion(t *testing.T) {
	for _, bpm := range []float64{0, -120, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := TickToTime(960, bpm); err == nil {
			t.Fatalf("accepted BPM %g", bpm)
		}
		if _, err := Schedule(Pattern{LengthTicks: 960}, bpm); err == nil {
			t.Fatalf("empty pattern accepted BPM %g", bpm)
		}
	}
	if _, err := TickToTime(-1, 120); err == nil {
		t.Fatal("accepted negative tick")
	}
	for _, tc := range []struct {
		tick int64
		bpm  float64
	}{{math.MaxInt64, 120}, {1, math.SmallestNonzeroFloat64}} {
		if _, err := TickToTime(tc.tick, tc.bpm); err == nil || !strings.Contains(err.Error(), "range") {
			t.Fatalf("expected duration overflow, got %v", err)
		}
	}
	if _, err := Schedule(Pattern{LengthTicks: 960, Notes: []Note{{0, 1}}}, math.SmallestNonzeroFloat64); err == nil {
		t.Fatal("schedule accepted duration overflow")
	}
}

func TestScheduleDeterminismAndIsolation(t *testing.T) {
	p := Pattern{Name: "chord", LengthTicks: 960, Notes: []Note{{3, 480}, {2, 0}, {0, 480}}}
	original := append([]Note(nil), p.Notes...)
	got, err := Schedule(p, 120)
	if err != nil {
		t.Fatal(err)
	}
	want := []ScheduledNote{{0, 2, 0, 0}, {1, 0, 480, 250 * time.Millisecond}, {2, 3, 480, 250 * time.Millisecond}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schedule = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(p.Notes, original) {
		t.Fatal("modified original pattern")
	}
	reordered := Pattern{Name: p.Name, LengthTicks: 960, Notes: []Note{{0, 480}, {3, 480}, {2, 0}}}
	again, err := Schedule(reordered, 120)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("order-dependent schedule: %+v, %v", again, err)
	}
	got[0].Lane = 0
	if !reflect.DeepEqual(p.Notes, original) {
		t.Fatal("scheduled notes alias original notes")
	}
	empty, err := Schedule(Pattern{LengthTicks: 960}, 120)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty schedule = %+v, %v", empty, err)
	}
}

func TestDirectPatternValidation(t *testing.T) {
	for _, p := range []Pattern{
		{LengthTicks: 0},
		{LengthTicks: 960, Notes: []Note{{4, 0}}},
		{LengthTicks: 960, Notes: []Note{{0, 960}}},
		{LengthTicks: 960, Notes: []Note{{0, 0}, {0, 0}}},
	} {
		if err := Validate(p); err == nil {
			t.Fatalf("accepted invalid pattern: %+v", p)
		}
		if _, err := Schedule(p, 120); err == nil {
			t.Fatalf("scheduled invalid pattern: %+v", p)
		}
	}
	if err := Validate(Pattern{LengthTicks: 960, Notes: []Note{{0, 0}, {1, 0}}}); err != nil {
		t.Fatal(err)
	}
}
