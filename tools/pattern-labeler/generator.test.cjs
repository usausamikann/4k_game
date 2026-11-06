"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const G = require("./generator.js");

test("same seed and generator index produce the same candidate", () => {
  assert.deepEqual(G.generate(20261106, 1234, "0"), G.generate(20261106, 1234, "0"));
  assert.notDeepEqual(G.generate(20261106, 1234, "0"), G.generate(20261106, 1235, "0"));
});

test("all sampled notes are legal 1-beat positions and fit 960 ticks", () => {
  for (const mode of ["0", "1", "mixed"]) {
    for (let i = 0; i < 2000; i++) {
      const item = G.generate(20261106, i, mode);
      assert.ok(G.GRIDS.includes(item.grid_divisions));
      assert.ok(item.pattern.notes.length >= 2);
      assert.ok(["stream","jack","chord","mixed"].includes(item.sampling_recipe));
      assert.ok(mode === "mixed" || item.pattern.time_scale === Number(mode));
      const cells = new Set();
      let lastTick = -1, lastLane = -1;
      for (const note of item.pattern.notes) {
        const [a, b] = note.position;
        assert.ok(Number.isInteger(a) && Number.isInteger(b) && b > 0 && a >= 0 && a < b);
        assert.ok(Number.isInteger(note.lane) && note.lane >= 0 && note.lane < 4);
        assert.equal((960*a) % b, 0);
        const tick = (960*a) / b;
        assert.equal(tick % (item.pattern.time_scale+1), 0);
        const fingerprint = note.lane + ":" + tick;
        assert.ok(!cells.has(fingerprint), "duplicate note " + fingerprint);
        cells.add(fingerprint);
        assert.ok(tick > lastTick || (tick === lastTick && note.lane > lastLane));
        lastTick = tick; lastLane = note.lane;
      }
    }
  }
});

test("mixed mode includes both time scales and generator includes diverse recipes", () => {
  const scales = new Set(), recipes = new Set(), fingerprints = new Set();
  for (let i = 0; i < 250; i++) {
    const item = G.generate(99, i, "mixed");
    scales.add(item.pattern.time_scale);
    recipes.add(item.sampling_recipe);
    fingerprints.add(G.fingerprint(item));
  }
  assert.deepEqual(Array.from(scales).sort(), [0,1]);
  assert.equal(recipes.size, 4);
  assert.ok(fingerprints.size > 180, "candidate diversity");
});

test("invalid generator configuration is rejected", () => {
  for (const args of [[-1,0,"0"],[1,-1,"0"],[1,0,"x"],[4294967296,0,"0"],[1,1.5,"0"]]) {
    assert.throws(() => G.generate(...args));
  }
});
