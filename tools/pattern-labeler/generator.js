/* Exhaustive, mirror-reduced 1-beat (4 sixteenth-note slots) candidates.
 * Each slot is REST or exactly one of 4 lanes. Nonempty = 5^4-1=624.
 * Mirror pairs have no nonempty fixed points, so there are 312 orbits.
 * Browser: window.PatternLabelerGenerator; Node: require("./generator.js").
 */
(function(root) {
  "use strict";
  const DEFAULT_CATEGORIES = Object.freeze([
    "stream-light", "stream-middle", "stream-dense",
    "jack-light", "jack-middle", "jack-dense", "other"
  ]);
  const SLOTS = 4;
  const TOTAL = 312;

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

  const codes = [];
  for (let code = 1; code < 625; code++) {
    if (code < mirrorCode(code)) codes.push(code);
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
      grid_divisions: 4,
      pattern: asPattern(code, id),
      mirror_pattern: asPattern(reversed, id + "-mirror")
    };
  }

  function fingerprint(candidate) {
    return JSON.stringify(candidate.pattern.notes);
  }
  const api = {generate, fingerprint, mirrorCode, order, TOTAL, SLOTS, DEFAULT_CATEGORIES};
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  root.PatternLabelerGenerator = api;
})(globalThis);
