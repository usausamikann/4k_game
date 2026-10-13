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
		return keys, fmt.Errorf("keys must contain four distinct A-Z letters")
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
