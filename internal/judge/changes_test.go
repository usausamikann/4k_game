package judge

import (
	"testing"
	"time"
)

func TestDrainResolvedOnlyAndExactlyOnce(t *testing.T) {
	e := engine(t, note(0, 0, time.Second), note(1, 1, time.Second))
	process(t, e, time.Second, press(0, time.Second))
	buf := e.DrainChanges(nil)
	if len(buf) != 1 || buf[0].Note.ID != 0 || buf[0].State != Hit {
		t.Fatalf("changes %+v", buf)
	}
	buf[0].State = Miss
	if e.Results()[0].State != Hit {
		t.Fatal("changes alias engine")
	}
	if got := e.DrainChanges(buf); len(got) != 0 {
		t.Fatal("change delivered twice")
	}
	process(t, e, time.Second+91*time.Millisecond)
	buf = e.DrainChanges(buf)
	if len(buf) != 1 || buf[0].Note.ID != 1 || buf[0].State != Miss {
		t.Fatalf("MISS diff %+v", buf)
	}
}
