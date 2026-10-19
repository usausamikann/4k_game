package audio

import (
	"fmt"
	"strings"
)

// Settings is shared by CLI and future GUI callers. Volumes are percentages,
// never raw device gains. Lane assignment is independent of physical bindings.
type Settings struct {
	KeySoundMode                    string
	KeySound                        string
	KeySoundLanes                   [4]string
	KeySoundVolume, MetronomeVolume int
}

var keySounds = [...]string{"bong_001", "drop_002", "drop_004", "glass_005", "toggle_001"}

func KeySoundIDs() []string { return append([]string(nil), keySounds[:]...) }
func validKeySound(id string) bool {
	for _, s := range keySounds {
		if s == id {
			return true
		}
	}
	return false
}
func DefaultSettings() Settings {
	return Settings{KeySoundMode: "off", KeySound: "drop_002", KeySoundLanes: [4]string{"drop_002", "drop_002", "drop_002", "drop_002"}, KeySoundVolume: 50, MetronomeVolume: 50}
}
func ParseSettings(mode, sound, lanes string, keyVolume, metVolume int) (Settings, error) {
	s := DefaultSettings()
	s.KeySoundMode = mode
	s.KeySound = sound
	s.KeySoundVolume = keyVolume
	s.MetronomeVolume = metVolume
	parts := strings.Split(lanes, ",")
	if len(parts) != 4 {
		return s, fmt.Errorf("keysound-lanes requires exactly 4 sound IDs in lane order 0..3")
	}
	for i, p := range parts {
		s.KeySoundLanes[i] = strings.TrimSpace(p)
	}
	return s, s.Validate()
}
func (s Settings) Validate() error {
	switch s.KeySoundMode {
	case "off", "unified", "per-lane":
	default:
		return fmt.Errorf("keysound-mode must be off, unified, or per-lane: %q", s.KeySoundMode)
	}
	if !validKeySound(s.KeySound) {
		return fmt.Errorf("unknown keysound-sound %q; choose %s", s.KeySound, strings.Join(KeySoundIDs(), ", "))
	}
	for i, id := range s.KeySoundLanes {
		if !validKeySound(id) {
			return fmt.Errorf("unknown keysound-lanes lane %d sound %q; choose %s", i, id, strings.Join(KeySoundIDs(), ", "))
		}
	}
	for _, v := range []struct {
		name  string
		value int
	}{{"keysound-volume", s.KeySoundVolume}, {"metronome-volume", s.MetronomeVolume}} {
		if v.value < 0 || v.value > 100 {
			return fmt.Errorf("%s must be 0..100 percent: %d", v.name, v.value)
		}
	}
	return nil
}
func (s Settings) SoundForLane(lane int) string {
	if lane < 0 || lane >= 4 || s.KeySoundMode == "off" {
		return ""
	}
	if s.KeySoundMode == "unified" {
		return s.KeySound
	}
	return s.KeySoundLanes[lane]
}

// Reserve 0.25 of full scale for the unchanged metronome PCM stream.
func (s Settings) MetronomeGain() float64 { return .25 * float64(s.MetronomeVolume) / 100 }
