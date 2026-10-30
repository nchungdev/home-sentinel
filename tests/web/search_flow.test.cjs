const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class Element {
  constructor(tag) {
    this.tag = tag; this.children = []; this.textContent = ''; this.attributes = {};
    this.listeners = {}; this.dataset = {};
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

function environment(events, contentType = 'application/x-ndjson', fetchHandler) {
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
    URL: TestURL, TextDecoder, Blob, AbortController,
    window: { open: (...args) => opened.push(args), location: { origin: 'http://localhost' } },
    setTimeout: callback => callback(), document: { createElement: tag => new Element(tag) },
    fetch: async (url, options) => {
      const overridden = fetchHandler && await fetchHandler(url, options);
      if (overridden) return overridden;
      return {
        ok: true, status: 200, headers: { get: () => contentType },
        json: async () => ({ status: 'legacy_route' }),
        body: new ReadableStream({ start(controller) {
          for (const event of events) {
            controller.enqueue(encoder.encode(JSON.stringify(event) + '\n'));
          }
          controller.close();
        } })
      };
    },
    getChatHistory: () => history, saveChatHistory: () => {}, addBubbleMeta: () => {}
  });
  const catalogScript = path.join(__dirname, '../../web/scripts/01_catalog.js');
  vm.runInContext(fs.readFileSync(catalogScript, 'utf8'), context);
  const mediaScript = path.join(__dirname, '../../web/scripts/01_catalog_media.js');
  vm.runInContext(fs.readFileSync(mediaScript, 'utf8'), context);
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

test('TMDB result opens detail and does not claim unverified providers', async () => {
  const requests = [];
  const { context } = environment([
    { type: 'plan_ready', plan: { understanding: { entities: {} }, tasks: [] } },
    { type: 'task_result', result: { source: 'tmdb', status: 'ok', items: [
      { kind: 'movie', title: 'The Running Man', year: 2025, poster: 'https://image.tmdb.org/poster.jpg',
        actions: [{ type: 'media_detail', item_id: 'movie/123' }] }
    ] } },
    { type: 'task_result', result: { source: 'youtube', status: 'ok', items: [
      { kind: 'video', title: 'Running Man clip', url: 'https://youtube.com/watch?v=1' }
    ] } },
    { type: 'response_completed', status: 'partial' }
  ], 'application/x-ndjson', async url => {
    requests.push(url);
    if (url === '/v2/flow') return null;
    if (url.endsWith('/discover')) {
      return {
        ok: true, json: async () => ({ source: {
          source_id: 'phimapi', status: 'ok', discovery_method: 'ai_query_expansion',
          items: [{ title: 'The Running Man', match_method: 'title_year', summary: '2025' }]
        } })
      };
    }
    return {
      ok: true, json: async () => ({ title: { title: 'The Running Man', year: 2025,
        overview: 'Synopsis' }, sources: [
        { source_id: 'plex', status: 'ok', items: [
          { title: 'Different localized title', actions: [
            { type: 'open_url', label: 'Mở Plex', url: 'http://plex.local/item/1' }
          ] }
        ] },
        { source_id: 'jellyfin', status: 'empty', error_code: 'identity_not_verified', items: [] },
        { source_id: 'phimapi', status: 'ok', items: [
          { title: 'The Running Man', match_method: 'title_year', summary: '2025' }
        ] }
      ] })
    };
  });
  const bubble = new Element('div');
  await context.trySearchFlow('running man', bubble, timer);
  const card = bubble.children[4].children[0].children[0].children[1].children[0];
  const tmdbTitle = card.children[1].children[0];
  assert.equal(tmdbTitle.tag, 'button');
  await tmdbTitle.listeners.click();
  assert.deepEqual(requests, [
    '/v2/flow', '/v2/media/tmdb/movie/123?stream=1', '/v2/media/tmdb/movie/123/discover'
  ]);
  const detailPanel = findByClass(bubble, 'catalog-modal');
  const detail = findByClass(detailPanel, 'catalog-detail');
  const sourceCards = findAllByClass(detail, 'catalog-source-card');
  assert.deepEqual(sourceCards.map(source => source.dataset.sourceId),
    ['plex', 'phimapi']);
  const phimapiCard = sourceCards.find(source => source.dataset.sourceId === 'phimapi');
  assert.match(findByClass(phimapiCard, 'catalog-source-warning').textContent,
    /AI đã mở rộng truy vấn/);
  assert.match(findByClass(detail, 'catalog-source-heading').children[1].textContent,
    /2\/3 nguồn/);
});

test('media detail layout uses safe poster URLs and explains unverified source matches', () => {
  const { context } = environment([]);
  const content = new Element('div');
  context.catalogRenderMediaDetail({
    title: { title: 'Movie', year: 2020, kind: 'movie', poster: 'javascript:alert(1)' },
    sources: [{ source_id: 'phimapi', status: 'ok', items: [
      { title: 'Movie', poster: 'https://images.example/poster.jpg',
        match_method: 'title_year', actions: [] }
    ] }]
  }, content, new AbortController().signal);
  const detail = findByClass(content, 'catalog-detail');
  assert.equal(findAllByClass(detail, 'catalog-detail-poster').length, 0);
  assert.equal(findAllByClass(detail, 'catalog-media-item-poster').length, 0);
});

test('AI-discovered results remain visibly unverified', () => {
  const { context } = environment([]);
  const content = new Element('div');
  context.catalogRenderMediaDetail({
    title: { title: 'One Piece', year: 1999, kind: 'show' },
    sources: [{ source_id: 'phimapi', status: 'ok',
      discovery_method: 'ai_query_expansion', items: [
        { title: 'ワンピース', match_method: 'ai_candidate', actions: [] }
      ] }]
  }, content, new AbortController().signal);
  const detail = findByClass(content, 'catalog-detail');
  assert.match(findByClass(detail, 'catalog-source-warning').textContent,
    /AI đã mở rộng truy vấn/);
});

test('AI discovery is skipped when PhimAPI already has an ID match', () => {
  const { context } = environment([]);
  assert.equal(context.catalogMediaNeedsAIDiscovery({ sources: [
    { source_id: 'phimapi', status: 'ok', items: [{ match_method: 'external_id' }] }
  ] }), false);
  assert.equal(context.catalogMediaNeedsAIDiscovery({ sources: [
    { source_id: 'phimapi', status: 'ok', items: [{ match_method: 'title_year' }] }
  ] }), true);
});

test('media detail surfaces partial alias search failures alongside matched sources', () => {
  const { context } = environment([]);
  const content = new Element('div');
  context.catalogRenderMediaDetail({
    title: { title: 'Series', year: 2020, kind: 'show' },
    sources: [{ source_id: 'phimapi', status: 'ok', error_code: 'partial_search_failed',
      items: [{ title: 'Series', match_method: 'external_id', actions: [] }] }]
  }, content, new AbortController().signal);
  const warning = findByClass(content, 'catalog-source-warning');
  assert.equal(warning.textContent, 'Một số truy vấn tên thay thế không hoàn tất.');
});

function findByClass(root, className) {
  if (root.className && root.className.split(/\s+/).includes(className)) return root;
  for (const child of root.children) {
    const found = findByClass(child, className);
    if (found) return found;
  }
  return null;
}

function findAllByClass(root, className) {
  const found = [];
  if (root.className && root.className.split(/\s+/).includes(className)) found.push(root);
  for (const child of root.children) found.push(...findAllByClass(child, className));
  return found;
}

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
    { type: 'task_result', result: {
      source: 'youtube', status: 'skipped', error_code: 'not_configured', items: []
    } },
    { type: 'response_completed', status: 'unavailable' }
  ]);
  const bubble = new Element('div');
  await context.trySearchFlow('tìm phim', bubble, timer);
  assert.match(bubble.children[1].children[0].textContent, /Nguồn chưa được hỗ trợ/);
  assert.match(bubble.children[1].children[1].textContent, /YOUTUBE_API_KEY/);
  assert.deepEqual(bubble.children[3].children.map(tab => tab.textContent), ['Tất cả (0)']);
  assert.equal(bubble.children[4].children.length, 1);
  assert.equal(bubble.children[4].children[0].children.length, 0);
});

test('media RAW opens the complete unfiltered JSON response in a separate tab', async () => {
  const { context, opened, blobs } = environment([]);
  const content = new Element('div');
  const response = {
    title: { title: '<script>unsafe</script>', kind: 'movie', external_ids: { tmdb: '317442' } },
    sources: [
      { source_id: 'plex', status: 'error', error_code: 'search_failed', items: [] },
      { source_id: 'sonarr', status: 'skipped', items: [] }
    ],
    diagnostic: { nested: ['complete', 42] }
  };
  context.catalogRenderMediaDetail(response, content, new AbortController().signal);
  const raw = findByClass(content, 'catalog-raw-button');
  assert.equal(raw.textContent, 'RAW');
  raw.listeners.click();
  assert.deepEqual(opened[0], ['blob:raw-response', '_blank', 'noopener']);
  assert.equal(blobs[0].type, 'application/json');
  assert.equal(await blobs[0].text(), JSON.stringify(response, null, 2));
});

test('unconnected or skipped sources like sonarr/radarr are excluded from media detail view', () => {
  const { context } = environment([]);
  const content = new Element('div');
  const response = {
    title: { title: 'Naruto', kind: 'show', external_ids: { tvdb: '12345' } },
    sources: [
      { source_id: 'sonarr', status: 'skipped', error_code: 'not_configured', items: [] },
      { source_id: 'phimapi', status: 'ok', items: [{ title: 'Naruto Ep 1', match_method: 'title' }] }
    ]
  };
  context.catalogRenderMediaDetail(response, content, new AbortController().signal);
  const detail = findByClass(content, 'catalog-detail');
  const sourceCards = findAllByClass(detail, 'catalog-source-card');
  assert.equal(sourceCards.length, 1);
  assert.equal(sourceCards[0].dataset.sourceId, 'phimapi');
});

test('multiple items under a provider render distinct item titles for seasons', () => {
  const { context } = environment([]);
  const content = new Element('div');
  const response = {
    title: { title: 'Loki', kind: 'show' },
    sources: [
      { source_id: 'phimapi', status: 'ok', items: [
        { title: 'Loki', kind: 'show', season: 1, year: 2021, actions: [{ type: 'catalog_detail', label: 'Chọn tập' }] },
        { title: 'Loki', kind: 'show', season: 2, year: 2023, actions: [{ type: 'catalog_detail', label: 'Chọn tập' }] }
      ] }
    ]
  };
  context.catalogRenderMediaDetail(response, content, new AbortController().signal);
  const detail = findByClass(content, 'catalog-detail');
  const itemTitles = findAllByClass(detail, 'catalog-item-title');
  assert.equal(itemTitles.length, 2);
  assert.equal(itemTitles[0].textContent, 'Loki · Phần 1');
  assert.equal(itemTitles[1].textContent, 'Loki · Phần 2');
});
