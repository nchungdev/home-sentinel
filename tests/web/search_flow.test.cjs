const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class Element {
  constructor(tag) { this.tag = tag; this.children = []; this.textContent = ''; }
  append(...items) { this.children.push(...items); }
  appendChild(item) { this.children.push(item); }
  replaceChildren(...items) { this.children = items; }
  set innerHTML(value) { throw new Error('Unsafe HTML sink used'); }
}

function environment(events, contentType = 'application/x-ndjson') {
  const history = [];
  const encoder = new TextEncoder();
  const context = vm.createContext({
    URL, TextDecoder, document: { createElement: tag => new Element(tag) },
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
  return { context, history };
}

const timer = { stop() {}, seconds() { return 1; } };

test('streamed provider text is rendered safely and source sections remain', async () => {
  const { context, history } = environment([
    { type: 'plan_ready', plan: { understanding: { entities: {}, needs_input: false }, tasks: [] } },
    { type: 'task_result', result: { source: 'internet', status: 'ok', items: [
      { title: '<img onerror=alert(1)>', url: 'javascript:alert(1)', content: '<script>bad()</script>' }
    ] } },
    { type: 'task_result', result: { source: 'plex', status: 'error', error_code: 'timeout', items: [] } },
    { type: 'response_completed', status: 'partial' }
  ]);
  const bubble = new Element('div');
  assert.equal(await context.trySearchFlow('tìm', bubble, timer), true);
  const groups = bubble.children[1];
  assert.equal(groups.children.length, 2);
  const cardTitle = groups.children[0].children[1].children[0];
  assert.equal(cardTitle.tag, 'strong');
  assert.equal(cardTitle.textContent, '<img onerror=alert(1)>');
  assert.match(history[0].text, /1 kết quả/);
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

test('unavailable sources are collapsed instead of rendered as large cards', async () => {
  const { context } = environment([
    { type: 'plan_ready', plan: { understanding: { entities: {} }, tasks: [] } },
    { type: 'task_result', result: {
      source: 'motchill', status: 'skipped', error_code: 'not_implemented', items: []
    } },
    { type: 'response_completed', status: 'unavailable' }
  ]);
  const bubble = new Element('div');
  await context.trySearchFlow('tìm phim', bubble, timer);
  const failures = bubble.children[1].children;
  assert.equal(failures.length, 1);
  assert.equal(failures[0].tag, 'details');
  assert.equal(failures[0].open, undefined);
});
