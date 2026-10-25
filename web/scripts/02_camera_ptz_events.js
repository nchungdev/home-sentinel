    async function loadClientLogs() {
      const container = document.getElementById('clientLogsContainer');
      if (!container) return;

      try {
        const res = await fetch('/api/logs/client');
        const data = await res.json();
        if (!data.ok || !data.logs || data.logs.length === 0) {
          container.innerHTML = '<div style="color:var(--text-muted); text-align:center; padding:30px;">Chưa có tin nhắn nào từ Client được gửi lên server.</div>';
          return;
        }

        let html = '';
        // Hiển thị tin mới nhất trước (reverse)
        const reversed = [...data.logs].reverse();
        reversed.forEach(item => {
          const isWeb = item.source === 'web';
          const badgeClass = isWeb ? 'web' : 'tv';
          const badgeText = isWeb ? 'WEB UI' : 'CLIENT / TV';
          html += `
            <div class="client-log-item">
              <div class="client-log-header">
                <span class="client-badge ${badgeClass}">${badgeText}</span>
                <span style="color:#6b7280;">[${escapeHtml(item.timestamp || '')}]</span>
                <span style="color:#9ca3af;">IP: ${escapeHtml(item.client_ip || 'unknown')}</span>
                <span style="color:#6b7280;">Session: ${escapeHtml(item.session || 'none')}</span>
              </div>
              <div class="client-log-prompt"><span class="mi" style="font-size:14px; color:var(--accent);">chat_bubble</span> ${escapeHtml(item.text || '')}</div>
              <div class="client-log-resp"><span class="mi" style="font-size:14px; color:#10b981;">smart_toy</span> ${escapeHtml(item.response || '(đang phản hồi hoặc trống)')}</div>
            </div>
          `;
        });
        container.innerHTML = html;
      } catch (e) {
        container.innerHTML = `<div style="color:#ef4444; text-align:center; padding:20px;">Lỗi tải logs: ${e.message}</div>`;
      }
    }

    async function clearServerClientLogs() {
      if (!confirm('Bạn có chắc muốn xóa danh sách Client Message Logs trên server?')) return;
      try {
        await fetch('/api/logs/client', { method: 'DELETE' });
        loadClientLogs();
      } catch (e) {
        alert('Lỗi: ' + e.message);
      }
    }

    function escapeHtml(str) {
      if (!str) return '';
      return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
    }

    // CAMERA LOGS & REAL-TIME EVENT STREAM
    function initCameraEventStream() {
      const log = document.getElementById('termLog');
      if (!log || window.camEventSource) return;

      try {
        const es = new EventSource('/api/camera/events');
        window.camEventSource = es;

        es.onmessage = function(e) {
          if (!e.data || e.data === 'CONNECTED') return;
          const time = new Date().toLocaleTimeString();
          log.textContent = `${log.textContent}\n${e.data}`.split('\n').slice(-300).join('\n');
          log.scrollTop = log.scrollHeight;
        };

        es.onerror = function() {
          // Tự động reconnect bởi trình duyệt
        };
      } catch (err) {
        console.error('Không thể kết nối EventSource camera:', err);
      }
    }

    async function callCamera(type, data) {
      const log = document.getElementById('termLog');
      const time = new Date().toLocaleTimeString();
      log.textContent += `\n[${time}] Gửi ${type}: ${JSON.stringify(data)}`;

      try {
        const url = type === 'ptz' ? '/api/camera/ptz' : '/api/camera/privacy';
        const res = await fetch(url, {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify(data)
        });
        const json = await res.json();
        log.textContent += `\n ➔ Phản hồi: ${JSON.stringify(json)}`;
      } catch (e) {
        log.textContent += `\n ➔ Lỗi: ${e.message}`;
      }
      log.scrollTop = log.scrollHeight;
    }

    // GỬI LỜI NÓI / PHÁT LOA CAMERA
    const camVoiceInput = document.getElementById('camVoiceInput');
    if (camVoiceInput) {
      camVoiceInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
          e.preventDefault();
          sendCameraSpeech();
        }
      });
    }

    const SVG_ICONS = {
      mic: `<svg viewBox="0 0 24 24"><path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/><path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/></svg>`,
      stop: `<svg viewBox="0 0 24 24"><path d="M6 6h12v12H6z"/></svg>`,
      phone: `<svg viewBox="0 0 24 24"><path d="M20.01 15.38c-1.23 0-2.42-.2-3.53-.56a.977.977 0 0 0-1.01.24l-2.2 2.2a15.053 15.053 0 0 1-6.59-6.59l2.2-2.21a.96.96 0 0 0 .25-1A11.36 11.36 0 0 1 8.5 4c0-.55-.45-1-1-1H4c-.55 0-1 .45-1 1 0 9.39 7.61 17 17 17 .55 0 1-.45 1-1v-3.62c0-.55-.45-1-1-1z"/></svg>`,
      phoneEnd: `<svg viewBox="0 0 24 24"><path d="M12 9c-1.6 0-3.15.25-4.6.72v3.1c0 .39-.23.74-.56.9-.98.49-1.87 1.12-2.66 1.85-.18.18-.43.28-.7.28-.28 0-.53-.11-.71-.29L.29 13.08a.996.996 0 0 1 0-1.41C3.28 8.91 7.42 7.5 12 7.5s8.72 1.41 11.71 4.17c.39.39.39 1.02 0 1.41l-2.48 2.48c-.18.18-.43.29-.71.29-.27 0-.52-.11-.7-.28-.79-.74-1.69-1.36-2.67-1.85-.33-.16-.56-.5-.56-.9v-3.1C15.15 9.25 13.6 9 12 9z"/></svg>`,
      send: `<svg viewBox="0 0 24 24"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>`,
      loading: `<svg viewBox="0 0 24 24" class="spin"><path d="M12 4V2C6.48 2 2 6.48 2 12h2c0-4.41 3.59-8 8-8zm0 14c-4.41 0-8-3.59-8-8H2c0 5.52 4.48 10 10 10v-2zm8-6h2c0-5.52-4.48-10-10-10v2c4.41 0 8 3.59 8 8z"/></svg>`
    };

    async function sendCameraSpeech(customText) {
      const input = document.getElementById('camVoiceInput');
      const text = (customText || (input ? input.value : '')).trim();
      if (!text) return;

      const log = document.getElementById('termLog');
      const btn = document.getElementById('btnCamSpeak');
      const time = new Date().toLocaleTimeString();

      // Cập nhật ngay tin nhắn lên giao diện đàm thoại
      const idleView = document.getElementById('intercomIdle');
      const waveView = document.getElementById('intercomActiveWave');
      const transcriptEl = document.getElementById('liveTranscriptText');
      const timerText = document.getElementById('liveCallTimerText');
      if (idleView) idleView.style.display = 'none';
      if (waveView) waveView.classList.add('visible');
      if (transcriptEl) transcriptEl.textContent = `"${text}"`;
      if (timerText) timerText.textContent = 'Đang phát qua loa...';

      if (btn) {
        btn.disabled = true;
        btn.innerHTML = SVG_ICONS.loading;
      }
      log.textContent += `\n[${time}] Phát giọng nói tới camera: "${text}"`;

      try {
        const res = await fetch('/api/camera/speak', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ text: text })
        });
        const json = await res.json();
        if (json.ok) {
          log.textContent += `\n ➔ Thành công: ${json.message}`;
          if (input && !customText) input.value = '';
          if (timerText) timerText.textContent = '✅ Đã phát xong';
          setTimeout(() => {
            if (!isCalling && !isRecordingVoice) {
              if (idleView) idleView.style.display = 'flex';
              if (waveView) waveView.classList.remove('visible');
            }
          }, 3500);
        } else {
          log.textContent += `\n ➔ Thất bại: ${json.error}`;
          if (timerText) timerText.textContent = '❌ Lỗi phát loa';
        }
      } catch (e) {
        log.textContent += `\n ➔ Lỗi phát âm: ${e.message}`;
        if (timerText) timerText.textContent = '❌ Lỗi kết nối';
      } finally {
        if (btn) {
          btn.disabled = false;
          btn.innerHTML = SVG_ICONS.send;
        }
        log.scrollTop = log.scrollHeight;
      }
    }

    // TÍNH NĂNG 1: GIỌNG NÓI (VOICE / STT)
    let speechRecognition = null;
    let isRecordingVoice = false;

