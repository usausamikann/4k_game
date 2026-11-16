/* 4K Game chord labeler: sample 128 mirrored-equivalence classes.
 * Four 16th-note slots, each with at least one active lane, at least one chord.
 * Each slot is a 4-bit lane mask (D=1,F=2,J=4,K=8).
 * All 15^4 patterns, mirror-fixed configurations included by Burnside:
 * (15^4+3^4)/2 - 4^4/2 = 25,225 mirror classes containing chords.
 * Browser: window.ChordPatternGenerator; Node: require("./generator.js")
 */
(function(root) {
  "use strict";

  const SLOTS = 4;
  const TOTAL = 128;
  const SELECTION_VERSION = "chords-stratified-v1";
  const RHYTHM_MASK = "1111";
  const DEFAULT_CATEGORIES = Object.freeze([
    "stream-light", "stream-middle", "stream-dense",
    "jack-light", "jack-middle", "jack-dense",
    "trill", "one-hand trill", "chord", "chord-jack", "chord-stream", "other"
  ]);
  // Counts by number of chord slots (1..4), maximum simultaneous notes (2..4).
  // Non-uniform by design: not a representative sample of natural chart frequency.
  const QUOTAS = Object.freeze([
    Object.freeze([24, 12, 4]),
    Object.freeze([18, 14, 8]),
    Object.freeze([10, 12, 10]),
    Object.freeze([3, 5, 8])
  ]);

  function popcount(n) {
    let result = 0;
    while (n !== 0) { result += n & 1; n >>>= 1; }
    return result;
  }

  function mirrorMask(mask) {
    return ((mask & 1) << 3) | ((mask & 2) << 1) |
      ((mask & 4) >>> 1) | ((mask & 8) >>> 3);
  }

  function mirrorCode(code) {
    if (!Number.isSafeInteger(code) || code < 0 || code > 65535) {
      throw new Error("invalid 4-slot mask code");
    }
    let result = 0;
    for (let slot = 0; slot < SLOTS; slot++) {
      result |= mirrorMask((code >>> (slot * 4)) & 15) << (slot * 4);
    }
    return result;
  }

  function masksFor(code) {
    return Array.from({length: SLOTS}, (_, slot) => (code >>> (slot * 4)) & 15);
  }

  function metadata(code) {
    const masks = masksFor(code);
    const notesPerSlot = masks.map(popcount);
    const chordSlots = notesPerSlot.filter(n => n > 1).length;
    const maxChordSize = Math.max(...notesPerSlot);
    let maxLaneRun = 0;
    for (let lane = 0; lane < 4; lane++) {
      let run = 0;
      for (const mask of masks) {
        run = (mask & (1 << lane)) ? run + 1 : 0;
        maxLaneRun = Math.max(run, maxLaneRun);
      }
    }
    return {
      masks, notesPerSlot, chordSlots, maxChordSize,
      totalNotes: notesPerSlot.reduce((sum, n) => sum + n, 0),
      maxLaneRun
    };
  }

  function asPattern(code, name) {
    const notes = [];
    masksFor(code).forEach((mask, slot) => {
      for (let lane = 0; lane < 4; lane++) {
        if (mask & (1 << lane)) notes.push({lane, position:[slot, 4]});
      }
    });
    return {name, time_scale:0, notes};
  }

  const groups = new Map();
  for (let chordSlots = 1; chordSlots <= 4; chordSlots++) {
    for (let maxChordSize = 2; maxChordSize <= 4; maxChordSize++) {
      groups.set(chordSlots + ":" + maxChordSize, []);
    }
  }
  let population = 0;
  for (let code = 1; code <= 65535; code++) {
    const masks = masksFor(code);
    if (masks.includes(0) || masks.every(mask => popcount(mask) === 1)) continue;
    if (code > mirrorCode(code)) continue;
    const m = metadata(code);
    groups.get(m.chordSlots + ":" + m.maxChordSize).push(code);
    population++;
  }
  if (population !== 25225) throw new Error("unexpected chord population " + population);

  function rng(seed) {
    let state = seed >>> 0;
    return function() {
      state = (state + 0x6d2b79f5) >>> 0;
      let t = state;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  function shuffle(random, items) {
    for (let i = items.length - 1; i > 0; i--) {
      const j = Math.floor(random() * (i + 1));
      [items[i], items[j]] = [items[j], items[i]];
    }
    return items;
  }

  // Reserve a known example so the user's DFJ/DFJK/DFJK/FJK shape
  // is guaranteed to be seen in the representative sample.
  const EXAMPLE_MASKS = Object.freeze([7, 15, 15, 14]);
  const exampleCode = EXAMPLE_MASKS.reduce((result, mask, slot) =>
    result | (mask << (slot * 4)), 0);
  const EXAMPLE_CODE = Math.min(exampleCode, mirrorCode(exampleCode));

  function order(seed) {
    if (!Number.isSafeInteger(seed) || seed < 0 || seed > 4294967295) {
      throw new Error("seed must be an unsigned 32-bit integer");
    }
    const random = rng(seed), selected = [];
    for (let chordSlots = 1; chordSlots <= 4; chordSlots++) {
      for (let maxChordSize = 2; maxChordSize <= 4; maxChordSize++) {
        const group = groups.get(chordSlots + ":" + maxChordSize);
        const quota = QUOTAS[chordSlots-1][maxChordSize-2];
        if (group.length < quota) throw new Error("insufficient pattern stratum");
        const chosen = shuffle(random, group.slice()).slice(0, quota);
        if (chordSlots === 4 && maxChordSize === 4 && !chosen.includes(EXAMPLE_CODE)) {
          chosen[chosen.length-1] = EXAMPLE_CODE;
        }
        selected.push(...chosen);
      }
    }
    if (selected.length !== TOTAL || new Set(selected).size !== TOTAL) {
      throw new Error("sample count mismatch");
    }
    return shuffle(random, selected);
  }

  let cachedSeed = null, cachedOrder = null;
  function generate(seed, index) {
    if (!Number.isSafeInteger(index) || index < 0 || index >= TOTAL) {
      throw new Error("index must be 0.." + (TOTAL - 1));
    }
    if (cachedSeed !== seed) {
      cachedOrder = order(seed);
      cachedSeed = seed;
    }
    const code = cachedOrder[index];
    const flipped = mirrorCode(code);
    const m = metadata(code);
    const id = "chord-" + code.toString(16).padStart(4, "0");
    return {
      id, generator_index:index, canonical_code:code, mirror_code:flipped,
      grid_divisions:4, rhythm_mask:RHYTHM_MASK,
      step_masks:m.masks, notes_per_slot:m.notesPerSlot,
      chord_slots:m.chordSlots, max_chord_size:m.maxChordSize,
      total_notes:m.totalNotes, max_lane_run:m.maxLaneRun,
      sampling_stratum:m.chordSlots + ":" + m.maxChordSize,
      pattern:asPattern(code,id),
      mirror_pattern:asPattern(flipped,id+"-mirror")
    };
  }

  function fingerprint(candidate) {
    return candidate.step_masks.join(",");
  }

  const api = {SLOTS,TOTAL,RHYTHM_MASK,SELECTION_VERSION,DEFAULT_CATEGORIES,
    QUOTAS,EXAMPLE_MASKS,EXAMPLE_CODE,POPULATION:population,generate,order,
    mirrorMask,mirrorCode,masksFor,metadata,asPattern,fingerprint};
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  root.ChordPatternGenerator = api;
})(globalThis);
