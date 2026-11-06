"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const G = require("./generator.js");

test("exactly 312 distinct mirror-equivalence classes of 624 nonempty patterns", () => {
  const indices = G.order(20261106);
  assert.equal(G.TOTAL, 312);
  assert.equal(indices.length, 312);
  assert.equal(new Set(indices).size, 312);

  const all = new Set();
  const ids = new Set();
  for (let i = 0; i < G.TOTAL; i++) {
    const candidate = G.generate(20261106, i);
    assert.ok(candidate.canonical_code < candidate.mirror_code);
    assert.equal(G.mirrorCode(candidate.canonical_code), candidate.mirror_code);
    assert.equal(G.mirrorCode(candidate.mirror_code), candidate.canonical_code);
    ids.add(candidate.id);
    all.add(candidate.canonical_code);
    all.add(candidate.mirror_code);
  }
  assert.equal(ids.size, 312);
  assert.deepEqual([...all].sort((a,b)=>a-b), Array.from({length:624}, (_,i)=>i+1));
});

test("all notes are on the 4 sixteenth-note slots, no chords, up to 4 notes", () => {
  for (let i = 0; i < G.TOTAL; i++) {
    const candidate = G.generate(12345, i);
    for (const pattern of [candidate.pattern, candidate.mirror_pattern]) {
      assert.equal(pattern.time_scale, 0);
      assert.ok(pattern.notes.length >= 1 && pattern.notes.length <= 4);
      const times = new Set();
      for (const note of pattern.notes) {
        assert.ok(Number.isInteger(note.lane) && note.lane >= 0 && note.lane < 4);
        assert.equal(note.position[1], 4);
        assert.ok(Number.isInteger(note.position[0]) &&
          note.position[0] >= 0 && note.position[0] < 4);
        assert.ok(!times.has(note.position[0]), "chord or duplicate at a slot");
        times.add(note.position[0]);
      }
    }
  }
});

test("seed changes ordering, not candidate set or stable identifiers", () => {
  assert.deepEqual(G.order(99), G.order(99));
  assert.notDeepEqual(G.order(99), G.order(100));
  const ids = seed => Array.from({length:G.TOTAL},
    (_,i) => G.generate(seed, i).id).sort();
  assert.deepEqual(ids(99), ids(100));
});

test("reflection is an involution and candidate IDs remain canonical", () => {
  for (let code = 0; code < 625; code++) {
    assert.equal(G.mirrorCode(G.mirrorCode(code)), code);
  }
  assert.equal(G.mirrorCode(0), 0);
  for (let code = 1; code < 625; code++) {
    assert.notEqual(G.mirrorCode(code), code);
  }
});

test("invalid generator arguments are rejected", () => {
  for (const args of [[-1,0],[1,-1],[4294967296,0],[1,312],[1,1.5]]) {
    assert.throws(() => G.generate(...args));
  }
});
