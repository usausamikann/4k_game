# Input edge / Windows Raw Input integration plan (2026-11-20)

This branch is a **draft** for a safer implementation of the Gemini 4 Neon proposal.
The existing judgement engine, midpoint boundaries and ±30/60/90 ms windows are
out of scope. Preserve `UpdateAt` and deterministic tests.

## Proposed implementation
1. Extend `internal/input.Tracker` with edge-aware sampling using Ebitengine
   v2.10.5 `IsKeyJustPressed`, `IsKeyJustReleased`, and `KeyPressDuration`.
   Handle one short tap and release/re-press in a tick. Do not claim recovery
   of arbitrarily many transitions within a tick.
2. Provide a Windows-only `WM_INPUT` receiver on a locked OS thread, timestamping
   message receipt and forwarding ordered Make/Break events to the existing Clock,
   Offsets and judge Engine. The reported time is *not* a hardware timestamp.
3. Keep the non-Windows and opt-out fallback. Preserve pause/focus handling
   and do not judge keys held while the window is inactive.
4. Validate Windows API return values correctly: `GetRawInputData` returns
   `UINT(-1)` / `0xFFFFFFFF` on error, not 64-bit `^uintptr(0)`.
   Count API failures and malformed packets instead of silently discarding.
5. On raw queue overflow, report the number of lost events and recover the
   Tracker/judge key state by emitting releases and blocking currently held
   keys until physical release. Never process a truncated event batch as normal.
6. On focus regain, do not unconditionally discard the first batch of new events;
   distinguish previously held keys from new presses where observable.
7. Avoid assuming US punctuation scancodes on JIS keyboards. Initially allow
   raw scancodes for A-Z only and fall back to Ebitengine for punctuation;
   investigate layout-aware mapping separately before enabling it.
8. Keep Raw Input *opt-in* during the experiment (`-raw-input=false` by default),
   and enable it by default only after native Windows validation.

## Tests and validation gates
- Existing `go test ./...`, old `UpdateAt`, and judge tests stay green.
- Unit tests: short press/release; release-press within a tick; raw auto-repeat;
  same-key and cross-key event ordering; future event deferral; focus loss/regain;
  raw queue overflow and resynchronization; release from a held key after resync.
- Integration tests: two notes hit in one update from distinct historical key
  events; independence from how events are batched; pause/resume and offsets.
- Windows compilation and native hardware validation with `D,F,J,K` first;
  separate JIS punctuation fallback smoke tests.
- Compare `-raw-input=true` vs `false`, 60/120/240 TPS, real detected presses,
  empty presses, judge offsets, CPU cost and timing jitter. `EmptyPresses` alone
  is not sufficient to distinguish hardware rollover from software input loss.

## Merge condition
No merge until compilation, tests, and native Windows keyboard/focus checks
are complete. This file documents the design first; implementation follows
on the same branch when validated.