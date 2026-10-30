    const catalogPlaybackRequests = new WeakMap();

    function catalogNode(tag, text) {
      const node = document.createElement(tag);
      if (text) node.textContent = text;
      return node;
    }

    function catalogError(code) {
      return {
        catalog_item_not_found: 'Phim hoặc tập không còn trên nguồn.',
        episode_playback_unsupported: 'Tập này chưa có nguồn phát trực tiếp được hỗ trợ.',
        catalog_timeout: 'Nguồn phản hồi quá lâu. Hãy thử lại.',
        catalog_source_unavailable: 'Chưa truy cập được nguồn phim.',
        ai_search_unavailable: 'AI tìm kiếm chưa được cấu hình.',
        ai_search_failed: 'AI không thể gợi ý thêm tên tìm kiếm.',
        ai_search_timeout: 'AI tìm kiếm phản hồi quá lâu. Hãy thử lại.',
        ai_invalid_response: 'AI trả về gợi ý không hợp lệ.'
      }[code] || 'Chưa lấy được dữ liệu phim. Hãy thử lại.';
    }

    async function catalogJSON(url, options) {
      const response = await fetch(url, options);
      const data = await response.json();
      if (!response.ok) throw new Error(catalogError(data.error));
      return data;
    }

    function catalogOpenModal(trigger, host) {
      const overlay = catalogNode('div');
      overlay.className = 'catalog-modal-overlay';
      overlay.setAttribute('role', 'dialog');
      overlay.setAttribute('aria-modal', 'true');

      const modal = catalogNode('div');
      modal.className = 'catalog-modal';

      const header = catalogNode('header');
      header.className = 'catalog-modal-header';

      const close = catalogNode('button', '✕ Quay lại');
      close.type = 'button';
      close.className = 'catalog-modal-close';
      close.setAttribute('aria-label', 'Đóng cửa sổ');
      header.appendChild(close);

      const content = catalogNode('div');
      content.className = 'catalog-modal-body';

      modal.append(header, content);
      overlay.appendChild(modal);

      const controller = new AbortController();
      const bodyNode = (typeof document !== 'undefined' && document.body) || host || null;
      const originalOverflow = bodyNode && bodyNode.style ? bodyNode.style.overflow : '';
      if (bodyNode && bodyNode.style) bodyNode.style.overflow = 'hidden';

      function destroy() {
        controller.abort();
        overlay.remove();
        if (bodyNode && bodyNode.style) bodyNode.style.overflow = originalOverflow;
        if (trigger) {
          trigger.disabled = false;
          if (typeof trigger.focus === 'function') trigger.focus();
        }
      }

      close.addEventListener('click', destroy);
      overlay.addEventListener('click', event => {
        if (event.target === overlay) destroy();
      });
      const onKeyDown = event => {
        if (event.key === 'Escape') destroy();
      };
      if (typeof document !== 'undefined' && document.addEventListener) {
        document.addEventListener('keydown', onKeyDown);
      }
      controller.signal.addEventListener('abort', () => {
        if (typeof document !== 'undefined' && document.removeEventListener) {
          document.removeEventListener('keydown', onKeyDown);
        }
      });

      if (bodyNode) bodyNode.appendChild(overlay);
      close.focus();

      return { overlay, content, controller, close: destroy };
    }

    async function catalogOpenTitle(id, trigger, detailHost) {
      if (trigger) trigger.disabled = true;
      const modal = catalogOpenModal(trigger, detailHost);
      modal.content.textContent = 'Đang lấy danh sách tập…';
      try {
        const data = await catalogJSON('/v2/catalog/titles/' + encodeURIComponent(id),
          { signal: modal.controller.signal });
        searchFlowTitle = data.title.name;
        catalogRenderTitle(data.title, modal.content, modal.controller.signal);
      } catch (error) {
        if (error.name !== 'AbortError') modal.content.textContent = error.message;
      }
    }

    async function catalogOpenMedia(identifier, trigger, detailHost) {
      const match = /^(movie|tv)\/([1-9]\d*)$/.exec(identifier || '');
      if (!match) return;
      if (trigger) trigger.disabled = true;
      const modal = catalogOpenModal(trigger, detailHost);
      modal.content.textContent = 'Đang lấy chi tiết và kiểm tra nguồn…';
      try {
        const data = await catalogJSON('/v2/media/tmdb/' + match[1] + '/' + match[2],
          { signal: modal.controller.signal });
        searchFlowTitle = data.title.title;
        catalogRenderMediaDetail(data, modal.content, modal.controller.signal);
        if (catalogMediaNeedsAIDiscovery(data)) {
          await catalogDiscoverMedia(match[1], match[2], data, modal.content, modal.controller.signal);
        }
      } catch (error) {
        if (error.name !== 'AbortError') modal.content.textContent = error.message;
      }
    }

    async function catalogDiscoverMedia(kind, id, data, content, signal) {
      const status = catalogNode('p', 'Chưa xác minh được ID. ' +
        'AI đang thử thêm tên tìm kiếm…');
      status.className = 'catalog-discovery-status';
      content.appendChild(status);
      try {
        const response = await catalogJSON('/v2/media/tmdb/' + kind + '/' + id + '/discover', {
          method: 'POST', signal
        });
        if (signal.aborted) return;
        const sources = [...(data.sources || [])];
        const sourceIndex = sources.findIndex(source => source.source_id === 'phimapi');
        if (sourceIndex < 0) sources.push(response.source);
        else sources[sourceIndex] = response.source;
        catalogRenderMediaDetail({ ...data, sources }, content, signal);
      } catch (error) {
        if (error.name !== 'AbortError') status.textContent = error.message;
      }
    }

    function catalogRenderTitle(title, content, signal) {
      const heading = catalogNode('strong', title.name);
      heading.className = 'catalog-title-heading';
      const groups = [...new Set(title.episodes.map(episode => episode.group))];
      let selectedGroup = groups[0] || '';

      const serverBar = catalogNode('div');
      serverBar.className = 'catalog-server-sources';

      const groupSelect = catalogNode('select');
      groupSelect.setAttribute('aria-label', 'Nguồn phát');
      if (groupSelect.style) groupSelect.style.display = 'none';

      const episodeList = catalogNode('div');
      episodeList.className = 'catalog-episodes';
      const previous = catalogNode('button', 'Trang trước');
      const next = catalogNode('button', 'Trang sau');
      const status = catalogNode('span');
      const playback = catalogNode('div');
      playback.className = 'catalog-playback';
      let page = 0;

      function renderServerButtons() {
        serverBar.replaceChildren();
        if (groups.length > 1) {
          const label = catalogNode('span', 'Nguồn server: ');
          label.className = 'catalog-server-label';
          serverBar.appendChild(label);
          for (const group of groups) {
            const btn = catalogNode('button', group || 'Nguồn mặc định');
            btn.type = 'button';
            btn.className = 'catalog-server-btn' + (group === selectedGroup ? ' catalog-server-btn--active' : '');
            btn.addEventListener('click', () => {
              selectedGroup = group;
              groupSelect.value = group;
              page = 0;
              render();
            });
            serverBar.appendChild(btn);
          }
        }
      }

      function render() {
        renderServerButtons();
        const episodes = title.episodes.filter(episode => episode.group === selectedGroup);
        const pages = Math.max(1, Math.ceil(episodes.length / 25));
        episodeList.replaceChildren();
        for (const episode of episodes.slice(page * 25, (page + 1) * 25)) {
          const button = catalogNode('button', episode.label);
          button.type = 'button';
          button.disabled = !episode.playable;
          button.addEventListener('click', () => catalogResolveEpisode(episode, playback, signal));
          episodeList.appendChild(button);
        }
        if (!episodes.length) episodeList.textContent = 'Nguồn chưa có tập nào.';
        status.textContent = 'Trang ' + (page + 1) + '/' + pages + ' · ' + episodes.length + ' tập';
        previous.disabled = page === 0;
        next.disabled = page + 1 >= pages;
      }
      previous.type = next.type = 'button';
      function focusNavigation() {
        if (!next.disabled) next.focus();
        else if (!previous.disabled) previous.focus();
      }
      previous.addEventListener('click', () => { page--; render(); focusNavigation(); });
      next.addEventListener('click', () => { page++; render(); focusNavigation(); });
      groupSelect.addEventListener('change', () => { page = 0; render(); });
      const navigation = catalogNode('div');
      navigation.className = 'catalog-pagination';
      navigation.append(previous, status, next);
      content.replaceChildren(heading, serverBar, groupSelect, episodeList, navigation, playback);
      render();
    }

    async function catalogResolveEpisode(episode, panel, signal) {
      if (signal.aborted) return;
      catalogPlaybackRequests.get(panel)?.abort();
      const request = new AbortController();
      catalogPlaybackRequests.set(panel, request);
      const onClose = () => request.abort();
      signal.addEventListener('abort', onClose, { once: true });
      panel.textContent = 'Đang lấy nguồn phát cho ' + episode.label + '…';
      try {
        const data = await catalogJSON('/v2/catalog/episodes/' +
          encodeURIComponent(episode.id) + '/playback', {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: '{}', signal: request.signal
        });
        if (request.signal.aborted) return;
        if (!data.playback.url.startsWith('/media/stream/')) {
          throw new Error('Link phát không hợp lệ.');
        }
        const streamURL = new URL(data.playback.url, window.location.origin).href;
        const name = catalogNode('strong', data.playback.title);
        const hint = catalogNode('p', 'Mở link này bằng VLC hoặc trình phát hỗ trợ HLS.');
        const input = catalogNode('input');
        input.type = 'text';
        input.readOnly = true;
        input.value = streamURL;
        input.setAttribute('aria-label', 'Link phát');
        const copy = catalogNode('button', 'Sao chép link phát');
        copy.type = 'button';
        copy.addEventListener('click', async () => {
          try {
            await navigator.clipboard.writeText(streamURL);
            copy.textContent = 'Đã sao chép';
          } catch (_) { input.focus(); input.select(); }
        });
        panel.replaceChildren(name, hint, input, copy);
        if (/Android/i.test(navigator.userAgent)) {
          const link = catalogNode('a', 'Mở bằng VLC (nếu đã cài)');
          const parsed = new URL(streamURL);
          link.href = 'intent://' + parsed.host + parsed.pathname + '#Intent;scheme=' +
            parsed.protocol.slice(0, -1) + ';type=' + data.playback.mime_type +
            ';package=org.videolan.vlc;end';
          panel.appendChild(link);
        }
      } catch (error) {
        if (error.name !== 'AbortError') panel.textContent = error.message;
      } finally {
        signal.removeEventListener('abort', onClose);
        if (catalogPlaybackRequests.get(panel) === request) catalogPlaybackRequests.delete(panel);
      }
    }

function catalogOpenRawJSON(data) {
  const rawURL = URL.createObjectURL(new Blob(
    [JSON.stringify(data, null, 2)], { type: 'application/json' }));
  window.open(rawURL, '_blank', 'noopener');
  setTimeout(() => URL.revokeObjectURL(rawURL), 60_000);
}
