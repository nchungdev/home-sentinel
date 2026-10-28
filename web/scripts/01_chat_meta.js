    const CHAT_INTENT_LABELS = {
      'system.status': 'trạng thái NAS',
      'media.search': 'phim',
      'home.control': 'thiết bị nhà',
      'camera.control': 'camera',
      'general.chat': 'trò chuyện'
    };

    function chatHandlerLabel(handler) {
      if (!handler) return '';
      if (handler.route === 'ai') return 'AI · ' + (handler.engine || 'chưa chọn engine');
      return 'Luật · ' + (CHAT_INTENT_LABELS[handler.intent] || handler.intent);
    }

    function startChatTimer(bubble) {
      const startedAt = performance.now();
      const timer = setInterval(() => {
        const seconds = Math.floor((performance.now() - startedAt) / 1000);
        bubble.textContent = 'Đang xử lý… ' + seconds + 's';
      }, 1000);
      return {
        stop() { clearInterval(timer); },
        seconds() { return ((performance.now() - startedAt) / 1000).toFixed(1); }
      };
    }

    function addBubbleMeta(bubble, handler, seconds) {
      const meta = document.createElement('div');
      meta.className = 'bubble-meta';
      const label = chatHandlerLabel(handler);
      meta.textContent = (label ? label + ' · ' : '') + seconds.replace('.', ',') + ' giây';
      bubble.appendChild(meta);
    }

    function webClientID() {
      const key = 'sentinel_web_client_id';
      let id = '';
      try { id = localStorage.getItem(key) || ''; } catch (_) {}
      if (!id) {
        id = 'web-' + Math.random().toString(36).slice(2, 10);
        try { localStorage.setItem(key, id); } catch (_) {}
      }
      return id;
    }

    function webRequestContext() {
      return JSON.stringify({ client_id: webClientID(), client_capabilities: ['chat'] });
    }
