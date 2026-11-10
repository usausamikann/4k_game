package main

// pauseState owns the Escape key edge and the persistent pause state.
// Losing focus always enters Pause; regaining focus never resumes implicitly.
// Escape held across a focus change must be released before it can resume.
type pauseState struct {
	active        bool
	escapeWasDown bool
	escapeBlocked bool
	everFocused   bool
}

func (p *pauseState) Update(focused, escapeDown bool) {
	if !focused {
		if p.everFocused {
			p.active = true
		}
		p.escapeBlocked = true
		p.escapeWasDown = escapeDown
		return
	}
	p.everFocused = true
	if p.escapeBlocked {
		if !escapeDown {
			p.escapeBlocked = false
		}
	} else if escapeDown && !p.escapeWasDown {
		p.active = !p.active
	}
	p.escapeWasDown = escapeDown
}
