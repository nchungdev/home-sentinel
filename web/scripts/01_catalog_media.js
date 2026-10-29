function catalogRenderMediaDetail(data, content, signal) {
  const detail = catalogNode('article');
  detail.className = 'catalog-detail';
  detail.appendChild(catalogRenderMediaHero(data.title));

  const sourceResults = data.sources || [];
  const foundCount = sourceResults.filter(result => (result.items || []).length > 0).length;
  const sourceHeader = catalogNode('div');
  sourceHeader.className = 'catalog-source-heading';
  sourceHeader.append(
    catalogNode('h3', 'Nguồn nội dung'),
    catalogNode('span', foundCount
      ? foundCount + '/' + sourceResults.length + ' nguồn có kết quả'
      : 'Chưa tìm thấy trong các nguồn')
  );

  const sources = catalogNode('div');
  sources.className = 'catalog-media-sources';
  for (const result of sourceResults) {
    sources.appendChild(catalogRenderMediaSource(result, signal));
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

function catalogRenderMediaSource(result, signal) {
  const items = result.items || [];
  const section = catalogNode('section');
  section.className = 'catalog-source-card' +
    (items.length ? ' catalog-source-card--found' : '');
  section.dataset.sourceId = result.source_id;

  const header = catalogNode('div');
  header.className = 'catalog-source-card-heading';
  const sourceName = catalogNode('h4', catalogSourceName(result.source_id));
  const badge = catalogNode('span', items.length ? 'Có kết quả' : 'Không có kết quả');
  badge.className = 'catalog-source-badge' +
    (items.length ? ' catalog-source-badge--found' : '');
  header.append(sourceName, badge);
  section.appendChild(header);

  if (result.discovery_method === 'ai_query_expansion') {
    const notice = catalogNode('p',
      'AI đã mở rộng truy vấn; kết quả cần được xác minh ' +
      'bằng ID hoặc tên/năm.');
    notice.className = 'catalog-source-warning';
    section.appendChild(notice);
  }
  if (result.error_code === 'partial_search_failed') {
    const warning = catalogNode('p', 'Một số truy vấn tên thay thế không hoàn tất.');
    warning.className = 'catalog-source-warning';
    section.appendChild(warning);
  }

  if (!items.length) {
    const status = catalogNode('p', catalogSourceStatus(result));
    status.className = 'catalog-source-status';
    section.appendChild(status);
    return section;
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

function catalogSourceStatus(result) {
  if (result.error_code === 'identity_not_verified') {
    return result.source_id === 'phimapi'
      ? 'Không có ID khớp hoặc ứng viên tên và năm duy nhất.'
      : 'Không thể xác minh kết quả bằng ID media.';
  }
  if (result.error_code === 'ai_no_new_queries') {
    return 'AI không tìm được tên thay thế mới để thử.';
  }
  if (result.error_code === 'partial_search_failed') {
    return 'Một số truy vấn tên không hoàn tất; chưa xác minh được kết quả.';
  }
  return {
    not_configured: 'Nguồn chưa được cấu hình.',
    unreachable: 'Không thể kết nối với nguồn.',
    auth_required: 'Nguồn yêu cầu xác thực.',
    timeout: 'Nguồn phản hồi quá lâu.',
    source_unavailable: 'Nguồn hiện không khả dụng.',
  }[result.error_code] || 'Không tìm thấy media phù hợp.';
}

function catalogRenderMediaItems(items, container, signal) {
  for (const item of items) {
    const entry = catalogNode('article');
    entry.className = 'catalog-media-item';
    const poster = catalogMediaImage(item.poster, item.title, 'catalog-media-item-poster');
    if (poster) entry.appendChild(poster);

    const information = catalogNode('div');
    information.className = 'catalog-media-item-information';
    information.appendChild(catalogNode('strong', item.title));
    if (item.match_method === 'title_year') {
      const notice = catalogNode('small', 'Ứng viên theo tên/năm · ID chưa xác minh');
      notice.className = 'catalog-media-match-notice';
      information.appendChild(notice);
    } else if (item.match_method === 'ai_candidate') {
      const notice = catalogNode('small', 'AI tìm được ứng viên · chưa xác minh');
      notice.className = 'catalog-media-match-notice';
      information.appendChild(notice);
    } else if (item.match_method === 'external_id') {
      const notice = catalogNode('small', 'Đã khớp ID media');
      notice.className = 'catalog-media-match-verified';
      information.appendChild(notice);
    }
    if (item.summary) information.appendChild(catalogNode('p', item.summary));
    catalogRenderMediaActions(item.actions || [], information, signal);
    entry.appendChild(information);
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
  try {
    await catalogJSON('/api/media/request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source_id: action.source_id, item_id: action.item_id,
        seasons: action.season ? [action.season] : [] }),
      signal
    });
    button.textContent = 'Đã gửi yêu cầu';
  } catch (error) {
    if (error.name !== 'AbortError') button.textContent = error.message;
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
