# Step 5 metronome candidate assets

Source: Kenney, [Interface Sounds](https://kenney.nl/assets/interface-sounds) — CC0 1.0. Attribution is not required, but credit to Kenney is welcome. Please retain the original upstream license/readme with the downloaded pack where convenient.

The five original Ogg Vorbis files committed for Step 5 are:

- `switch8.ogg`
- `switch19.ogg`
- `switch29.ogg`
- `glass_002.ogg`
- `pluck_001.ogg`

All five were selected by the user. Current subjective favorites: `glass_002` and `pluck_001`; `switch8`, `switch19`, and `switch29` must remain available, and any default is provisional.

Filenames, not embedded OGG `TITLE` tags, identify a sound. Some upstream files contain mismatched tags.

All five binaries were added in PR #9 and remain unchanged. `assets.go` embeds them in the executable. Step 5 decodes the selected file once, resamples to 48kHz stereo, and reproducibly trims the lead-in in memory; it never overwrites the original OGG. See the root README for sound selection and measured trim offsets.

License reference: https://creativecommons.org/publicdomain/zero/1.0/ . Filenames and explicit sound IDs are authoritative; embedded TITLE tags are not used.

See `memo/4k_game_step5_metronome_handoff_20261018.md` for the actual Codex implementation requirements. No downbeat accent and no keysounds in Step 5.
