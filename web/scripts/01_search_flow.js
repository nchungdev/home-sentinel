    let searchFlowTitle = '';

    function flowLink(value) {
      try {
        const parsed = new URL(value);
        return ['http:', 'https:'].includes(parsed.protocol) && !parsed.username &&
          !parsed.password ? parsed.href : null;
      } catch (_) {
        return null;
      }
    }

    function flowSourceLabel(source) {
      return { internet: 'Internet', plex: 'Plex', jellyfin: 'Jellyfin', youtube: 'YouTube',
        radarr: 'Radarr', sonarr: 'Sonarr', tmdb: 'TMDb', official_docs: 'Tài liệu',
        phimapi: 'PhimAPI' }[source] || source;
    }

    function flowErrorLabel(code, source) {
      if (source === 'youtube' && code === 'not_configured') {
        return 'Cần cấu hình YOUTUBE_API_KEY trên máy chủ';
      }
      return { not_configured: 'Chưa cấu hình nguồn', auth_required: 'Kiểm tra tài khoản, quyền truy cập và cấu hình API',
        not_implemented: 'Nguồn chưa được hỗ trợ', skill_unavailable: 'Chức năng chưa được hỗ trợ',
        source_unavailable: 'Nguồn chưa được hỗ trợ', timeout: 'Nguồn phản hồi quá lâu',
        unreachable: 'Không kết nối được tới nguồn (kiểm tra mạng/DNS của máy chủ)',
        invalid_service: 'Nguồn trả về dữ liệu không hợp lệ' }[code] ||
        'Chưa lấy được dữ liệu từ nguồn';
    }

    const CONTENT_TYPES = {
      movie: 'Phim', series: 'Series', videos: 'Videos', other: 'Khác'
    };

    function flowContentType(item) {
      const kind = (item.kind || '').toLowerCase();
      if (['movie', 'film'].includes(kind)) return 'movie';
      if (['show', 'series', 'season', 'episode', 'tv'].includes(kind)) return 'series';
      if (['video', 'clip', 'clips'].includes(kind)) return 'videos';
      if (['wiki', 'web', 'web_link', 'information', 'article'].includes(kind)) return 'other';
      return 'other';
    }

    function createContentTabs() {
      const bar = document.createElement('div');
      bar.className = 'search-flow-tabs';
      bar.setAttribute('role', 'tablist');
      const panels = document.createElement('div');
      panels.className = 'search-flow-groups';
      const entries = [];
      function select(entry) {
        for (const other of entries) {
          const active = other === entry;
          other.tab.setAttribute('aria-selected', String(active));
          other.tab.tabIndex = active ? 0 : -1;
          other.panel.hidden = !active;
        }
      }
      function move(entry, step) {
        const next = entries[(entries.indexOf(entry) + step + entries.length) % entries.length];
        select(next);
        next.tab.focus();
      }
      function ensure(contentType) {
        const existing = entries.find(entry => entry.contentType === contentType);
        if (existing) return existing;

        const panel = document.createElement('div');
        panel.className = 'search-flow-content-panel';
        panels.appendChild(panel);
        const tab = document.createElement('button');
        const entry = { tab, panel };
        tab.type = 'button';
        tab.setAttribute('role', 'tab');
        entry.contentType = contentType;
        entry.count = 0;
        tab.textContent = (contentType === 'all' ? 'Tất cả' :
          CONTENT_TYPES[contentType] || contentType) + ' (0)';
        tab.setAttribute('aria-selected', 'false');
        tab.tabIndex = -1;
        panel.setAttribute('role', 'tabpanel');
        tab.addEventListener('click', () => select(entry));
        tab.addEventListener('keydown', event => {
          if (event.key === 'ArrowRight') move(entry, 1);
          else if (event.key === 'ArrowLeft') move(entry, -1);
          else return;
          event.preventDefault();
        });
        entries.push(entry);
        bar.appendChild(tab);
        select(entries.find(candidate =>
          candidate.tab.getAttribute('aria-selected') === 'true') || entry);
        return entry;
      }
      function add(contentType, source, item, createCard) {
        const entry = ensure(contentType);
        if (!entry.sections) entry.sections = new Map();
        const sectionEntry = entry.sections.get(source);
        let section = sectionEntry?.section;
        let row = sectionEntry?.row;
        if (!row) {
          section = document.createElement('section');
          const heading = document.createElement('h4');
          row = document.createElement('div');
          row.className = 'search-flow-results';
          heading.textContent = flowSourceLabel(source);
          section.append(heading, row);
          entry.sections.set(source, { section, row });
          entry.panel.appendChild(section);
        }
        row.appendChild(createCard(item, section));
        entry.count++;
        entry.tab.textContent = (contentType === 'all' ? 'Tất cả' :
          CONTENT_TYPES[contentType] || contentType) + ' (' + entry.count + ')';
      }
      ensure('all');
      return { bar, panels, add };
    }

    async function trySearchFlow(text, bubble, timer) {
      const response = await fetch('/v2/flow', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text, context: { title: searchFlowTitle } })
      });
      if (response.status === 404) return false;
      if (!response.ok) throw new Error('Không thực hiện được yêu cầu (' + response.status + ')');
      if ((response.headers.get('content-type') || '').includes('application/json')) {
        const result = await response.json();
        if (result.status === 'legacy_route') return false;
        throw new Error('Response không hợp lệ');
      }

      const summary = document.createElement('div');
      const tabs = createContentTabs();
      const viewSwitch = document.createElement('div');
      viewSwitch.className = 'search-flow-view-switch';
      viewSwitch.setAttribute('role', 'group');
      viewSwitch.setAttribute('aria-label', 'Chế độ kết quả');
      const resultButton = document.createElement('button');
      resultButton.type = 'button';
      resultButton.textContent = 'Result';
      resultButton.setAttribute('aria-pressed', 'true');
      viewSwitch.appendChild(resultButton);
      const rawButton = document.createElement('button');
      rawButton.type = 'button';
      rawButton.textContent = 'Raw';
      rawButton.disabled = true;
      rawButton.setAttribute('aria-pressed', 'false');
      viewSwitch.appendChild(rawButton);
      const rawEvents = [];
      rawButton.addEventListener('click', () => {
        if (!rawEvents.length) return;
        const rawURL = URL.createObjectURL(new Blob(
          [JSON.stringify(rawEvents, null, 2)], { type: 'application/json' }));
        window.open(rawURL, '_blank', 'noopener');
        setTimeout(() => URL.revokeObjectURL(rawURL), 60_000);
      });
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '', initialized = false, total = 0, finalText = '', terminal = false;
      let checked = 0;
      const sourceStatus = document.createElement('div');
      sourceStatus.className = 'search-flow-status';
      const statusLines = new Map();
      function setSourceStatus(source, text) {
        let line = statusLines.get(source);
        if (!line) {
          line = document.createElement('p');
          statusLines.set(source, line);
          sourceStatus.appendChild(line);
        }
        line.textContent = flowSourceLabel(source) + ': ' + text;
      }
      function clearSourceStatus(source) {
        const line = statusLines.get(source);
        if (line) line.remove();
        statusLines.delete(source);
      }
      function initialize() {
        if (initialized) return;
        initialized = true;
        timer.stop();
        bubble.replaceChildren(summary, sourceStatus, viewSwitch, tabs.bar, tabs.panels);
      }
      function accept(event) {
        initialize();
        rawEvents.push(event);
        if (event.type === 'plan_ready') {
          const understanding = event.plan.understanding;
          if (understanding.needs_input) {
            searchFlowTitle = text;
            finalText = understanding.question;
            summary.textContent = finalText;
          } else {
            if (understanding.entities.title) searchFlowTitle = understanding.entities.title;
            summary.textContent = 'Đang tìm từ các nguồn phù hợp…';
          }
        }
        if (event.type === 'plan_ready') {
          for (const task of event.plan.tasks || []) setSourceStatus(task.source, 'đang tìm…');
        }
        if (event.type === 'task_result') {
          const result = event.result;
          if (result.error_code) {
            setSourceStatus(result.source, flowErrorLabel(result.error_code, result.source));
          }
          else clearSourceStatus(result.source);
          if (result.status === 'ok' || result.status === 'empty') checked++;
          if (!result.items.length) return;
          function createCard(item, detailHost) {
            const card = document.createElement('div');
            const contentType = flowContentType(item);
            card.className = 'search-flow-item' +
              (contentType === 'videos' ? ' search-flow-item--video' : '');
            const poster = flowLink(item.poster);
            if (poster) {
              const image = document.createElement('img');
              image.className = 'search-flow-poster';
              image.src = poster;
              image.alt = item.title;
              image.loading = 'lazy';
              image.referrerPolicy = 'no-referrer';
              card.appendChild(image);
            } else {
              const placeholder = document.createElement('div');
              placeholder.className = 'search-flow-poster search-flow-poster--empty';
              placeholder.setAttribute('aria-hidden', 'true');
              card.appendChild(placeholder);
            }
            const details = document.createElement('div');
            details.className = 'search-flow-item-details';
            const destination = flowLink(item.url);
            const detailAction = (item.actions || []).find(action =>
              action.type === 'catalog_detail' || action.type === 'media_detail');
            const titleTag = detailAction ? 'button' : destination ? 'a' : 'strong';
            const title = document.createElement(titleTag);
            title.className = 'search-flow-item-title';
            title.textContent = item.title;
            if (detailAction) {
              title.type = 'button';
              title.addEventListener('click', () => detailAction.type === 'media_detail' ?
                catalogOpenMedia(detailAction.item_id, title, detailHost) :
                catalogOpenTitle(detailAction.item_id, title, detailHost));
            } else if (destination) {
              title.href = destination;
              title.target = '_blank';
              title.rel = 'noopener noreferrer';
            }
            const metadata = document.createElement('p');
            metadata.className = 'search-flow-item-meta';
            metadata.textContent = [item.year || '', item.content || ''].filter(Boolean).join(' · ');
            details.appendChild(title);
            if (metadata.textContent) details.appendChild(metadata);
            card.appendChild(details);
            return card;
          }
          for (const item of result.items) {
            tabs.add('all', result.source, item, createCard);
            tabs.add(flowContentType(item), result.source, item, createCard);
            total++;
          }
          summary.textContent = 'Đã nhận ' + total + ' kết quả; đang kiểm tra các nguồn còn lại…';
        }
        if (event.type === 'response_completed') {
          terminal = true;
          rawButton.disabled = false;
          if (event.status !== 'needs_input') {
            finalText = total ? 'Tìm được ' + total + ' kết quả, chia theo loại nội dung.' :
              checked ? 'Không tìm thấy kết quả trong các nguồn đã kiểm tra.' :
              statusLines.size ? 'Chưa truy cập được nguồn tìm kiếm. Bạn có thể thử lại.' :
              'Chưa có nguồn tìm kiếm khả dụng. Hãy cấu hình nguồn rồi thử lại.';
            summary.textContent = finalText;
          }
        }
      }
      while (true) {
        const { done, value } = await reader.read();
        buffer += done ? decoder.decode() : decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop();
        for (const line of lines) if (line.trim()) accept(JSON.parse(line));
        if (done) break;
      }
      if (buffer.trim()) accept(JSON.parse(buffer));
      if (!terminal) throw new Error('Kết nối kết thúc trước khi hoàn tất tìm kiếm');
      const history = getChatHistory();
      history.push({ sender: 'agent', text: finalText });
      saveChatHistory(history);
      addBubbleMeta(bubble, { route: 'search', intent: 'source.search' }, timer.seconds());
      return true;
    }
