'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { imagePlan } = require('./image-tags.cjs');
const sha = 'a'.repeat(40);
const base = { owner: 'Wilooper', ref: 'refs/heads/main', sha, currentSha: sha, defaultBranch: 'main' };

test('full commit tag and default branch latest', () => {
  const plan = imagePlan(base);
  assert.equal(plan.immutableTag, 'ghcr.io/wilooper/ommr:sha-' + sha);
  assert.ok(plan.aliases.includes('latest'));
});
test('other branches cannot update latest', () => {
  assert.ok(!imagePlan({ ...base, ref: 'refs/heads/feature/a' }).aliases.includes('latest'));
});
test('actual default branch is used rather than hardcoded main', () => {
  assert.ok(imagePlan({ ...base, ref: 'refs/heads/develop', defaultBranch: 'develop' }).aliases.includes('latest'));
  assert.ok(!imagePlan({ ...base, defaultBranch: 'develop' }).aliases.includes('latest'));
});
test('moved or deleted refs cannot promote', () => {
  for (const currentSha of ['b'.repeat(40), undefined]) {
    const plan = imagePlan({ ...base, currentSha });
    assert.equal(plan.current, false);
    assert.deepEqual(plan.aliases, []);
    assert.ok(plan.immutableTag.includes(sha));
  }
});
test('Python variant never overwrites Go aliases', () => {
  const plan = imagePlan({ ...base, suffix: '-python' });
  assert.ok(plan.immutableTag.endsWith('-python'));
  assert.ok(plan.aliases.includes('latest-python'));
  assert.ok(plan.aliases.every(tag => tag.endsWith('-python')));
});
test('normalized branch collisions remain distinct', () => {
  const a = imagePlan({ ...base, ref: 'refs/heads/feature/a' });
  const b = imagePlan({ ...base, ref: 'refs/heads/feature-a' });
  assert.notEqual(a.aliases[0], b.aliases[0]);
});
test('release tag preserves exact version without taking latest', () => {
  const plan = imagePlan({ ...base, ref: 'refs/tags/v1.2.3' });
  assert.ok(plan.aliases.includes('1.2.3'));
  assert.ok(!plan.aliases.includes('latest'));
});
test('tag and branch namespaces stay distinct', () => {
  assert.notEqual(imagePlan({ ...base, ref: 'refs/tags/main' }).aliases[0], imagePlan(base).aliases[0]);
});
test('long and unusual ref names produce legal bounded tags', () => {
  const plan = imagePlan({ ...base, ref: 'refs/heads/' + 'Very-long.ref/'.repeat(30), suffix: '-python' });
  assert.ok(plan.aliases[0].length <= 128);
  assert.match(plan.aliases[0], /^[a-zA-Z0-9_][a-zA-Z0-9_.-]*$/);
});
test('reject malformed publish inputs', () => {
  for (const override of [{ sha: 'abc' }, { suffix: '-other' }, { owner: '../owner' }, { ref: 'refs/pull/1/merge' }]) {
    assert.throws(() => imagePlan({ ...base, ...override }));
  }
});
