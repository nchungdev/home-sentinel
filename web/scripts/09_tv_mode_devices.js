    function tvmActionGroup(mode, action, actionIndex) {
      const group = document.createElement('section');
      group.className = 'tvm-action-group';
      const domainEntities = tvModes.entities.filter(entity => entity.domain === action.domain);
      const knownEntities = new Set(domainEntities.map(entity => entity.entity_id));
      const missingEntities = (action.entities || [])
        .filter(entityId => !knownEntities.has(entityId))
        .map(entityId => ({ entity_id: entityId, name: entityId, state: 'không khả dụng' }));
      const entities = [...domainEntities, ...missingEntities];
      const extra = TVM_EXTRA[action.domain];
      const extraValue = extra && action.data ? action.data[extra.key] ?? '' : '';
      const domainLabel = TVM_DOMAIN_LABELS[action.domain] || action.domain;
      group.innerHTML = `
        <div class="tvm-action-controls">
          <strong>${tvmEscape(domainLabel)}</strong>
          <select class="tvm-service">
            <option value="turn_off" ${action.service === 'turn_off' ? 'selected' : ''}>
              Tắt
            </option>
            <option value="turn_on" ${action.service === 'turn_on' ? 'selected' : ''}>Bật</option>
          </select>
          ${extra && action.service === 'turn_on' ?
            `<input class="tvm-extra" type="number" placeholder="${extra.label}"
              value="${tvmEscape(extraValue)}">` : ''}
          <button class="btn-modern" data-remove title="Xóa nhóm thiết bị">
            <span class="mi">delete</span>
          </button>
        </div>
        <div class="tvm-device-toolbar">
          <span class="tvm-device-count"></span>
          <input class="tvm-device-search" type="search" placeholder="Tìm thiết bị…">
          <button class="ha-filter-chip" data-select-visible>Chọn tất cả</button>
          <button class="ha-filter-chip" data-clear>Bỏ chọn hết</button>
        </div>
        <div class="tvm-checklist">
          ${entities.map(entity => {
            const checked = action.entities.includes(entity.entity_id);
            const state = entity.state === 'on' ? 'BẬT' :
              entity.state === 'off' ? 'TẮT' : entity.state || '';
            return `
              <label class="tvm-device ${checked ? 'checked' : ''}"
                data-device-name="${
                  tvmEscape((entity.name + ' ' + entity.entity_id).toLowerCase())
                }">
                <span class="tvm-device-name">
                  <input type="checkbox" value="${tvmEscape(entity.entity_id)}"
                    ${checked ? 'checked' : ''}>
                  ${tvmEscape(entity.name || entity.entity_id)}
                </span>
                <span class="tvm-device-state">${tvmEscape(state)}</span>
              </label>`;
          }).join('')}
        </div>`;

      const updateCount = () => {
        const count = group.querySelectorAll('.tvm-device input:checked').length;
        group.querySelectorAll('.tvm-device-count').forEach(element => {
          element.textContent = `${count} thiết bị đã chọn`;
        });
      };
      group.querySelectorAll('.tvm-device input').forEach(checkbox => {
        checkbox.onchange = () => {
          checkbox.closest('.tvm-device').classList.toggle('checked', checkbox.checked);
          action.entities = [...group.querySelectorAll('.tvm-device input:checked')]
            .map(selected => selected.value);
          updateCount();
        };
      });
      group.querySelector('.tvm-device-search').oninput = event => {
        const query = event.target.value.trim().toLowerCase();
        group.querySelectorAll('.tvm-device').forEach(device => {
          device.hidden = !device.dataset.deviceName.includes(query);
        });
      };
      group.querySelector('[data-select-visible]').onclick = () => {
        group.querySelectorAll('.tvm-device:not([hidden]) input').forEach(checkbox => {
          checkbox.checked = true;
          checkbox.closest('.tvm-device').classList.add('checked');
        });
        action.entities = [...group.querySelectorAll('.tvm-device input:checked')]
          .map(checkbox => checkbox.value);
        updateCount();
      };
      group.querySelector('[data-clear]').onclick = () => {
        group.querySelectorAll('.tvm-device input').forEach(checkbox => {
          checkbox.checked = false;
          checkbox.closest('.tvm-device').classList.remove('checked');
        });
        action.entities = [];
        updateCount();
      };
      group.querySelector('.tvm-service').onchange = event => {
        action.service = event.target.value;
        renderTvModes();
      };
      const extraInput = group.querySelector('.tvm-extra');
      if (extraInput) {
        extraInput.oninput = event => {
          const value = event.target.value === '' ? null : Number(event.target.value);
          action.data = value === null ? undefined : { [extra.key]: value };
        };
      }
      group.querySelector('[data-remove]').onclick = () => {
        mode.actions.splice(actionIndex, 1);
        renderTvModes();
      };
      updateCount();
      return group;
    }
