const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class Element {
  constructor(tag) {
    this.tag = tag; this.children = []; this.textContent = ''; this.attributes = {};
    this.listeners = {};
  }
  append(...items) { items.forEach(item => this.appendChild(item)); }
  appendChild(item) { item.parent = this; this.children.push(item); }
  replaceChildren(...items) {
    this.children = [];
    items.forEach(item => this.appendChild(item));
  }
  setAttribute(name, value) { this.attributes[name] = value; }
  getAttribute(name) { return this.attributes[name]; }
  addEventListener(name, callback) { this.listeners[name] = callback; }
  focus() {}
  remove() {
    if (this.parent) this.parent.children = this.parent.children.filter(item => item !== this);
  }
  set innerHTML(value) { throw new Error('Unsafe HTML sink used'); }
}

function environment(events, contentType = 'application/x-ndjson') {
  const history = [];
  const opened = [];
  const blobs = [];
  class TestURL extends URL {
    static createObjectURL(blob) {
      blobs.push(blob);
      return 'blob:raw-response';
    }
    static revokeObjectURL() {}
  }
  const encoder = new TextEncoder();
  const context = vm.createContext({
    URL: TestURL, TextDecoder, Blob, window: { open: (...args) => opened.push(args) },
    setTimeout: callback => callback(), document: { createElement: tag => new Element(tag) },
    fetch: async () => ({
      ok: true, status: 200, headers: { get: () => contentType },
      json: async () => ({ status: 'legacy_route' }),
      body: new ReadableStream({ start(controller) {
        for (const event of events) controller.enqueue(encoder.encode(JSON.stringify(event) + '\n'));
        controller.close();
      } })
    }),
    getChatHistory: () => history, saveChatHistory: () => {}, addBubbleMeta: () => {}
  });
  const script = path.join(__dirname, '../../web/scripts/01_search_flow.js');
  vm.runInContext(fs.readFileSync(script, 'utf8'), context);
  return { context, history, opened, blobs };
}

const timer = { stop() {}, seconds() { return 1; } };

test('streamed search has all and type tabs with source sections', async () => {
  const { context, history, opened, blobs } = environment([
    { type: 'plan_ready', plan: { understanding: { entities: {}, needs_input: false }, tasks: [] } },
    { type: 'task_result', result: { source: 'internet', status: 'ok', items: [
      { kind: 'wiki', title: '<img onerror=alert(1)>', url: 'javascript:alert(1)',
        poster: 'https://images.example/wiki.jpg', year: 2002, content: '<script>bad()</script>' }
    ] } },
    { type: 'task_result', result: { source: 'youtube', status: 'ok', items: [
      { kind: 'video', title: 'Naruto clip', url: 'https://youtube.com/watch?v=1' }
    ] } },
    { type: 'task_result', result: { source: 'plex', status: 'error', error_code: 'timeout', items: [] } },
    { type: 'response_completed', status: 'partial' }
  ]);
  const bubble = new Element('div');
  assert.equal(await context.trySearchFlow('tìm', bubble, timer), true);
  const viewSwitch = bubble.children[2];
  const resultButton = viewSwitch.children[0];
  const rawButton = viewSwitch.children[1];
  const tabs = bubble.children[3];
  assert.deepEqual(tabs.children.map(tab => tab.textContent), [
    'Tất cả (2)', 'Khác (1)', 'Videos (1)'
  ]);
  assert.equal(tabs.children[0].getAttribute('aria-selected'), 'true');
  const groups = bubble.children[4];
  assert.equal(groups.children.length, 3);
  const wikiCard = groups.children[1].children[0].children[1].children[0];
  assert.equal(wikiCard.children[0].tag, 'img');
  assert.equal(wikiCard.children[0].src, 'https://images.example/wiki.jpg');
  const cardTitle = wikiCard.children[1].children[0];
  assert.equal(cardTitle.tag, 'strong');
  assert.equal(cardTitle.textContent, '<img onerror=alert(1)>');
  assert.match(wikiCard.children[1].children[1].textContent, /2002/);
  assert.match(history[0].text, /2 kết quả/);
  assert.equal(resultButton.textContent, 'Result');
  assert.equal(resultButton.getAttribute('aria-pressed'), 'true');
  assert.equal(rawButton.textContent, 'Raw');
  assert.equal(rawButton.disabled, false);
  rawButton.listeners.click();
  assert.deepEqual(opened, [['blob:raw-response', '_blank', 'noopener']]);
  const rawEvents = JSON.parse(await blobs[0].text());
  assert.equal(rawEvents.length, 5);
  assert.equal(rawEvents[1].result.items[0].kind, 'wiki');
});

test('unsupported request hands off without modifying the chat bubble', async () => {
  const { context } = environment([], 'application/json');
  const bubble = new Element('div');
  bubble.textContent = 'original';
  assert.equal(await context.trySearchFlow('tắt đèn', bubble, timer), false);
  assert.equal(bubble.textContent, 'original');
});

test('truncated stream is not presented as complete', async () => {
  const { context } = environment([
    { type: 'plan_ready', plan: { understanding: { entities: {} }, tasks: [] } }
  ]);
  await assert.rejects(context.trySearchFlow('tìm', new Element('div'), timer), /kết thúc/);
});

test('no configured source produces one explanation without error cards', async () => {
  const { context } = environment([
    { type: 'plan_ready', plan: { understanding: { entities: {} }, tasks: [] } },
    { type: 'response_completed', status: 'unavailable' }
  ]);
  const bubble = new Element('div');
  await context.trySearchFlow('tìm phim', bubble, timer);
  assert.match(bubble.children[0].textContent, /Chưa có nguồn tìm kiếm khả dụng/);
  assert.equal(bubble.children[1].children.length, 0);
});

test('unavailable sources stay in the status area without result cards', async () => {
  const { context } = environment([
    { type: 'plan_ready', plan: { understanding: { entities: {} }, tasks: [] } },
    { type: 'task_result', result: {
      source: 'motchill', status: 'skipped', error_code: 'not_implemented', items: []
    } },
    { type: 'response_completed', status: 'unavailable' }
  ]);
  const bubble = new Element('div');
  await context.trySearchFlow('tìm phim', bubble, timer);
  assert.match(bubble.children[1].children[0].textContent, /Nguồn chưa được hỗ trợ/);
  assert.deepEqual(bubble.children[3].children.map(tab => tab.textContent), ['Tất cả (0)']);
  assert.equal(bubble.children[4].children.length, 1);
  assert.equal(bubble.children[4].children[0].children.length, 0);
});
