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

    async function catalogOpenTitle(id, trigger, detailHost) {
      trigger.disabled = true;
      const panel = catalogNode('div');
      panel.className = 'catalog-panel';
      const close = catalogNode('button', '‹ Quay lại');
      const content = catalogNode('div', 'Đang lấy danh sách tập…');
      const controller = new AbortController();
      close.type = 'button';
      close.setAttribute('aria-label', 'Quay lại kết quả');
      close.addEventListener('click', () => {
        controller.abort();
        panel.remove();
        trigger.disabled = false;
        trigger.focus();
      });
      panel.append(close, content);
      detailHost.appendChild(panel);
      close.focus();
      try {
        const data = await catalogJSON('/v2/catalog/titles/' + encodeURIComponent(id),
          { signal: controller.signal });
        searchFlowTitle = data.title.name;
        catalogRenderTitle(data.title, content, controller.signal);
      } catch (error) {
        if (error.name !== 'AbortError') content.textContent = error.message;
      }
    }

    async function catalogOpenMedia(identifier, trigger, detailHost) {
      const match = /^(movie|tv)\/([1-9]\d*)$/.exec(identifier || '');
      if (!match) return;
      trigger.disabled = true;
      const panel = catalogNode('div');
      panel.className = 'catalog-panel';
      const close = catalogNode('button', '‹ Quay lại');
      const content = catalogNode('div', 'Đang lấy chi tiết và kiểm tra nguồn…');
      const controller = new AbortController();
      close.type = 'button';
      close.setAttribute('aria-label', 'Quay lại kết quả');
      close.addEventListener('click', () => {
        controller.abort();
        panel.remove();
        trigger.disabled = false;
        trigger.focus();
      });
      panel.append(close, content);
      detailHost.appendChild(panel);
      close.focus();
      try {
        const data = await catalogJSON('/v2/media/tmdb/' + match[1] + '/' + match[2],
          { signal: controller.signal });
        searchFlowTitle = data.title.title;
        catalogRenderMediaDetail(data, content, controller.signal);
        if (catalogMediaNeedsAIDiscovery(data)) {
          await catalogDiscoverMedia(match[1], match[2], data, content, controller.signal);
        }
      } catch (error) {
        if (error.name !== 'AbortError') content.textContent = error.message;
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
      const groups = [...new Set(title.episodes.map(episode => episode.group))];
      const groupSelect = catalogNode('select');
      groupSelect.setAttribute('aria-label', 'Nguồn phát');
      groupSelect.disabled = groups.length < 2;
      for (const group of groups) {
        const option = catalogNode('option', group || 'Nguồn mặc định');
        option.value = group;
        groupSelect.appendChild(option);
      }
      const episodeList = catalogNode('div');
      episodeList.className = 'catalog-episodes';
      const previous = catalogNode('button', 'Trang trước');
      const next = catalogNode('button', 'Trang sau');
      const status = catalogNode('span');
      const playback = catalogNode('div');
      playback.className = 'catalog-playback';
      let page = 0;
      function render() {
        const episodes = title.episodes.filter(episode => episode.group === groupSelect.value);
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
      content.replaceChildren(heading, groupSelect, episodeList, navigation, playback);
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
