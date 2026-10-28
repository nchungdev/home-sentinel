const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class Element {
  constructor(tag) {
    this.tag = tag; this.children = []; this.handlers = {}; this.value = ''; this.disabled = false;
  }
  append(...nodes) { for (const node of nodes) this.appendChild(node); }
  appendChild(node) {
    this.children.push(node);
    if (this.tag === 'select' && this.children.length === 1) this.value = node.value;
  }
  replaceChildren(...nodes) { this.children = []; this.append(...nodes); }
  addEventListener(event, handler) { this.handlers[event] = handler; }
  setAttribute() {}
  focus() { this.focused = true; }
  select() {}
  remove() { this.removed = true; }
  set innerHTML(_) { throw new Error('unsafe HTML sink'); }
}

function load(fetch) {
  const context = vm.createContext({
    document: { createElement: tag => new Element(tag) }, fetch, AbortController, WeakMap, URL,
    navigator: { userAgent: 'Desktop', clipboard: { writeText: async () => {} } },
    window: { location: { origin: 'http://localhost:8788' } }
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, '../../web/scripts/01_catalog.js'), 'utf8'),
    context);
  return context;
}

test('500 episodes render in pages instead of creating 500 buttons', () => {
  const context = load();
  const episodes = Array.from({ length: 500 }, (_, index) => ({
    id: String(index), label: 'Tập ' + index, group: 'Vietsub', playable: true
  }));
  const content = new Element('div');
  context.catalogRenderTitle({ name: 'Series', episodes }, content, new AbortController().signal);
  const list = content.children[2];
  const navigation = content.children[3];
  assert.equal(list.children.length, 25);
  assert.equal(list.children[0].textContent, 'Tập 0');
  assert.match(navigation.children[1].textContent, /1\/20/);
  navigation.children[2].handlers.click();
  assert.equal(list.children[0].textContent, 'Tập 25');
  assert.match(navigation.children[1].textContent, /2\/20/);
});

test('special labels are preserved and unsupported episodes stay disabled', () => {
  const context = load();
  const content = new Element('div');
  context.catalogRenderTitle({ name: 'Series', episodes: [
    { id: 'special', label: 'Tết đặc biệt', group: 'Main', playable: false }
  ] }, content, new AbortController().signal);
  const episode = content.children[2].children[0];
  assert.equal(episode.textContent, 'Tết đặc biệt');
  assert.equal(episode.disabled, true);
});

test('playback resolver does not turn an unexpected URL into a player link', async () => {
  const context = load(async () => ({ ok: true, json: async () => ({
    playback: { url: 'https://attacker.example/stream.m3u8' }
  }) }));
  const panel = new Element('div');
  await context.catalogResolveEpisode({ id: 'episode', label: 'Tập 1' }, panel,
    new AbortController().signal);
  assert.equal(panel.textContent, 'Link phát không hợp lệ.');
});
