"use strict";
const test = require("node:test");
const assert = require("node:assert/strict");
const G = require("./generator.js");

const countBits = x => x.toString(2).replace(/0/g, "").length;

test("mirror-reduced population is exactly 25,225 including 81 self-mirrors", () => {
  assert.equal(G.POPULATION, 25225);
  let classes = 0, fixed = 0;
  for (let code = 1; code <= 65535; code++) {
    const masks = G.masksFor(code);
    if (masks.includes(0) || masks.every(x => countBits(x) === 1)) continue;
    const flipped = G.mirrorCode(code);
    if (code <= flipped) {
      classes++;
      if (code === flipped) fixed++;
    }
  }
  assert.equal(classes, 25225);
  assert.equal(fixed, 81);
});

test("128 unique candidates are stratified as configured; sample contains example", () => {
  for (const seed of [0, 20261106, 0xffffffff]) {
    const selected = G.order(seed);
    assert.equal(selected.length, 128);
    assert.equal(new Set(selected).size, 128);
    assert.ok(selected.includes(G.EXAMPLE_CODE));
    const strata = new Map();
    const ids = new Set();
    for (let i = 0; i < G.TOTAL; i++) {
      const candidate = G.generate(seed, i);
      ids.add(candidate.id);
      assert.equal(candidate.rhythm_mask, "1111");
      assert.equal(candidate.pattern.time_scale, 0);
      assert.ok(candidate.step_masks.every(mask => mask > 0 && mask <= 15));
      assert.ok(candidate.chord_slots >= 1 && candidate.chord_slots <= 4);
      assert.ok(candidate.max_chord_size >= 2 && candidate.max_chord_size <= 4);
      assert.equal(candidate.total_notes,
        candidate.notes_per_slot.reduce((sum, n) => sum + n, 0));
      assert.equal(candidate.pattern.notes.length, candidate.total_notes);
      assert.equal(candidate.mirror_pattern.notes.length, candidate.total_notes);
      assert.equal(G.mirrorCode(candidate.canonical_code), candidate.mirror_code);
      assert.equal(G.mirrorCode(candidate.mirror_code), candidate.canonical_code);
      assert.ok(candidate.canonical_code <= candidate.mirror_code);
      strata.set(candidate.sampling_stratum, (strata.get(candidate.sampling_stratum)||0)+1);
      for (const note of candidate.pattern.notes) {
        const slot = note.position[0];
        assert.deepEqual(note.position, [slot,4]);
        assert.ok(note.lane >= 0 && note.lane <= 3);
        assert.ok(slot >= 0 && slot <= 3);
        assert.ok(candidate.step_masks[slot] & (1 << note.lane));
      }
    }
    assert.equal(ids.size, 128);
    for (let slots = 1; slots <= 4; slots++) {
      for (let maximum = 2; maximum <= 4; maximum++) {
        assert.equal(strata.get(slots+":"+maximum),
          G.QUOTAS[slots-1][maximum-2], "stratum: "+slots+":"+maximum);
      }
    }
  }
});

test("mirror operation is an involution and preserves time slots and note counts", () => {
  for (let mask=0;mask<16;mask++) assert.equal(G.mirrorMask(G.mirrorMask(mask)),mask);
  for (let code=0;code<=65535;code++) {
    assert.equal(G.mirrorCode(G.mirrorCode(code)),code);
  }
  for (let i=0;i<G.TOTAL;i++) {
    const candidate=G.generate(11,i);
    const expected=candidate.pattern.notes.map(note =>
      ({lane:3-note.lane, position:note.position.slice()}));
    expected.sort((a,b)=>a.position[0]-b.position[0]||a.lane-b.lane);
    assert.deepEqual(candidate.mirror_pattern.notes,expected);
  }
});

test("selection is reproducible and seed changes the selection", () => {
  assert.deepEqual(G.order(20261106), G.order(20261106));
  assert.notDeepEqual(G.order(20261106), G.order(20261107));
});

test("lane-run metric counts per-lane contiguous membership in chords", () => {
  const maskCode = [3,7,15,14].reduce((n,mask,slot)=>n|(mask<<(4*slot)),0);
  assert.equal(G.metadata(maskCode).maxLaneRun,4);
  const separate = [1,2,4,8].reduce((n,mask,slot)=>n|(mask<<(4*slot)),0);
  assert.equal(G.metadata(separate).maxLaneRun,1);
});

test("bad seeds and indices are rejected", () => {
  for (const args of [[-1,0],[1,-1],[4294967296,0],[1,128],[1,1.5]]) {
    assert.throws(()=>G.generate(...args));
  }
});
