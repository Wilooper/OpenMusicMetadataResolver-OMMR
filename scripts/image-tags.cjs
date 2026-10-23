'use strict';

function imagePlan({ owner, ref, sha, currentSha, defaultBranch, suffix = '' }) {
  if (!/^[0-9a-f]{40}$/.test(sha)) throw new Error('Expected a full commit SHA');
  if (!['', '-python'].includes(suffix)) throw new Error('Unknown image variant');
  if (!/^[a-z0-9-]+$/i.test(owner)) throw new Error('Invalid registry owner');
  const match = /^refs\/(heads|tags)\/(.+)$/.exec(ref);
  if (!match) throw new Error('Only branch and tag refs can be published');
  const { createHash } = require('node:crypto');
  const kind = match[1] === 'heads' ? 'branch' : 'tag';
  const name = match[2];
  const image = 'ghcr.io/' + owner.toLowerCase() + '/ommr';
  const immutableTag = image + ':sha-' + sha + suffix;
  if (currentSha !== sha) return { image, immutableTag, current: false, aliases: [] };
  const slug = name.toLowerCase().replace(/[^a-z0-9_.-]+/g, '-').replace(/^[.-]+/, '').slice(0, 70) || 'ref';
  const hash = createHash('sha256').update(ref).digest('hex').slice(0, 8);
  const aliases = [kind + '-' + slug + '-' + hash + suffix];
  if (kind === 'branch' && name === defaultBranch) aliases.push('latest' + suffix);
  if (kind === 'tag' && /^v?\d+\.\d+\.\d+$/.test(name)) aliases.push(name.replace(/^v/, '') + suffix);
  return { image, immutableTag, current: true, aliases };
}

module.exports = { imagePlan };
