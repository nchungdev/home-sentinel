function catalogRenderMediaDetail(data, content, signal) {
  const detail = catalogNode('article');
  detail.className = 'catalog-detail';
  detail.appendChild(catalogRenderMediaHero(data.title));

  const kind = data.title?.kind;
  const sourceResults = (data.sources || []).filter(result => {
    if (kind === 'movie' && result.source_id === 'sonarr') return false;
    if (kind === 'show' && result.source_id === 'radarr') return false;
    return true;
  });

  const foundCount = sourceResults.filter(result => (result.items || []).length > 0).length;
  const sourceHeader = catalogNode('div');
  sourceHeader.className = 'catalog-source-heading';
  sourceHeader.append(
    catalogNode('h3', 'Nguồn nội dung'),
    catalogNode('span', foundCount
      ? foundCount + '/' + sourceResults.length + ' nguồn có kết quả'
      : 'Chưa tìm thấy trong các nguồn')
  );

  const rawButton = catalogNode('button', 'RAW');
  rawButton.type = 'button';
  rawButton.className = 'catalog-raw-button';
  rawButton.title = 'Xem toàn bộ JSON phản hồi';
  rawButton.addEventListener('click', () => catalogOpenRawJSON(data));
  sourceHeader.appendChild(rawButton);

  const sources = catalogNode('div');
  sources.className = 'catalog-media-sources';
  for (const result of sourceResults) {
    const card = catalogRenderMediaSource(result, signal, data.title);
    if (card) {
      sources.appendChild(card);
    }
  }
  detail.append(sourceHeader, sources);
  content.replaceChildren(detail);
}

function catalogMediaNeedsAIDiscovery(data) {
  const phimAPI = (data.sources || []).find(result => result.source_id === 'phimapi');
  if (!phimAPI || (phimAPI.items || []).some(item => item.match_method === 'external_id')) {
    return false;
  }
  return phimAPI.status === 'empty' || phimAPI.status === 'ok';
}

function catalogRenderMediaHero(title) {
  const hero = catalogNode('header');
  hero.className = 'catalog-detail-hero';
  const poster = catalogMediaImage(title.poster, title.title, 'catalog-detail-poster');
  if (poster) hero.appendChild(poster);

  const information = catalogNode('div');
  information.className = 'catalog-detail-information';
  const metadata = [title.year, title.kind === 'show' ? 'Phim bộ' : 'Phim lẻ']
    .filter(Boolean).join(' · ');
  information.append(
    catalogNode('p', metadata || 'Thông tin phim'),
    catalogNode('h2', title.title),
    catalogNode('p', title.overview || 'Chưa có nội dung giới thiệu.')
  );
  hero.appendChild(information);

  return hero;
}

function catalogRenderMediaSource(result, signal, title) {
  const items = result.items || [];

  if (!items.length) {
    if (result.source_id === 'radarr') {
      const tmdbID = title?.external_ids?.tmdb || (title?.id || '').replace(/^(movie|tv)\//, '');
      if (tmdbID) {
        const section = catalogNode('section');
        section.className = 'catalog-source-card';
        section.dataset.sourceId = result.source_id;

        const header = catalogNode('div');
        header.className = 'catalog-source-card-heading';
        header.appendChild(catalogNode('h4', catalogSourceName(result.source_id)));
        section.appendChild(header);

        const btn = catalogNode('button', '➕ Thêm vào Radarr');
        btn.type = 'button';
        btn.className = 'catalog-request-btn';
        btn.addEventListener('click', () => catalogRequestMedia({
          source_id: 'radarr', item_id: String(tmdbID)
        }, btn, signal));
        section.appendChild(btn);
        return section;
      }
    } else if (result.source_id === 'sonarr') {
      const tvdbID = title?.external_ids?.tvdb || title?.external_ids?.tmdb || (title?.id || '').replace(/^(movie|tv)\//, '');
      if (tvdbID) {
        const section = catalogNode('section');
        section.className = 'catalog-source-card';
        section.dataset.sourceId = result.source_id;

        const header = catalogNode('div');
        header.className = 'catalog-source-card-heading';
        header.appendChild(catalogNode('h4', catalogSourceName(result.source_id)));
        section.appendChild(header);

        const btn = catalogNode('button', '➕ Thêm vào Sonarr');
        btn.type = 'button';
        btn.className = 'catalog-request-btn';
        btn.addEventListener('click', () => catalogRequestMedia({
          source_id: 'sonarr', item_id: String(tvdbID), season: 1
        }, btn, signal));
        section.appendChild(btn);
        return section;
      }
    }
    return null;
  }

  const section = catalogNode('section');
  section.className = 'catalog-source-card catalog-source-card--found';
  section.dataset.sourceId = result.source_id;

  const header = catalogNode('div');
  header.className = 'catalog-source-card-heading';
  header.appendChild(catalogNode('h4', catalogSourceName(result.source_id)));
  section.appendChild(header);

  if (result.discovery_method === 'ai_query_expansion') {
    const notice = catalogNode('p',
      'AI đã mở rộng truy vấn; kết quả cần được xác minh.');
    notice.className = 'catalog-source-warning';
    section.appendChild(notice);
  }
  if (result.error_code === 'partial_search_failed') {
    const warning = catalogNode('p', 'Một số truy vấn tên thay thế không hoàn tất.');
    warning.className = 'catalog-source-warning';
    section.appendChild(warning);
  }

  catalogRenderMediaItems(items, section, signal);
  return section;
}

function catalogSourceName(source) {
  return {
    plex: 'Plex',
    jellyfin: 'Jellyfin',
    radarr: 'Radarr',
    sonarr: 'Sonarr',
    phimapi: 'PhimAPI'
  }[source] || source;
}

function catalogRenderMediaItems(items, container, signal) {
  for (const item of items) {
    const entry = catalogNode('div');
    entry.className = 'catalog-media-item-simple';

    if (item.kind === 'season' && item.title) {
      const seasonTitle = catalogNode('strong', item.title);
      seasonTitle.className = 'catalog-season-title';
      entry.appendChild(seasonTitle);
    }

    catalogRenderMediaActions(item.actions || [], entry, signal);
    container.appendChild(entry);
  }
}

function catalogMediaImage(source, title, className) {
  if (!source) return null;
  let destination;
  try {
    destination = new URL(source, window.location.origin);
  } catch (_) {
    return null;
  }
  if (!['http:', 'https:'].includes(destination.protocol)) return null;

  const image = catalogNode('img');
  image.className = className;
  image.src = destination.href;
  image.alt = 'Poster: ' + title;
  image.loading = 'lazy';
  return image;
}

function catalogRenderMediaActions(actions, container, signal) {
  const actionGroup = catalogNode('div');
  actionGroup.className = 'catalog-media-actions';
  for (const action of actions) {
    if (action.type === 'catalog_detail') {
      const button = catalogNode('button', action.label || 'Xem phim / Chọn tập');
      button.type = 'button';
      button.className = 'catalog-media-action-primary';
      button.addEventListener('click', () => catalogOpenTitle(action.item_id, button, container));
      actionGroup.appendChild(button);
      continue;
    }
    if (action.type === 'request_library') {
      const button = catalogNode('button', action.label);
      button.type = 'button';
      button.addEventListener('click', () => catalogRequestMedia(action, button, signal));
      actionGroup.appendChild(button);
      continue;
    }
    if (action.type === 'browse') {
      const button = catalogNode('button', action.label);
      button.type = 'button';
      button.addEventListener('click', () => catalogBrowseMedia(action, container, signal));
      actionGroup.appendChild(button);
      continue;
    }
    if (action.type !== 'open_url' && action.type !== 'open_app') continue;
    let destination;
    try {
      destination = new URL(action.url);
    } catch (_) {
      continue;
    }
    const safeWebURL = ['http:', 'https:'].includes(destination.protocol);
    const safePlexURL = action.type === 'open_app' && destination.protocol === 'plex:';
    if (!safeWebURL && !safePlexURL) continue;
    const link = catalogNode('a', action.label);
    link.href = destination.href;
    if (safeWebURL) {
      link.target = '_blank';
      link.rel = 'noopener noreferrer';
    }
    actionGroup.appendChild(link);
  }
  if (actionGroup.children.length) container.appendChild(actionGroup);
}

async function catalogRequestMedia(action, button, signal) {
  button.disabled = true;
  const originalText = button.textContent;
  button.textContent = 'Đang gửi yêu cầu…';
  try {
    await catalogJSON('/api/media/request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source_id: action.source_id, item_id: action.item_id,
        seasons: action.season ? [action.season] : [] }),
      signal
    });
    button.textContent = '✓ Đã gửi yêu cầu thêm';
  } catch (error) {
    if (error.name !== 'AbortError') button.textContent = error.message;
    else button.textContent = originalText;
    button.disabled = false;
  }
}

async function catalogBrowseMedia(action, container, signal) {
  const status = catalogNode('p', 'Đang tải danh sách…');
  container.appendChild(status);
  try {
    const query = new URLSearchParams({ source_id: action.source_id, item_id: action.item_id });
    const response = await fetch('/api/media/browse?' + query, { signal });
    const data = await response.json();
    if (!response.ok || !data.ok) throw new Error(data.error || 'Không tải được danh sách.');
    status.remove();
    catalogRenderMediaItems(data.items || [], container, signal);
  } catch (error) {
    if (error.name !== 'AbortError') status.textContent = error.message;
  }
}
