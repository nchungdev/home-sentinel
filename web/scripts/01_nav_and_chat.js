    let isCameraStreaming = false;

    function toggleCameraPlayback() {
      const img = document.getElementById('mjpegFeed');
      const placeholder = document.getElementById('camPlaceholderOverlay');
      const badge = document.getElementById('camLiveBadge');
      const label = document.getElementById('feedStatusLabel');

      if (!isCameraStreaming) {
        // Bắt đầu phát
        isCameraStreaming = true;
        img.src = '/api/camera/stream.mjpeg?t=' + Date.now();
        img.style.display = 'block';
        placeholder.style.display = 'none';
        badge.style.display = 'flex';
        label.textContent = 'Đang kết nối luồng...';
        label.style.color = '#38bdf8';
      } else {
        // Tắt phát (dừng luồng RTSP giải phóng CPU)
        isCameraStreaming = false;
        img.src = '';
        img.style.display = 'none';
        placeholder.style.display = 'flex';
        badge.style.display = 'none';
        label.textContent = 'Đang tạm dừng (Bấm để xem)';
        label.style.color = 'var(--text-muted)';
      }
    }

    function handleStreamLoaded() {
      if (!isCameraStreaming) return;
      document.getElementById('mjpegFeed').style.display = 'block';
      document.getElementById('camPlaceholderOverlay').style.display = 'none';
      document.getElementById('unconfiguredOverlay').style.display = 'none';
      document.getElementById('feedStatusLabel').textContent = 'Đang truyền trực tiếp (12 FPS)';
      document.getElementById('feedStatusLabel').style.color = '#10b981';
    }

    function handleStreamError() {
      if (!isCameraStreaming) return;
      document.getElementById('mjpegFeed').style.display = 'none';
      document.getElementById('camPlaceholderOverlay').style.display = 'none';
      document.getElementById('unconfiguredOverlay').style.display = 'flex';
      document.getElementById('feedStatusLabel').textContent = 'Chưa kết nối';
      document.getElementById('feedStatusLabel').style.color = '#ef4444';
    }

    function jumpToConfig(subtabId) {
      const navSetting = document.getElementById('nav-setting');
      if (navSetting) {
        switchNav('setting', navSetting);
      }
      const subtabBtn = document.getElementById('btn-subtab-' + subtabId);
      if (subtabBtn) {
        switchSubTab(subtabId, subtabBtn);
      }
    }

    function switchAutoTab(autotabId, btnEl) {
      document.querySelectorAll('#view-automation .auto-tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('#view-automation .auto-tab-panel').forEach(p => p.classList.remove('active'));

      if (btnEl) btnEl.classList.add('active');
      const target = document.getElementById('autotab-' + autotabId);
      if (target) target.classList.add('active');

      try {
        localStorage.setItem('sentinel_active_autotab', autotabId);
      } catch (e) {}
    }

    function switchSubTab(subtabId, btnEl) {
      document.querySelectorAll('#view-setting .subtab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('#view-setting .subtab-panel').forEach(p => p.classList.remove('active'));

      if (btnEl) btnEl.classList.add('active');
      const target = document.getElementById('subtab-' + subtabId);
      if (target) target.classList.add('active');

      try {
        localStorage.setItem('sentinel_active_subtab', subtabId);
      } catch (e) {}

      if (subtabId === 'agent-cfg') loadEngines();
      if (subtabId === 'automation-cfg') loadAutomationConfigAndDevices();
      if (subtabId === 'logs') loadClientLogs();
    }

    function switchNav(name, el) {
      document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
      document.querySelectorAll('.view-content').forEach(v => v.classList.remove('active'));

      if (el) el.classList.add('active');
      const target = document.getElementById('view-' + name);
      if (target) target.classList.add('active');

      const titles = {
        'agent': 'Agent Chat',
        'camera': 'Camera Live & PTZ',
        'automation': 'Kịch Bản Tự Động Hóa (Automation)',
        'setting': 'Settings'
      };
      document.getElementById('viewHeaderTitle').textContent = titles[name] || 'Home Sentinel';

      try {
        localStorage.setItem('sentinel_active_nav', name);
      } catch (e) {}

      if (name === 'camera') {
        initCameraEventStream();
        loadDetectionStates();
      }

      if (name === 'automation') {
        loadAutomationConfigAndDevices();
      }

      if (name === 'setting') {
        const savedSubtab = localStorage.getItem('sentinel_active_subtab') || 'agent-cfg';
        const subtabBtn = document.getElementById('btn-subtab-' + savedSubtab);
        if (subtabBtn) {
          switchSubTab(savedSubtab, subtabBtn);
        }
      }
    }

    // CHAT & HISTORY (LocalStorage)
    const promptInput = document.getElementById('promptInput');
    const chatList = document.getElementById('chatList');
    const CHAT_STORAGE_KEY = 'sentinel_chat_history_v1';

    promptInput.addEventListener('keydown', (e) => {
      if (e.isComposing || e.keyCode === 229) return;
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendChat();
      }
    });

    function quickSend(t) {
      promptInput.value = t;
      sendChat();
    }

    function getChatHistory() {
      try {
        const raw = localStorage.getItem(CHAT_STORAGE_KEY);
        return raw ? JSON.parse(raw) : [];
      } catch (e) {
        return [];
      }
    }

    function saveChatHistory(history) {
      try {
        if (history.length > 50) history = history.slice(history.length - 50);
        localStorage.setItem(CHAT_STORAGE_KEY, JSON.stringify(history));
      } catch (e) {}
    }

    function renderSavedChat() {
      const history = getChatHistory();
      if (!history || history.length === 0) return;

      // Giữ tin nhắn chào ban đầu và append các tin nhắn cũ
      history.forEach(item => {
        appendBubble(item.sender, item.text, false);
      });
    }

    function clearChatHistory() {
      if (!confirm('Bạn có chắc muốn xóa toàn bộ lịch sử đoạn chat này không?')) return;
      try {
        localStorage.removeItem(CHAT_STORAGE_KEY);
      } catch (e) {}
      chatList.innerHTML = `
        <div class="message-row agent">
          <div class="avatar ai">AI</div>
          <div class="bubble">Xin chào! Tôi là Trợ lý Home Sentinel. Bạn có thể chọn AI engine trong Cài đặt để bắt đầu trò chuyện. Bạn cần tôi hỗ trợ gì?</div>
        </div>
      `;
    }

    let chatInFlight = false;

    async function sendChat() {
      const text = promptInput.value.trim();
      if (!text || chatInFlight) return;
      chatInFlight = true;

      appendBubble('user', text, true);
      promptInput.value = '';

      const agentBubble = appendBubble('agent', 'Đang xử lý… 0s', false);
      const timer = startChatTimer(agentBubble);
      let handler = null;

      try {
        if (await trySearchFlow(text, agentBubble, timer)) return;
        const res = await fetch('/v2/ask', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Request-Context': webRequestContext()
          },
          body: JSON.stringify({
            request_id: 'web-' + Date.now(),
            session_id: 'web_session',
            text: text,
            locale: 'vi-VN',
            input_type: 'text'
          })
        });

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let fullAnswer = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop();
          for (const line of lines) {
            if (!line.trim()) continue;
            try {
              const event = JSON.parse(line);
              if (event.type === 'presentation') {
                timer.stop();
                handler = event.payload.handler || null;
                fullAnswer = event.payload.speech_text || event.payload.text || '';
                agentBubble.textContent = fullAnswer || 'Đã xử lý xong yêu cầu.';
              } else if (event.type === 'error') {
                timer.stop();
                fullAnswer = 'Lỗi: ' + (event.payload.message || 'Không thể xử lý');
                agentBubble.textContent = fullAnswer;
              }
            } catch (_) {}
          }
          chatList.scrollTop = chatList.scrollHeight;
        }

        timer.stop();
        if (!fullAnswer || !agentBubble.textContent.trim()) {
          fullAnswer = agentBubble.textContent.trim() || 'Tôi đã nhận được yêu cầu của bạn.';
          agentBubble.textContent = fullAnswer;
        }

        const history = getChatHistory();
        history.push({ sender: 'agent', text: fullAnswer });
        saveChatHistory(history);
        addBubbleMeta(agentBubble, handler, timer.seconds());
      } catch (err) {
        timer.stop();
        agentBubble.textContent = 'Lỗi kết nối: ' + err.message;
      } finally {
        chatInFlight = false;
      }
    }

    function appendBubble(sender, text, shouldSave = false) {
      const row = document.createElement('div');
      row.className = 'message-row ' + sender;

      const avatar = document.createElement('div');
      avatar.className = 'avatar ' + (sender === 'user' ? 'user' : 'ai');
      avatar.textContent = sender === 'user' ? 'U' : 'AI';

      const bubble = document.createElement('div');
      bubble.className = 'bubble';
      bubble.textContent = text;

      row.appendChild(avatar);
      row.appendChild(bubble);

      chatList.appendChild(row);
      chatList.scrollTop = chatList.scrollHeight;

      if (shouldSave) {
        const history = getChatHistory();
        history.push({ sender: sender, text: text });
        saveChatHistory(history);
      }

      return bubble;
    }

    // CLIENT LOGS CONSOLE (TV Launcher & Clients)
