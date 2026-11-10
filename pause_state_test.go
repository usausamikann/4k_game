package main

import "testing"

func TestPauseStateEscapeEdgesAndFocus(t *testing.T) {
	var p pauseState
	p.Update(true, false)
	if p.active {
		t.Fatal("starts unpaused")
	}
	p.Update(true, true)
	if !p.active {
		t.Fatal("first Escape must pause")
	}
	p.Update(true, true)
	if !p.active {
		t.Fatal("held Escape must not toggle")
	}
	p.Update(true, false)
	p.Update(true, true)
	if p.active {
		t.Fatal("second fresh Escape must resume")
	}
	p.Update(false, true)
	if !p.active {
		t.Fatal("focus loss must pause")
	}
	p.Update(true, true)
	if !p.active {
		t.Fatal("held Escape on refocus must not resume")
	}
	p.Update(true, false)
	if !p.active {
		t.Fatal("releasing Escape on refocus must not resume")
	}
	p.Update(true, true)
	if p.active {
		t.Fatal("fresh Escape after refocus must resume")
	}
}

func TestPauseStateFocusLossBeforeFirstPress(t *testing.T) {
	var p pauseState
	p.Update(false, false)
	p.Update(true, false)
	if !p.active {
		t.Fatal("refocus must stay paused")
	}
	p.Update(true, true)
	if p.active {
		t.Fatal("Escape must resume after release")
	}
}
