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

    function flowErrorLabel(code) {
      return { not_configured: 'Chưa cấu hình nguồn', auth_required: 'Kiểm tra tài khoản, quyền truy cập và cấu hình API',
        not_implemented: 'Nguồn chưa được hỗ trợ', skill_unavailable: 'Chức năng chưa được hỗ trợ',
        source_unavailable: 'Nguồn chưa được hỗ trợ', timeout: 'Nguồn phản hồi quá lâu' }[code] ||
        'Chưa lấy được dữ liệu từ nguồn';
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
      const groups = document.createElement('div');
      groups.className = 'search-flow-groups';
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '', initialized = false, total = 0, finalText = '', terminal = false;
      let checked = 0;
      const failures = [];
      function initialize() {
        if (initialized) return;
        initialized = true;
        timer.stop();
        bubble.replaceChildren(summary, groups);
      }
      function accept(event) {
        initialize();
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
        if (event.type === 'task_result') {
          const result = event.result;
          if (result.status === 'ok' || result.status === 'empty') checked++;
          if (!result.items.length) {
            if (result.error_code) failures.push(result);
            return;
          }
          const section = document.createElement('section');
          const heading = document.createElement('strong');
          heading.textContent = flowSourceLabel(result.source);
          section.appendChild(heading);
          for (const item of result.items) {
            const card = document.createElement('div');
            card.className = 'search-flow-item';
            const destination = flowLink(item.url);
            const detailAction = (item.actions || []).find(action => action.type === 'catalog_detail');
            const title = document.createElement(detailAction ? 'button' : destination ? 'a' : 'strong');
            title.textContent = item.title;
            if (detailAction) {
              title.type = 'button';
              title.addEventListener('click', () => catalogOpenTitle(detailAction.item_id, title, card));
            } else if (destination) {
              title.href = destination;
              title.target = '_blank';
              title.rel = 'noopener noreferrer';
            }
            const content = document.createElement('p');
            content.textContent = item.content || '';
            card.append(title, content);
            section.appendChild(card);
          }
          total += result.items.length;
          groups.appendChild(section);
          summary.textContent = 'Đã nhận ' + total + ' kết quả; đang kiểm tra các nguồn còn lại…';
        }
        if (event.type === 'response_completed') {
          terminal = true;
          if (event.status !== 'needs_input') {
            finalText = total ? 'Tìm được ' + total + ' kết quả, chia theo nguồn.' :
              checked ? 'Không tìm thấy kết quả trong các nguồn đã kiểm tra.' :
              failures.length ? 'Chưa truy cập được nguồn tìm kiếm. Bạn có thể thử lại.' :
              'Chưa có nguồn tìm kiếm khả dụng. Hãy cấu hình nguồn rồi thử lại.';
            if (failures.length) {
              const details = document.createElement('details');
              const label = document.createElement('summary');
              label.textContent = 'Trạng thái ' + failures.length + ' nguồn chưa phản hồi';
              details.appendChild(label);
              for (const failure of failures) {
                const line = document.createElement('p');
                line.textContent = flowSourceLabel(failure.source) + ': ' +
                  flowErrorLabel(failure.error_code);
                details.appendChild(line);
              }
              groups.appendChild(details);
            }
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
