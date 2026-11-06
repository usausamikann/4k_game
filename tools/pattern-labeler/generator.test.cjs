"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const G = require("./generator.js");

function fullyOccupied(code) {
  for (let slot = 0; slot < 4; slot++) {
    if (code % 5 === 0) return false;
    code = Math.floor(code / 5);
  }
  return true;
}

test("128 distinct representatives cover all 256 full-rhythm sequences", () => {
  assert.equal(G.TOTAL, 128);
  assert.equal(G.RHYTHM_MASK, "1111");
  const seen = new Set(), canonical = new Set(), ids = new Set();
  for (let i = 0; i < G.TOTAL; i++) {
    const c = G.generate(20261106, i);
    assert.ok(c.canonical_code < c.mirror_code);
    assert.ok(fullyOccupied(c.canonical_code));
    assert.ok(fullyOccupied(c.mirror_code));
    assert.equal(G.mirrorCode(c.canonical_code), c.mirror_code);
    assert.equal(G.mirrorCode(c.mirror_code), c.canonical_code);
    assert.equal(c.rhythm_mask, "1111");
    assert.match(c.id, /^pattern-\d{3}$/);
    canonical.add(c.canonical_code);
    seen.add(c.canonical_code);
    seen.add(c.mirror_code);
    ids.add(c.id);
  }
  assert.equal(canonical.size, 128);
  assert.equal(ids.size, 128);
  assert.equal(seen.size, 256);
  const expected = [];
  for (let code = 1; code < 625; code++) if (fullyOccupied(code)) expected.push(code);
  assert.deepEqual([...seen].sort((a,b)=>a-b), expected);
});

test("each one-beat chart has 4 strictly timed notes, with no chords", () => {
  for (let i = 0; i < G.TOTAL; i++) {
    const c = G.generate(42, i);
    for (const p of [c.pattern,c.mirror_pattern]) {
      assert.equal(p.time_scale, 0);
      assert.equal(p.notes.length, 4);
      assert.deepEqual(p.notes.map(note => note.position),
        [[0,4],[1,4],[2,4],[3,4]]);
      for (const note of p.notes) {
        assert.ok(Number.isInteger(note.lane) && note.lane >= 0 && note.lane <= 3);
      }
    }
    const mirrored = c.pattern.notes.map(n => 3-n.lane);
    assert.deepEqual(c.mirror_pattern.notes.map(n=>n.lane), mirrored);
  }
});

test("seed changes order, not set or identifiers (base-5 IDs unchanged)", () => {
  assert.deepEqual(G.order(99), G.order(99));
  assert.notDeepEqual(G.order(99), G.order(100));
  const ids = seed => Array.from({length:G.TOTAL},(_,i)=>G.generate(seed,i).id).sort();
  assert.deepEqual(ids(99), ids(100));
  for (const id of ids(99)) assert.match(id, /^pattern-\d{3}$/);
});

test("mirror operation is an involution; no fully occupied pattern is self-mirror", () => {
  assert.equal(G.mirrorCode(0), 0);
  for (let code = 0; code < 625; code++) {
    assert.equal(G.mirrorCode(G.mirrorCode(code)), code);
    if (fullyOccupied(code)) assert.notEqual(G.mirrorCode(code), code);
  }
});

test("invalid seed / index are rejected", () => {
  for (const args of [[-1,0],[1,-1],[4294967296,0],[1,128],[1,1.5]]) {
    assert.throws(() => G.generate(...args));
  }
});
