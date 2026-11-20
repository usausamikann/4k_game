package main

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/usausamikann/4k_game/internal/chart"
)

func defaultBindings() [chart.LaneCount]ebiten.Key {
	return [chart.LaneCount]ebiten.Key{ebiten.KeyD, ebiten.KeyF, ebiten.KeyJ, ebiten.KeyK}
}
func parseBindings(text string) ([chart.LaneCount]ebiten.Key, error) {
	var keys [chart.LaneCount]ebiten.Key
	names := strings.Split(strings.ToUpper(text), ",")
	if len(names) != chart.LaneCount {
		return keys, fmt.Errorf("keys must contain four distinct supported keys (A-Z, ;, ])")
	}
	available := map[string]ebiten.Key{
		"A": ebiten.KeyA,
		"B": ebiten.KeyB,
		"C": ebiten.KeyC,
		"D": ebiten.KeyD,
		"E": ebiten.KeyE,
		"F": ebiten.KeyF,
		"G": ebiten.KeyG,
		"H": ebiten.KeyH,
		"I": ebiten.KeyI,
		"J": ebiten.KeyJ,
		"K": ebiten.KeyK,
		"L": ebiten.KeyL,
		"M": ebiten.KeyM,
		"N": ebiten.KeyN,
		"O": ebiten.KeyO,
		"P": ebiten.KeyP,
		"Q": ebiten.KeyQ,
		"R": ebiten.KeyR,
		"S": ebiten.KeyS,
		"T": ebiten.KeyT,
		"U": ebiten.KeyU,
		"V": ebiten.KeyV,
		"W": ebiten.KeyW,
		"X": ebiten.KeyX,
		"Y": ebiten.KeyY,
		"Z": ebiten.KeyZ,
	}
	// Punctuation is supported alongside the existing A-Z bindings.
	// The symbolic aliases avoid confusing key names in command-line flags.
	available[";"] = ebiten.KeySemicolon
	available["SEMICOLON"] = ebiten.KeySemicolon
	available["]"] = ebiten.KeyBracketRight
	available["BRACKETRIGHT"] = ebiten.KeyBracketRight
	used := map[ebiten.Key]bool{}
	for lane, name := range names {
		key, ok := available[strings.TrimSpace(name)]
		if !ok || used[key] {
			return keys, fmt.Errorf("invalid or duplicate lane key %q", name)
		}
		keys[lane] = key
		used[key] = true
	}
	return keys, nil
}

// Standard US physical letter-position scancodes. Non-letter keys are
// intentionally excluded: JIS punctuation positions can differ.
var keyScanCodes = map[ebiten.Key]uint16{
	ebiten.KeyA: 0x1E,
	ebiten.KeyB: 0x30,
	ebiten.KeyC: 0x2E,
	ebiten.KeyD: 0x20,
	ebiten.KeyE: 0x12,
	ebiten.KeyF: 0x21,
	ebiten.KeyG: 0x22,
	ebiten.KeyH: 0x23,
	ebiten.KeyI: 0x17,
	ebiten.KeyJ: 0x24,
	ebiten.KeyK: 0x25,
	ebiten.KeyL: 0x26,
	ebiten.KeyM: 0x32,
	ebiten.KeyN: 0x31,
	ebiten.KeyO: 0x18,
	ebiten.KeyP: 0x19,
	ebiten.KeyQ: 0x10,
	ebiten.KeyR: 0x13,
	ebiten.KeyS: 0x1F,
	ebiten.KeyT: 0x14,
	ebiten.KeyU: 0x16,
	ebiten.KeyV: 0x2F,
	ebiten.KeyW: 0x11,
	ebiten.KeyX: 0x2D,
	ebiten.KeyY: 0x15,
	ebiten.KeyZ: 0x2C,
}

func scanCodesForBindings(keys [chart.LaneCount]ebiten.Key) ([chart.LaneCount]uint16, bool) {
	var codes [chart.LaneCount]uint16
	for lane, key := range keys {
		code, ok := keyScanCodes[key]
		if !ok {
			return codes, false
		}
		codes[lane] = code
	}
	return codes, true
}
