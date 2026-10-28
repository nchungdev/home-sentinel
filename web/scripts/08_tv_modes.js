    const TVM_DOMAINS = ['light', 'switch', 'fan', 'climate', 'media_player', 'cover', 'scene', 'script'];
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
        tvModes.config = await configRes.json();
        const entityData = await entityRes.json();
        tvModes.entities = (entityData.entities || [])
          .filter(e => TVM_DOMAINS.includes(e.domain))
          .sort((a, b) => (a.name || a.entity_id).localeCompare(b.name || b.entity_id));
        renderTvModes();
        tvmSay(tvModes.entities.length ? '' : 'Chưa đọc được thiết bị từ Home Assistant');
      } catch (e) {
        tvmSay('Không tải được: ' + e.message);
      }
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
      const active = tvModes.config.active === mode.id ? '<span class="tvm-active">Đang bật</span>' : '';
      card.innerHTML = `
        <div class="tvm-head">
          <input class="tvm-icon" maxlength="4" value="${tvmEscape(mode.icon || '')}">
          <input class="tvm-name" maxlength="40" placeholder="Tên chế độ" value="${tvmEscape(mode.name)}">
          ${active}
          <button class="btn-modern" data-run><span class="mi">play_arrow</span> Chạy thử</button>
          <button class="btn-modern" data-remove><span class="mi">delete</span></button>
        </div>
        <input class="tvm-desc" maxlength="60" placeholder="Mô tả ngắn hiện dưới nút trên TV"
          value="${tvmEscape(mode.description || '')}">
        <div class="tvm-actions"></div>
        <button class="btn-modern tvm-add" data-add><span class="mi">add</span> Thêm thiết bị</button>`;
      card.querySelector('.tvm-icon').oninput = e => { mode.icon = e.target.value; };
      card.querySelector('.tvm-name').oninput = e => { mode.name = e.target.value; };
      card.querySelector('.tvm-desc').oninput = e => { mode.description = e.target.value; };
      card.querySelector('[data-run]').onclick = () => runTvMode(mode.id);
      card.querySelector('[data-remove]').onclick = () => {
        tvModes.config.modes.splice(index, 1);
        renderTvModes();
      };
      card.querySelector('[data-add]').onclick = () => {
        mode.actions = mode.actions || [];
        mode.actions.push({ entity: '', service: 'turn_off' });
        renderTvModes();
      };
      const rows = card.querySelector('.tvm-actions');
      (mode.actions || []).forEach((action, actionIndex) => {
        rows.appendChild(tvmActionRow(mode, action, actionIndex));
      });
      return card;
    }

    function tvmActionRow(mode, action, actionIndex) {
      const row = document.createElement('div');
      row.className = 'tvm-row';
      const options = tvModes.entities.map(e =>
        `<option value="${e.entity_id}" ${e.entity_id === action.entity ? 'selected' : ''}>` +
        `${tvmEscape(e.name || e.entity_id)} (${e.entity_id})</option>`).join('');
      const known = tvModes.entities.some(e => e.entity_id === action.entity);
      const custom = action.entity && !known ?
        `<option value="${tvmEscape(action.entity)}" selected>${tvmEscape(action.entity)}</option>` : '';
      const extra = TVM_EXTRA[action.entity.split('.')[0]];
      const extraValue = extra && action.data ? action.data[extra.key] ?? '' : '';
      row.innerHTML = `
        <select class="tvm-entity"><option value="">Chọn thiết bị…</option>${custom}${options}</select>
        <select class="tvm-service">
          <option value="turn_off" ${action.service === 'turn_off' ? 'selected' : ''}>Tắt</option>
          <option value="turn_on" ${action.service === 'turn_on' ? 'selected' : ''}>Bật</option>
        </select>
        ${extra && action.service === 'turn_on' ?
          `<input class="tvm-extra" type="number" placeholder="${extra.label}" value="${extraValue}">` : ''}
        <button class="btn-modern" data-remove><span class="mi">close</span></button>`;
      row.querySelector('.tvm-entity').onchange = e => {
        action.entity = e.target.value;
        delete action.data;
        renderTvModes();
      };
      row.querySelector('.tvm-service').onchange = e => {
        action.service = e.target.value;
        renderTvModes();
      };
      const extraInput = row.querySelector('.tvm-extra');
      if (extraInput) {
        extraInput.oninput = e => {
          const value = e.target.value === '' ? null : Number(e.target.value);
          action.data = value === null ? undefined : { [extra.key]: value };
        };
      }
      row.querySelector('[data-remove]').onclick = () => {
        mode.actions.splice(actionIndex, 1);
        renderTvModes();
      };
      return row;
    }

    function addTvMode() {
      const id = 'mode_' + Date.now().toString(36);
      tvModes.config.modes.push({ id, name: 'Chế độ mới', icon: '✨', description: '', actions: [] });
      renderTvModes();
    }

    async function saveTvModes() {
      tvModes.config.modes.forEach(mode => {
        mode.actions = (mode.actions || []).filter(action => action.entity);
      });
      tvmSay('Đang lưu…');
      const res = await fetch('/api/tv-modes/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(tvModes.config),
      });
      const result = await res.json();
      if (!result.ok) return tvmSay('Không lưu được: ' + (result.error || res.status));
      tvModes.config = result.config;
      renderTvModes();
      tvmSay('Đã lưu, TV sẽ cập nhật ngay');
    }

    async function runTvMode(id) {
      tvmSay('Đang chạy…');
      const res = await fetch('/v2/modes/' + encodeURIComponent(id) + '/activate', { method: 'POST' });
      const result = await res.json();
      if (!res.ok) return tvmSay('Không chạy được: ' + (result.error || res.status));
      tvModes.config.active = result.active;
      renderTvModes();
      tvmSay(result.warning ? 'Đã chạy, nhưng ' + result.warning : 'Đã chạy');
    }

    function tvmSay(text) {
      document.getElementById('tvmStatus').textContent = text;
    }

    function tvmEscape(text) {
      return String(text).replace(/[&<>"']/g, c => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
      })[c]);
    }
