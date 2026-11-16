package chart

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMultiBeatLoadAndSchedule(t *testing.T) {
	input := `{"name":"two-beats","length_beats":2,"time_scale":0,"notes":[
		{"lane":0,"position":[0,4]},
		{"lane":1,"position":[4,4]},
		{"lane":2,"position":[7,4]},
		{"lane":3,"position":[5,4]}
	]}`
	p, err := decode(strings.NewReader(input))
	if err != nil { t.Fatal(err) }
	if p.LengthBeats != 2 { t.Fatalf("length_beats=%d",p.LengthBeats) }
	want := []Note{{0,0},{1,960},{3,1200},{2,1680}}
	if !reflect.DeepEqual(p.Notes,want) { t.Fatalf("notes=%v want=%v",p.Notes,want) }
	length, err := EffectiveLengthTicks(p)
	if err != nil || length != 1920 { t.Fatalf("length=%d %v",length,err) }
	end, err := EndTime(p,120)
	if err != nil || end != time.Second { t.Fatalf("end=%v %v",end,err) }
	notes, err := Schedule(p,120)
	if err != nil || len(notes) != 4 { t.Fatalf("schedule=%v %v",notes,err) }
	for i,n := range notes {
		if n.ID != int64(i) || n.AbsoluteTick != want[i].Tick || n.Lane != want[i].Lane {
			t.Fatalf("note[%d]=%+v want=%+v",i,n,want[i])
		}
		at,e := TickToTime(want[i].Tick,120)
		if e!=nil || n.Time!=at { t.Fatalf("time note[%d]=%v want=%v",i,n.Time,at) }
	}
}

func TestMultiBeatLegacyAndInvalidLengths(t *testing.T) {
	legacy := `{"name":"old","notes":[{"lane":1,"position":[3,4]}]}`
	p, err := decode(strings.NewReader(legacy))
	if err!=nil || p.LengthBeats!=0 { t.Fatalf("legacy: %+v %v",p,err) }
	if n,e := EffectiveLengthTicks(p); e!=nil || n!=960 { t.Fatalf("legacy length=%d %v",n,e) }
	tests:=[]struct{input, want string}{
		{`{"name":"x","length_beats":0,"notes":[]}`,"length_beats"},
		{`{"name":"x","length_beats":null,"notes":[]}`,"must not be null"},
		{`{"name":"x","length_beats":-2,"notes":[]}`,"length_beats"},
		{`{"name":"x","length_beats":1000001,"notes":[]}`,"length_beats"},
		{`{"name":"x","length_beats":1.5,"notes":[]}`,"decode length_beats"},
		{`{"name":"x","length_beats":2,"notes":[{"lane":0,"position":[8,4]}]}`,"length_beats"},
		{`{"name":"x","length_beats":2,"notes":[{"lane":0,"position":[-1,4]}]}`,"length_beats"},
		{`{"name":"x","notes":[{"lane":0,"position":[4,4]}]}`,"numerator < denominator"},
	}
	for _,tt:=range tests {
		_,e:=decode(strings.NewReader(tt.input))
		if e==nil || !strings.Contains(e.Error(),tt.want) { t.Errorf("input %s: got %v, want %q",tt.input,e,tt.want) }
	}
	for _,p := range []Pattern{
		{LengthBeats:-1}, {LengthBeats:1000001},
		{LengthBeats:2,Notes:[]Note{{Lane:0,Tick:1920}}},
	} {
		if err := Validate(p);err==nil { t.Errorf("invalid direct pattern accepted: %+v",p) }
	}
}

func TestMultiBeatTimeScaleAndDuplicates(t *testing.T) {
	p,err:=decode(strings.NewReader(`{"name":"scaled","time_scale":1,"length_beats":3,
	"notes":[{"lane":0,"position":[4,4]},{"lane":1,"position":[11,4]}]}`))
	if err!=nil {t.Fatal(err)}
	if l,e:=EffectiveLengthTicks(p);e!=nil||l!=1440 {t.Fatalf("scaled length %d %v",l,e)}
	notes,e:=Schedule(p,120)
	if e!=nil||len(notes)!=2||notes[0].AbsoluteTick!=480||notes[1].AbsoluteTick!=1320 {
		t.Fatalf("scaled schedule: %v %v",notes,e)
	}
	_,err=decode(strings.NewReader(`{"name":"dupe","length_beats":2,
	"notes":[{"lane":0,"position":[4,4]},{"lane":0,"position":[1,1]}]}`))
	if err==nil||!strings.Contains(err.Error(),"duplicate") {t.Fatalf("dupe error %v",err)}
}
