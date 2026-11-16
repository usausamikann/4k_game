/* Exhaustive, mirror-reduced 1-beat 1111-rhythm candidates.
 * Four 16th-note slots, exactly one note on one of the 4 lanes per slot.
 * There are 4^4=256 fully occupied patterns, paired by horizontal mirror
 * (lane 0<->3 and lane 1<->2), yielding 128 orbit representatives.
 * Browser: window.PatternLabelerGenerator; Node: require("./generator.js").
 */
(function(root) {
  "use strict";
  const DEFAULT_CATEGORIES = Object.freeze([
    "stream-light", "stream-middle", "stream-dense",
    "jack-light", "jack-middle", "jack-dense", "other", "trill"
  ]);
  const SLOTS = 4;
  const TOTAL = 128;
  const RHYTHM_MASK = "1111";

  function mirrorCode(code) {
    if (!Number.isInteger(code) || code < 0 || code >= 625) throw new Error("invalid code");
    let digits = code, mirrored = 0, multiplier = 1;
    for (let i = 0; i < SLOTS; i++) {
      const digit = digits % 5;
      mirrored += (digit === 0 ? 0 : 5 - digit) * multiplier;
      digits = Math.floor(digits / 5);
      multiplier *= 5;
    }
    return mirrored;
  }

  function asPattern(code, id) {
    let digits = code;
    const notes = [];
    for (let slot = 0; slot < SLOTS; slot++) {
      const digit = digits % 5;
      digits = Math.floor(digits / 5);
      if (digit !== 0) notes.push({lane: digit - 1, position: [slot, 4]});
    }
    return {name: id, time_scale: 0, notes};
  }

  // Keep base-5 codes and names stable with the former 312-pattern dataset.
  // A nonzero digit at all four slots means the rhythm mask is exactly 1111.
  function isFullyOccupied(code) {
    for (let slot = 0; slot < SLOTS; slot++) {
      if (code % 5 === 0) return false;
      code = Math.floor(code / 5);
    }
    return true;
  }

  const codes = [];
  for (let code = 1; code < 625; code++) {
    if (isFullyOccupied(code) && code < mirrorCode(code)) codes.push(code);
  }
  if (codes.length !== TOTAL) throw new Error("unexpected orbit count");

  // Fisher-Yates shuffle with a reproducible seeded unsigned 32-bit PRNG.
  function order(seed) {
    if (!Number.isSafeInteger(seed) || seed < 0 || seed > 4294967295) {
      throw new Error("seed must be an unsigned 32-bit integer");
    }
    let state = seed >>> 0;
    const random = function() {
      state = (state + 0x6d2b79f5) >>> 0;
      let t = state;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
    const list = codes.slice();
    for (let i = list.length - 1; i > 0; i--) {
      const j = Math.floor(random() * (i + 1));
      [list[i], list[j]] = [list[j], list[i]];
    }
    return list;
  }

  let cachedSeed = null, cachedOrder = null;
  function generate(seed, index) {
    if (!Number.isSafeInteger(index) || index < 0 || index >= TOTAL) {
      throw new Error("index must be 0.." + (TOTAL - 1));
    }
    // Validation is performed by order, including when the seed changes.
    if (cachedSeed !== seed) {
      cachedOrder = order(seed);
      cachedSeed = seed;
    }
    const code = cachedOrder[index];
    const reversed = mirrorCode(code);
    const id = "pattern-" + String(code).padStart(3, "0");
    return {
      id, generator_index: index, canonical_code: code, mirror_code: reversed,
      grid_divisions: 4, rhythm_mask: RHYTHM_MASK,
      pattern: asPattern(code, id),
      mirror_pattern: asPattern(reversed, id + "-mirror")
    };
  }

  function fingerprint(candidate) {
    return JSON.stringify(candidate.pattern.notes);
  }
  const api = {generate, fingerprint, mirrorCode, order, TOTAL, SLOTS, RHYTHM_MASK, DEFAULT_CATEGORIES};
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  root.PatternLabelerGenerator = api;
})(globalThis);
