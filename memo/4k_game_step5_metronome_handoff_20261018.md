# Codex handoff — Step 5: metronome audio and synchronization (2026-10-18)

## Scope / current status
Implement **Step 5 only** for `usausamikann/4k_game`, starting from the already merged Step 4.5. Read `memo/4k_game_implementation_plan_20261012.txt`, `memo/4k_game_step45_input_measurement_and_future_event_design_20261013.txt`, `README.md`, `internal/play/timeline.go`, `internal/play/clock.go`, and `internal/play/session.go` first. Do **not** rewrite older memo files. This new handoff supersedes any suggestion that a first-beat / bar accent is necessary.

The user reports that Windows-native Step 4.5 runs at 60/120/240 TPS, including BPM220 16th-note repetitions, with no obvious problems. CPU load was assessed informally only. Keep `-tps` default 240, existing judgement, input, scroll speed, A/B, SUDDEN+ 0.0–40.0, and focus pause/resume behavior intact.

## 1. Audio choices — all five candidates, not only glass/pluck
All five are Kenney *Interface Sounds* CC0 Ogg Vorbis files (user-selected candidates):
- `switch8.ogg`
- `switch19.ogg`
- `switch29.ogg`
- `glass_002.ogg`
- `pluck_001.ogg`

The user's latest preference is **glass_002** or **pluck_001**, but the **switch8, switch19, and switch29** candidates must remain available. Do not omit the switch series. Use `glass_002` as a **provisional default**, not a final artistic decision; a launch option such as `-metronome-sound glass_002` should allow selecting any of the five without editing code. An in-game selection UI is **not** required in Step 5. Document the supported values.

Important: some original OGG `TITLE` tags disagree with the on-disk filenames. Treat file path / explicit sound ID as authoritative; never use tags to choose the sound. Preserve original OGG assets rather than destructively renaming/modifying them. Support both mono and stereo candidates; all originals are 44.1 kHz. Convert/normalize as needed to one well-defined PCM output format during initialization. Kenney source: https://kenney.nl/assets/interface-sounds ; include a small attribution/license note (CC0; credit optional).

**No strong-beat accent.** The user explicitly wants a flat quarter-note metronome, without a special sound/volume/pitch on the first beat of each measure. The random pattern design should support practicing rhythms independent of a bar accent. Do not assign musical accents by `tick % (4 * TicksPerBeat)`. Avoid accidental gain emphasis at the start of PLAY.

**No keysounds in Step 5.** Eventually there may be keysound modes `Off / Unified / Per-lane`, independent of the metronome, with sounds triggered by actual Press events. Leave a clean separation in audio code so these can be added as Step 5.5, but do not add these audio effects, UI, or extra playback dependencies now.

## 2. Exact click schedule
Current timing: four COUNT beats, four SILENT beats, then PLAY at chart time 0. `internal/play.Timeline` already provides these times. At BPM180 the eight-beat preparation lasts ~2.667 s.

- COUNT: identical click on each of its four beats.
- SILENT: no clicks during the next four beats.
- PLAY: identical click every quarter note, starting **at chart time 0**, including across note-pattern boundaries; no per-measure accent.
- Existing A/B timing adjustments affect drawing and/or judgement per current definitions, not the audio beat timeline. Do not silently change their semantics.
- A click sound may overlap the following beat only if its waveform physically does so; avoid artificial cutoff/restart of the global audio stream. In practice assets are short.
- Prevent clipping when mixing clicks or when converting/resampling audio, without needlessly flattening dynamics.

## 3. Timing and architecture requirements
- Compute each desired beat onset from an **absolute beat index** and BPM. For target sample rate `sampleRate`, use `sampleIndex(n) = round(n * 60 * sampleRate / BPM)` (with a documented rounding convention). Use an overflow-safe strategy for long sessions; do **not** advance by repeatedly adding a rounded 'samples per beat' interval.
- For the eight-beat countdown, choose a single precise mapping between stream sample zero and chart time `-Preparation`, so COUNT/SILENT/PLAY placements all follow the same origin. The PLAY click at chart time 0 should map to the eighth-beat sample index from stream start.
- Either a bounded, pull-generated PCM reader, or another bounded-memory way to mix clicks into a continuous stream is preferred. Avoid preallocating whole multi-minute silent PCM buffers. Do not use `time.Ticker` or per-Update `Player.Play()` calls to schedule individual beats.
- Decoding each OGG is permitted **once at startup**, with sensible output sample-rate handling. Avoid per-beat decoder/player allocation. Read audio from a stable asset path or embed it in the executable; document relative-path handling if external files remain necessary.
- Some `switch` files contain a noticeable low-amplitude/near-silent lead-in before the transient (~100ms or more). Measure/alignment-check each original and align the perceived click attack to the scheduled beat. Prefer a documented non-destructive per-asset onset offset or a reproducible trim at initialization. Do not assume file sample 0 is the audible onset. Avoid accidental removal of attack transients.
- Ebitengine v2.10.5 is fixed by go.mod. Use its supported audio context / player APIs. Keep `internal/audio` separate from judgement/visual code.
- Explicitly map audio playback position and the existing monotonic game Clock, including startup offset and backend buffer latency. `Player.Position()` is useful for diagnostics but is not guaranteed to equal the physical acoustic output time. Do not chase it via seek every frame. Make start behavior stable and investigate measurable drift rather than claiming perfect audio latency.

**Clarification on cumulative drift:** with absolute PCM sample placement, beat-position *calculation* does NOT accumulate rounding error. The remaining relevant concerns are independent audio-device vs system-clock rate differences, startup/output buffer latency, glitches, pause/resume, and imperfect event-time mapping. Preserve this distinction in docs and test reporting.

## 4. Focus, stability, performance
- On loss of focus, stop/pause **both** game Clock and metronome; while unfocused, audio must not run ahead.
- On return, resume them consistently without restarting COUNT from the beginning or emitting duplicate clicks. Existing held-key behavior must remain unchanged.
- Keep memory and CPU requirements modest: bounded buffers, no per-frame PCM array churn; maintain current TPS/FPS and simple HUD unless minimally needed for diagnostics.
- No musical backing tracks, real-time BPM changes, waveform-editor UI, rhythm-pattern generation, results screen, or controls unrelated to Step 5.

## 5. Tests and verification
Add headless tests for:
- Exact sample positions at varied BPM (e.g. 120, 180, 220, and awkward decimal BPM), including after minutes of beats; each beat placement differs from its absolute mathematical ideal by at most the stated rounding bound.
- COUNT 4 clicks -> SILENT 4 beats -> PLAY clicks from chart t=0, all uniform volume/timbre regardless of bar index.
- Decode/resample/playback preparation with each of the five candidate OGGS (mono and stereo), safe amplitude and click-onset alignment; erroneous/missing files produce useful errors.
- Audio/Clock mapping at start and after pause/resume without duplicate or lost scheduled beats.
- No regression in judge, input, A/B offsets, SUDDEN+, speed, focus handling, and Step 4.5 TPS settings.

Run `go test ./...`, `go vet ./...`, any practical graphics/audio-free unit tests, and Windows amd64 build. Update README with sound selection, whether assets are embedded, and how to run a test. Explicitly identify items unverified by cloud tests: Windows speaker/headphone latency, stable synchronization, perceived comfort at BPM180–220, sound output, and actual CPU usage. Do not claim physical device latency is automatically measured by `Player.Position()`.

## 6. Windows manual acceptance
- All five candidates can be selected and each is actually audible.
- Four clicks -> four beats silence -> identical quarter-note clicks in PLAY; **never a downbeat accent**.
- `-speed 480` (or other speed) changes note scroll only, not audio beat timings.
- Changing A/B and switching TPS does not silently change audio BPM.
- Alt+Tab pauses/resumes without persistent phase shift or double clicks.
- BPM180 and BPM220 listening tests show consistent clicks and acceptable CPU usage. Report any observable audio/video/input offset, not unsupported precision claims.

## Asset handoff / PR sequencing
This PR is a **handoff specification**, not a completed Step 5 implementation. The original five OGG files are user-provided chat attachments but are **not yet committed** to the GitHub branch. Before Codex implements Step 5, add them to `assets/audio/` on this branch, preserving names. Then ask Codex to implement the above requirements in a separate code PR (or on a dedicated implementation branch). Do not merge this specification PR thinking Step 5 has been implemented.
