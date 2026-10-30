    // MEDIA SERVICES CONFIG & DISCOVERY
    async function loadMediaServicesConfig() {
      try {
        const res = await fetch('/api/media/config');
        const data = await res.json();
        if (data.ok && data.config) {
          const cfg = data.config;
          if (cfg.tmdb_api_key) document.getElementById('cfgTmdbApiKey').value = cfg.tmdb_api_key;
          if (cfg.youtube_api_key) document.getElementById('cfgYoutubeApiKey').value = cfg.youtube_api_key;
          if (cfg.plex_url) document.getElementById('cfgPlexUrl').value = cfg.plex_url;
          if (cfg.plex_token) document.getElementById('cfgPlexToken').value = cfg.plex_token;
          if (cfg.jellyfin_url) document.getElementById('cfgJellyfinUrl').value = cfg.jellyfin_url;
          if (cfg.jellyfin_api_key) document.getElementById('cfgJellyfinApiKey').value = cfg.jellyfin_api_key;
          if (cfg.radarr_url) document.getElementById('cfgRadarrUrl').value = cfg.radarr_url;
          if (cfg.radarr_api_key) document.getElementById('cfgRadarrApiKey').value = cfg.radarr_api_key;
          if (cfg.sonarr_url) document.getElementById('cfgSonarrUrl').value = cfg.sonarr_url;
          if (cfg.sonarr_api_key) document.getElementById('cfgSonarrApiKey').value = cfg.sonarr_api_key;
          if (cfg.searxng_url) document.getElementById('cfgSearxngUrl').value = cfg.searxng_url;
        }
        await discoverMediaServices();
      } catch (e) {
        console.error('Failed to load media services config', e);
      }
    }

    async function discoverMediaServices() {
      const container = document.getElementById('mediaDiscoveryList');
      if (!container) return;
      container.innerHTML = '<div style="padding:14px; text-align:center; color:var(--text-muted); font-size:13px; grid-column:1/-1;">Đang kiểm tra kết nối tới các dịch vụ…</div>';
      try {
        const res = await fetch('/api/media/sources?refresh=1');
        const data = await res.json();
        if (!data.ok || !data.sources) throw new Error('Không lấy được trạng thái dịch vụ');

        container.innerHTML = '';
        const statusLabels = {
          ready: { text: 'Sẵn sàng (Ready)', color: '#10b981', icon: 'check_circle', bg: 'rgba(16,185,129,0.12)' },
          auth_required: { text: 'Cần API Key / Token', color: '#f59e0b', icon: 'key', bg: 'rgba(245,158,11,0.12)' },
          not_configured: { text: 'Chưa cấu hình', color: '#6b7280', icon: 'remove_circle_outline', bg: 'rgba(107,114,128,0.12)' },
          unreachable: { text: 'Không kết nối được', color: '#ef4444', icon: 'error_outline', bg: 'rgba(239,68,68,0.12)' },
          invalid_service: { text: 'Lỗi phản hồi dịch vụ', color: '#ef4444', icon: 'warning', bg: 'rgba(239,68,68,0.12)' }
        };

        for (const source of data.sources) {
          const info = statusLabels[source.status] || { text: source.status, color: '#a5a7b3', icon: 'info', bg: 'rgba(255,255,255,0.05)' };
          const card = document.createElement('div');
          card.style.cssText = `padding:12px 14px; border-radius:10px; background:${info.bg}; border:1px solid ${info.color}40; display:flex; align-items:center; justify-content:space-between; gap:10px;`;

          const left = document.createElement('div');
          left.style.cssText = 'display:flex; align-items:center; gap:8px;';
          const name = document.createElement('strong');
          name.style.cssText = 'color:var(--text-bright); font-size:14px;';
          name.textContent = source.name || source.id;
          left.appendChild(name);

          const right = document.createElement('div');
          right.style.cssText = `display:flex; align-items:center; gap:5px; color:${info.color}; font-size:12px; font-weight:600;`;
          right.innerHTML = `<span class="mi" style="font-size:16px;">${info.icon}</span> ${info.text}`;

          card.append(left, right);
          container.appendChild(card);
        }
      } catch (e) {
        container.innerHTML = `<div style="padding:14px; text-align:center; color:#ef4444; font-size:13px; grid-column:1/-1;">Lỗi: ${e.message}</div>`;
      }
    }

    async function saveMediaServicesConfig() {
      const btn = document.getElementById('btnSaveMediaConfig');
      const status = document.getElementById('mediaConfigStatus');
      if (btn) btn.disabled = true;
      if (status) status.innerHTML = '<span style="color:var(--text-muted);">Đang lưu cấu hình và kiểm tra kết nối dịch vụ…</span>';

      const body = {
        tmdb_api_key: document.getElementById('cfgTmdbApiKey').value.trim(),
        youtube_api_key: document.getElementById('cfgYoutubeApiKey').value.trim(),
        plex_url: document.getElementById('cfgPlexUrl').value.trim(),
        plex_token: document.getElementById('cfgPlexToken').value.trim(),
        jellyfin_url: document.getElementById('cfgJellyfinUrl').value.trim(),
        jellyfin_api_key: document.getElementById('cfgJellyfinApiKey').value.trim(),
        radarr_url: document.getElementById('cfgRadarrUrl').value.trim(),
        radarr_api_key: document.getElementById('cfgRadarrApiKey').value.trim(),
        sonarr_url: document.getElementById('cfgSonarrUrl').value.trim(),
        sonarr_api_key: document.getElementById('cfgSonarrApiKey').value.trim(),
        searxng_url: document.getElementById('cfgSearxngUrl').value.trim()
      };

      try {
        const res = await fetch('/api/media/config', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        });
        const data = await res.json();
        if (data.ok) {
          if (status) status.innerHTML = '<span style="color:#10b981;"><span class="mi" style="font-size:15px;">check_circle</span> Đã lưu cấu hình dịch vụ thành công!</span>';
          await discoverMediaServices();
        } else {
          if (status) status.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi lưu cấu hình: ${data.error || 'Thất bại'}</span>`;
        }
      } catch (e) {
        if (status) status.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi: ${e.message}</span>`;
      } finally {
        if (btn) btn.disabled = false;
      }
    }
