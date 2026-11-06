/* Deterministic candidate generator for the 4K Game labeling draft.
 * Browser: window.PatternLabelerGenerator; Node: require("./generator.js").
 * This samples candidate patterns; it does NOT enumerate all possible patterns.
 */
(function (root) {
  "use strict";

  const GRIDS = Object.freeze([4, 6, 8, 12, 16]);
  const DEFAULT_CATEGORIES = Object.freeze([
    "stream-light",
    "stream-middle",
    "stream-dense",
    "jack-light",
    "jack-middle",
    "jack-dense",
    "other",
  ]);

  function gcd(a, b) {
    while (b !== 0) {
      const next = a % b;
      a = b;
      b = next;
    }
    return a;
  }

  // Each index has its own PRNG state: skipping duplicate candidates is stable.
  function rngFor(seed, index) {
    let state = ((seed >>> 0) ^ Math.imul(index + 1, 0x9e3779b1) ^ 0x85ebca6b) >>> 0;
    return function () {
      state = (state + 0x6d2b79f5) >>> 0;
      let value = state;
      value = Math.imul(value ^ (value >>> 15), value | 1);
      value ^= value + Math.imul(value ^ (value >>> 7), value | 61);
      return ((value ^ (value >>> 14)) >>> 0) / 4294967296;
    };
  }

  function randomInt(random, min, max) {
    return min + Math.floor(random() * (max - min + 1));
  }

  function shuffled(random, length) {
    const values = Array.from({ length: length }, (_, i) => i);
    for (let i = values.length - 1; i > 0; i--) {
      const j = randomInt(random, 0, i);
      [values[i], values[j]] = [values[j], values[i]];
    }
    return values;
  }

  function generate(seed, index, scaleMode) {
    if (!Number.isSafeInteger(seed) || seed < 0 || seed > 4294967295) {
      throw new Error("seed must be an unsigned 32-bit integer");
    }
    if (!Number.isSafeInteger(index) || index < 0) {
      throw new Error("index must be a nonnegative safe integer");
    }
    if (!["0", "1", "mixed"].includes(scaleMode)) {
      throw new Error("scaleMode must be 0, 1, or mixed");
    }

    const random = rngFor(seed, index);
    const grid = GRIDS[randomInt(random, 0, GRIDS.length - 1)];
    const timeScale = scaleMode === "mixed"
      ? (random() < 0.25 ? 1 : 0) : Number(scaleMode);
    const recipes = ["stream", "jack", "chord", "mixed"];
    const recipe = recipes[randomInt(random, 0, recipes.length - 1)];
    const target = randomInt(random, 2, Math.min(12, grid * 2));
    const cells = new Set();

    function add(slot, lane) {
      cells.add(slot * 4 + lane);
    }

    if (recipe === "stream") {
      const times = shuffled(random, grid).slice(0, Math.min(grid, target)).sort((a, b) => a - b);
      let prev = randomInt(random, 0, 3);
      for (const slot of times) {
        let lane = randomInt(random, 0, 3);
        if (lane === prev && random() < 0.8) {
          lane = (prev + randomInt(random, 1, 3)) % 4;
        }
        add(slot, lane);
        prev = lane;
      }
    } else if (recipe === "jack") {
      const times = shuffled(random, grid).slice(0, Math.min(grid, Math.max(3, target)));
      const anchor = randomInt(random, 0, 3);
      for (const slot of times) {
        add(slot, random() < 0.8 ? anchor : randomInt(random, 0, 3));
      }
    } else if (recipe === "chord") {
      const times = shuffled(random, grid).slice(0, randomInt(random, 1, Math.min(grid, 4)));
      for (let i = 0; i < target; i++) {
        const slot = times[randomInt(random, 0, times.length - 1)];
        add(slot, randomInt(random, 0, 3));
      }
    } else {
      const cellsShuffled = shuffled(random, grid * 4);
      for (const cell of cellsShuffled.slice(0, target)) {
        cells.add(cell);
      }
    }

    // A randomly selected cell can repeat in the chord recipe.
    // Guarantee at least two unique notes without changing the deterministic sequence.
    while (cells.size < 2) {
      cells.add(randomInt(random, 0, grid * 4 - 1));
    }

    const placements = Array.from(cells, cell => ({
      slot: Math.floor(cell / 4),
      lane: cell % 4,
    })).sort((a, b) => a.slot - b.slot || a.lane - b.lane);

    const notes = placements.map(({ slot, lane }) => {
      const divisor = gcd(slot, grid);
      return {
        lane: lane,
        position: [slot / divisor, grid / divisor],
      };
    });

    const id = "sample-" + seed + "-" + index;
    return {
      id: id,
      generator_index: index,
      grid_divisions: grid,
      sampling_recipe: recipe,
      pattern: {
        name: id,
        time_scale: timeScale,
        notes: notes,
      },
    };
  }

  function fingerprint(candidate) {
    return JSON.stringify({
      time_scale: candidate.pattern.time_scale,
      notes: candidate.pattern.notes,
    });
  }

  const api = { generate, fingerprint, GRIDS, DEFAULT_CATEGORIES };
  if (typeof module !== "undefined" && module.exports) {
    module.exports = api;
  }
  root.PatternLabelerGenerator = api;
})(globalThis);
