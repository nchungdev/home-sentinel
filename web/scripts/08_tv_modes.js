    const TVM_DOMAINS = [
      'light', 'switch', 'fan', 'climate', 'media_player', 'cover', 'scene', 'script',
    ];
    const TVM_DOMAIN_LABELS = {
      light: 'Đèn',
      switch: 'Công tắc',
      fan: 'Quạt',
      climate: 'Điều hòa',
      media_player: 'Media player',
      cover: 'Rèm / cửa',
      scene: 'Cảnh',
      script: 'Script',
    };
    const TVM_EXTRA = {
      light: { key: 'brightness_pct', label: 'Độ sáng %' },
      fan: { key: 'percentage', label: 'Tốc độ %' },
      climate: { key: 'temperature', label: 'Nhiệt độ °C' },
    };
    const tvModes = { config: { modes: [], auto_on_playback: false }, entities: [] };

    async function loadTvModes() {
      tvmSay('Đang tải…');
      try {
        const [configRes, entityRes] = await Promise.all([
          fetch('/api/tv-modes/config'),
          fetch('/api/ha/entities'),
        ]);
        if (!configRes.ok || !entityRes.ok) {
          throw new Error(`HTTP ${configRes.status}/${entityRes.status}`);
        }
        tvModes.config = await configRes.json();
        const entityData = await entityRes.json();
        tvModes.config.modes.forEach(tvmGroupActions);
        tvModes.entities = (entityData.entities || [])
          .filter(entity => TVM_DOMAINS.includes(entity.domain))
          .sort((a, b) => (a.name || a.entity_id).localeCompare(b.name || b.entity_id));
        renderTvModes();
        tvmSay(tvModes.entities.length ? '' :
          'Chưa đọc được thiết bị từ Home Assistant');
      } catch (error) {
        tvmSay('Không tải được: ' + error.message);
      }
    }

    function tvmGroupActions(mode) {
      const groups = new Map();
      (mode.actions || []).forEach(action => {
        const domain = String(action.entity || '').split('.')[0];
        const key = `${domain}|${action.service}|${JSON.stringify(action.data || {})}`;
        if (!groups.has(key)) {
          groups.set(key, {
            domain,
            entities: [],
            service: action.service,
            data: action.data,
          });
        }
        groups.get(key).entities.push(action.entity);
      });
      mode.actions = [...groups.values()];
    }

    function renderTvModes() {
      const list = document.getElementById('tvmList');
      list.innerHTML = '';
      if (!tvModes.config.modes.length) {
        list.innerHTML = '<div class="tvm-empty">Chưa có chế độ nào.</div>';
        return;
      }
      tvModes.config.modes.forEach((mode, index) => list.appendChild(tvmModeCard(mode, index)));
    }

    function tvmModeCard(mode, index) {
      const card = document.createElement('div');
      card.className = 'tvm-card';
      const active = tvModes.config.active === mode.id ?
        '<span class="tvm-active">Đang bật</span>' : '';
      const domainOptions = TVM_DOMAINS.map(domain =>
        `<option value="${domain}">${TVM_DOMAIN_LABELS[domain]}</option>`).join('');
      card.innerHTML = `
        <div class="tvm-head">
          <input class="tvm-icon" maxlength="4" value="${tvmEscape(mode.icon || '')}">
          <input class="tvm-name" maxlength="40" placeholder="Tên chế độ"
            value="${tvmEscape(mode.name)}">
          ${active}
          <button class="btn-modern" data-run>
            <span class="mi">play_arrow</span> Chạy thử
          </button>
          <button class="btn-modern" data-remove><span class="mi">delete</span></button>
        </div>
        <input class="tvm-desc" maxlength="60"
          placeholder="Mô tả ngắn hiện dưới nút trên TV"
          value="${tvmEscape(mode.description || '')}">
        <div class="tvm-actions"></div>
        <div class="tvm-add-row">
          <select class="tvm-new-domain">${domainOptions}</select>
          <button class="btn-modern tvm-add">
            <span class="mi">add</span> Thêm nhóm thiết bị
          </button>
        </div>`;
      card.querySelector('.tvm-icon').oninput = event => { mode.icon = event.target.value; };
      card.querySelector('.tvm-name').oninput = event => { mode.name = event.target.value; };
      card.querySelector('.tvm-desc').oninput = event => { mode.description = event.target.value; };
      card.querySelector('[data-run]').onclick = () => runTvMode(mode.id);
      card.querySelector('[data-remove]').onclick = () => {
        tvModes.config.modes.splice(index, 1);
        renderTvModes();
      };
      card.querySelector('.tvm-add').onclick = () => {
        mode.actions.push({
          domain: card.querySelector('.tvm-new-domain').value,
          entities: [],
          service: 'turn_off',
        });
        renderTvModes();
      };
      const groups = card.querySelector('.tvm-actions');
      (mode.actions || []).forEach((action, actionIndex) => {
        groups.appendChild(tvmActionGroup(mode, action, actionIndex));
      });
      return card;
    }

    function addTvMode() {
      const id = 'mode_' + Date.now().toString(36);
      tvModes.config.modes.push({
        id,
        name: 'Chế độ mới',
        icon: '✨',
        description: '',
        actions: [],
      });
      renderTvModes();
    }

    async function saveTvModes() {
      const config = {
        ...tvModes.config,
        modes: tvModes.config.modes.map(mode => ({
          ...mode,
          actions: (mode.actions || []).flatMap(action => action.entities.map(entity => ({
            entity,
            service: action.service,
            ...(action.data ? { data: action.data } : {}),
          }))),
        })),
      };
      tvmSay('Đang lưu…');
      try {
        const res = await fetch('/api/tv-modes/config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(config),
        });
        const result = await res.json();
        if (!res.ok || !result.ok) {
          return tvmSay('Không lưu được: ' + (result.error || res.status));
        }
        tvModes.config = result.config;
        tvModes.config.modes.forEach(tvmGroupActions);
        renderTvModes();
        tvmSay('Đã lưu, TV sẽ cập nhật ngay');
      } catch (error) {
        tvmSay('Không lưu được: ' + error.message);
      }
    }

    async function runTvMode(id) {
      tvmSay('Đang chạy…');
      try {
        const res = await fetch('/v2/modes/' + encodeURIComponent(id) + '/activate', {
          method: 'POST',
        });
        const result = await res.json();
        if (!res.ok) return tvmSay('Không chạy được: ' + (result.error || res.status));
        tvModes.config.active = result.active;
        renderTvModes();
        tvmSay(result.warning ? 'Đã chạy, nhưng ' + result.warning : 'Đã chạy');
      } catch (error) {
        tvmSay('Không chạy được: ' + error.message);
      }
    }

    function tvmSay(text) {
      document.getElementById('tvmStatus').textContent = text;
    }

    function tvmEscape(text) {
      return String(text).replace(/[&<>"']/g, character => ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;',
      })[character]);
    }
