    // GENERALIZED HOME ASSISTANT AUTOMATION RENDERER (WHEN / AND IF / THEN DO)
    function renderAllAutomationUI(focusRuleId) {
      const tabsBar = document.getElementById('autoTabsBar');
      const container = document.getElementById('autoPanelsContainer');
      if (!tabsBar || !container) return;

      const rules = (currentAutoCfg && Array.isArray(currentAutoCfg.rules)) ? currentAutoCfg.rules : [];
      if (rules.length === 0) {
        container.innerHTML = `<div style="padding:40px; text-align:center; color:var(--text-muted); font-size:13px;">Chưa có kịch bản nào. Bấm <b>"+ Thêm kịch bản"</b> để bắt đầu.</div>`;
        tabsBar.innerHTML = '';
        return;
      }

      tabsBar.innerHTML = rules.map(rule => `
        <button class="subtab-btn auto-tab-btn" id="btn-autotab-${escapeHtml(rule.id)}" onclick="switchAutoTab('${escapeHtml(rule.id)}', this)">
          <span class="mi">${escapeHtml(rule.icon || 'bolt')}</span>
          <span id="tab-label-${escapeHtml(rule.id)}">${escapeHtml(rule.name)}</span>
        </button>
      `).join('');

      container.innerHTML = rules.map(rule => renderSingleRulePanel(rule)).join('');

      const targetTab = focusRuleId || localStorage.getItem('sentinel_active_autotab') || rules[0].id;
      const btn = document.getElementById('btn-autotab-' + targetTab);
      if (btn) switchAutoTab(targetTab, btn);
      else if (rules[0]) {
        const firstBtn = document.getElementById('btn-autotab-' + rules[0].id);
        if (firstBtn) switchAutoTab(rules[0].id, firstBtn);
      }
    }

    function renderSingleRulePanel(rule) {
      const isEditing = editingRulesSet.has(rule.id);
      return `
        <div id="autotab-${escapeHtml(rule.id)}" class="subtab-panel auto-tab-panel">
          <div class="card ha-auto-card ${isEditing ? 'editing' : ''}" id="auto-card-${escapeHtml(rule.id)}" style="display:flex; flex-direction:column; gap:18px;">
            ${renderCardHeader(rule, isEditing)}
            <div class="${isEditing ? '' : 'auto-editable-field'}" style="display:flex; flex-direction:column; gap:18px; width:100%;">
              <div class="ha-ha-top-row">
                ${renderWhenSection(rule, isEditing)}
                ${renderAndIfSection(rule, isEditing)}
              </div>
              ${renderThenDoSection(rule, isEditing)}
            </div>
          </div>
        </div>
      `;
    }

    function renderCardHeader(rule, isEditing) {
      return `
        <div style="display:flex; justify-content:space-between; align-items:center; gap:12px; flex-wrap:wrap; border-bottom:1px solid var(--border); padding-bottom:14px;">
          <div style="display:flex; align-items:center; gap:10px; flex-wrap:wrap;">
            <span class="mi" style="color:var(--accent); font-size:22px;">${escapeHtml(rule.icon || 'bolt')}</span>
            ${isEditing
              ? `<input type="text" id="rule-name-${escapeHtml(rule.id)}" value="${escapeHtml(rule.name)}" style="background:var(--surface); border:1px solid var(--border); color:var(--text-bright); padding:5px 12px; border-radius:6px; font-size:14.5px; font-weight:600; width:280px;" oninput="updateTabTitleLive('${escapeHtml(rule.id)}', this.value)">`
              : `<span style="font-size:15px; font-weight:600; color:var(--text-bright);">${escapeHtml(rule.name)}</span>`}
          </div>
          <div style="display:flex; align-items:center; gap:8px;">
            <button class="btn-action" style="padding:6px 12px; font-size:12px;" onclick="testAutomationRule('${escapeHtml(rule.id)}')">
              <span class="mi" style="font-size:15px; color:#38bdf8;">play_arrow</span> Chạy thử
            </button>
            <button class="btn-action" id="btn-edit-${escapeHtml(rule.id)}" style="padding:6px 12px; font-size:12px;" onclick="toggleEditScenario('${escapeHtml(rule.id)}')">
              <span class="mi" style="font-size:15px;">${isEditing ? 'check' : 'edit'}</span> ${isEditing ? 'Xong & Lưu' : 'Chỉnh sửa'}
            </button>
            <label class="ha-toggle-label">
              <input type="checkbox" id="rule-enabled-${escapeHtml(rule.id)}" ${rule.enabled !== false ? 'checked' : ''} onchange="toggleRuleEnabled('${escapeHtml(rule.id)}', this.checked)"> Bật kịch bản
            </label>
            ${(currentAutoCfg.rules || []).length > 1 ? `
              <button class="btn-action btn-delete" style="padding:6px 10px; font-size:12px;" onclick="deleteAutomationRule('${escapeHtml(rule.id)}')">
                <span class="mi" style="font-size:15px;">delete</span>
              </button>
            ` : ''}
          </div>
        </div>
      `;
    }

    function renderWhenSection(rule, isEditing) {
      const triggers = Array.isArray(rule.triggers) ? rule.triggers : [];
      return `
        <div class="ha-ha-section">
          <div class="ha-ha-section-header">
            <div class="ha-ha-title" style="color:#38bdf8;"><span class="mi">sensors</span> Khi nào (When / Triggers)</div>
            ${isEditing ? `<button class="btn-action" style="padding:4px 9px; font-size:11.5px;" onclick="addStepToRule('${escapeHtml(rule.id)}', 'triggers')"><span class="mi" style="font-size:14px;">add</span> Thêm kích hoạt</button>` : ''}
          </div>
          <div style="display:flex; flex-direction:column; gap:10px;">
            ${triggers.map((trig, idx) => `
              <div class="ha-step-card when">
                <div class="ha-step-header">
                  <span style="font-size:12px; font-weight:600; color:#38bdf8; display:flex; align-items:center; gap:5px;">
                    <span class="mi" style="font-size:15px;">bolt</span> Kích hoạt #${idx + 1}
                  </span>
                  ${isEditing && triggers.length > 1 ? `<button class="btn-action btn-delete" style="padding:2px 6px; font-size:11px;" onclick="removeStepFromRule('${escapeHtml(rule.id)}', 'triggers', ${idx})"><span class="mi" style="font-size:13px;">close</span></button>` : ''}
                </div>
                <div class="form-group">
                  <label>Sự kiện:</label>
                  <select id="trig-type-${escapeHtml(rule.id)}-${idx}" onchange="onTriggerTypeChange('${escapeHtml(rule.id)}', ${idx}, this.value)">
                    <option value="idle_room" ${trig.type === 'idle_room' ? 'selected' : ''}>💤 Vắng người liên tục trong phòng</option>
                    <option value="person_detected" ${trig.type === 'person_detected' ? 'selected' : ''}>👤 Camera phát hiện có người</option>
                    <option value="clap_sound" ${trig.type === 'clap_sound' ? 'selected' : ''}>👏 Ngồi Sofa + Tiếng vỗ tay</option>
                    <option value="time_schedule" ${trig.type === 'time_schedule' ? 'selected' : ''}>⏰ Theo khung giờ / Lịch trình</option>
                  </select>
                </div>
                ${trig.type === 'idle_room' ? `
                  <div class="form-group"><label>Số phút vắng người:</label><input type="number" id="trig-idle-${escapeHtml(rule.id)}-${idx}" value="${trig.idle_minutes || 1}" min="1" max="180"></div>
                ` : `
                  <div class="form-group"><label>Khu vực phát hiện:</label>
                    <select id="trig-preset-${escapeHtml(rule.id)}-${idx}">
                      <option value="" ${!trig.preset ? 'selected' : ''}>Bất kỳ khu vực nào</option>
                      <option value="1" ${trig.preset === '1' ? 'selected' : ''}>Preset 1 (Sofa)</option>
                      <option value="2" ${trig.preset === '2' ? 'selected' : ''}>Preset 2 (Bàn ăn)</option>
                      <option value="3" ${trig.preset === '3' ? 'selected' : ''}>Preset 3 (Cửa chính)</option>
                    </select>
                  </div>
                `}
              </div>
            `).join('')}
          </div>
        </div>
      `;
    }

    function renderAndIfSection(rule, isEditing) {
      const conditions = Array.isArray(rule.conditions) ? rule.conditions : [];
      return `
        <div class="ha-ha-section">
          <div class="ha-ha-section-header">
            <div class="ha-ha-title" style="color:#f59e0b;"><span class="mi">tune</span> Và nếu (And if / Conditions)</div>
            ${isEditing ? `<button class="btn-action" style="padding:4px 9px; font-size:11.5px;" onclick="addStepToRule('${escapeHtml(rule.id)}', 'conditions')"><span class="mi" style="font-size:14px;">add</span> Thêm điều kiện</button>` : ''}
          </div>
          <div style="display:flex; flex-direction:column; gap:10px;">
            ${conditions.length === 0 ? `<div style="font-size:12px; color:var(--text-muted); padding:10px 0;">Không có điều kiện lọc (hành động luôn được thực thi khi sự kiện kích hoạt).</div>` : ''}
            ${conditions.map((cond, idx) => `
              <div class="ha-step-card and-if">
                <div class="ha-step-header">
                  <span style="font-size:12px; font-weight:600; color:#f59e0b; display:flex; align-items:center; gap:5px;">
                    <span class="mi" style="font-size:15px;">filter_alt</span> Điều kiện #${idx + 1}
                  </span>
                  ${isEditing ? `<button class="btn-action btn-delete" style="padding:2px 6px; font-size:11px;" onclick="removeStepFromRule('${escapeHtml(rule.id)}', 'conditions', ${idx})"><span class="mi" style="font-size:13px;">close</span></button>` : ''}
                </div>
                <div class="form-group">
                  <label>Loại điều kiện:</label>
                  <select id="cond-type-${escapeHtml(rule.id)}-${idx}" onchange="onConditionTypeChange('${escapeHtml(rule.id)}', ${idx}, this.value)">
                    <option value="tv_state" ${cond.type === 'tv_state' ? 'selected' : ''}>Trạng thái Android TV</option>
                    <option value="time_range" ${cond.type === 'time_range' ? 'selected' : ''}>Giới hạn khung giờ (Từ - Đến)</option>
                  </select>
                </div>
                ${cond.type === 'time_range' ? `
                  <div style="display:grid; grid-template-columns:1fr 1fr; gap:10px;">
                    <div class="form-group"><label>Từ giờ (0-23):</label><input type="number" id="cond-start-${escapeHtml(rule.id)}-${idx}" value="${cond.start_hour ?? 18}" min="0" max="23"></div>
                    <div class="form-group"><label>Đến giờ (0-23):</label><input type="number" id="cond-end-${escapeHtml(rule.id)}-${idx}" value="${cond.end_hour ?? 6}" min="0" max="23"></div>
                  </div>
                ` : `
                  <div class="form-group"><label>Điều kiện TV:</label>
                    <select id="cond-tv-${escapeHtml(rule.id)}-${idx}">
                      <option value="off" ${cond.tv_state === 'off' ? 'selected' : ''}>Chỉ khi TV đang TẮT</option>
                      <option value="on" ${cond.tv_state === 'on' ? 'selected' : ''}>Chỉ khi TV đang BẬT</option>
                    </select>
                  </div>
                `}
              </div>
            `).join('')}
          </div>
        </div>
      `;
    }

    function renderThenDoSection(rule, isEditing) {
      const actions = Array.isArray(rule.actions) ? rule.actions : [];
      return `
        <div class="ha-ha-section">
          <div class="ha-ha-section-header">
            <div class="ha-ha-title" style="color:#10b981;"><span class="mi">play_circle</span> Thì làm (Then do / Actions Pipeline)</div>
            ${isEditing ? `<button class="btn-action" style="padding:4px 9px; font-size:11.5px;" onclick="addStepToRule('${escapeHtml(rule.id)}', 'actions')"><span class="mi" style="font-size:14px;">add</span> Thêm hành động</button>` : ''}
          </div>
          <div style="display:flex; flex-direction:column; gap:14px;">
            ${actions.map((act, idx) => renderSingleActionCard(rule, act, idx, isEditing, actions.length)).join('')}
          </div>
        </div>
      `;
    }

    function renderSingleActionCard(rule, act, idx, isEditing, totalActions) {
      const controllable = haDevicesCache.filter(d => d.domain === 'light' || d.domain === 'fan' || d.domain === 'switch');
      const scripts = haDevicesCache.filter(d => d.domain === 'script');
      const selectedSet = new Set(act.target_entities || []);

      return `
        <div class="ha-step-card then-do">
          <div class="ha-step-header">
            <div style="display:flex; align-items:center; gap:8px;">
              <span class="ha-step-badge">BƯỚC ${idx + 1}</span>
              <span style="font-size:13px; font-weight:600; color:var(--text-bright);">Hành động thực thi</span>
            </div>
            ${isEditing && totalActions > 1 ? `<button class="btn-action btn-delete" style="padding:2px 6px; font-size:11px;" onclick="removeStepFromRule('${escapeHtml(rule.id)}', 'actions', ${idx})"><span class="mi" style="font-size:13px;">close</span></button>` : ''}
          </div>
          <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:12px;">
            <div class="form-group">
              <label>Loại hành động:</label>
              <select id="act-type-${escapeHtml(rule.id)}-${idx}" onchange="onActionTypeChange('${escapeHtml(rule.id)}', ${idx}, this.value)">
                <option value="entities" ${act.type === 'entities' ? 'selected' : ''}>💡 Thao tác thiết bị (Home Assistant)</option>
                <option value="script" ${act.type === 'script' ? 'selected' : ''}>📜 Kích hoạt Script Home Assistant</option>
                <option value="tv" ${act.type === 'tv' ? 'selected' : ''}>📺 Điều khiển Android TV (ADB)</option>
                <option value="camera" ${act.type === 'camera' ? 'selected' : ''}>📷 Xoay Camera PTZ về Preset</option>
              </select>
            </div>
            ${act.type === 'entities' ? `
              <div class="form-group">
                <label>Lệnh thực thi:</label>
                <select id="act-srv-${escapeHtml(rule.id)}-${idx}">
                  <option value="turn_off" ${act.service !== 'turn_on' ? 'selected' : ''}>🔴 Tắt toàn bộ thiết bị đã chọn</option>
                  <option value="turn_on" ${act.service === 'turn_on' ? 'selected' : ''}>🟢 Bật toàn bộ thiết bị đã chọn</option>
                </select>
              </div>
            ` : ''}
            ${act.type === 'tv' ? `
              <div class="form-group">
                <label>Lệnh TV:</label>
                <select id="act-tv-cmd-${escapeHtml(rule.id)}-${idx}">
                  <option value="turn_off" ${act.service === 'turn_off' ? 'selected' : ''}>🔴 Tắt TV (Sleep / Tắt màn hình)</option>
                  <option value="turn_on" ${act.service !== 'turn_off' ? 'selected' : ''}>🟢 Bật TV (Wake up / Bật màn hình)</option>
                </select>
              </div>
              <div class="form-group"><label>IP Android TV:</label><input type="text" id="act-tv-${escapeHtml(rule.id)}-${idx}" value="${escapeHtml(act.target_tv || '192.168.1.8')}"></div>
            ` : ''}
            ${act.type === 'camera' ? `
              <div class="form-group"><label>Vị trí Preset:</label>
                <select id="act-cam-${escapeHtml(rule.id)}-${idx}">
                  <option value="1" ${act.camera_preset === '1' ? 'selected' : ''}>Preset 1 (Sofa)</option>
                  <option value="2" ${act.camera_preset === '2' ? 'selected' : ''}>Preset 2 (Bàn ăn)</option>
                  <option value="3" ${act.camera_preset === '3' ? 'selected' : ''}>Preset 3 (Cửa chính)</option>
                </select>
              </div>
            ` : ''}
            ${act.type === 'script' ? `
              <div class="form-group"><label>Script Home Assistant:</label>
                <select id="act-script-${escapeHtml(rule.id)}-${idx}">
                  ${scripts.map(s => `<option value="${escapeHtml(s.entity_id)}" ${act.script_id === s.entity_id ? 'selected' : ''}>${escapeHtml(s.name)}</option>`).join('')}
                </select>
              </div>
            ` : ''}
          </div>
          ${act.type === 'entities' ? `
            <div style="margin-top:8px; display:flex; flex-direction:column; gap:8px;">
              <div style="display:flex; justify-content:space-between; align-items:center; gap:8px; flex-wrap:wrap;">
                <span id="badge-act-${escapeHtml(rule.id)}-${idx}" class="ha-badge-count">${selectedSet.size} thiết bị đã chọn</span>
                <div style="display:flex; align-items:center; gap:6px; flex-wrap:wrap;">
                  <input type="text" placeholder="Tìm nhanh thiết bị..." oninput="filterDeviceGrid('grid-${escapeHtml(rule.id)}-${idx}', this.value)" style="padding:4px 9px; font-size:12px; border-radius:6px; background:var(--surface); border:1px solid var(--border); color:var(--text-bright); width:160px;">
                  <button class="ha-filter-chip" onclick="filterSelectedOnly('grid-${escapeHtml(rule.id)}-${idx}', this)">Chỉ hiện đã chọn</button>
                  <button class="ha-filter-chip" onclick="quickSelectByDomain('${escapeHtml(rule.id)}', ${idx}, 'light')">Chọn tất cả đèn</button>
                  <button class="ha-filter-chip" onclick="quickSelectClear('${escapeHtml(rule.id)}', ${idx})">Bỏ chọn hết</button>
                </div>
              </div>
              <div id="grid-${escapeHtml(rule.id)}-${idx}" class="ha-checklist-grid">
                ${controllable.map(d => {
                  const checked = selectedSet.has(d.entity_id);
                  const stateColor = d.state === 'on' ? '#10b981' : '#64748b';
                  return `
                    <label class="ha-device-item ${checked ? 'checked' : ''}" data-device-name="${escapeHtml((d.name + ' ' + d.entity_id).toLowerCase())}">
                      <span style="display:flex; align-items:center; gap:7px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">
                        <input type="checkbox" class="device-checkbox-${escapeHtml(rule.id)}-${idx}" value="${escapeHtml(d.entity_id)}" ${checked ? 'checked' : ''} onchange="onDeviceCheckboxChange(this, '${escapeHtml(rule.id)}', ${idx})">
                        <span title="${escapeHtml(d.entity_id)}">${escapeHtml(d.name)}</span>
                      </span>
                      <span style="font-size:10px; color:${stateColor}; font-weight:500; flex-shrink:0;">${d.state === 'on' ? 'BẬT' : 'TẮT'}</span>
                    </label>
                  `;
                }).join('')}
              </div>
            </div>
          ` : ''}
        </div>
      `;
    }

    function updateTabTitleLive(ruleId, val) {
      const label = document.getElementById(`tab-label-${ruleId}`);
      if (label) label.textContent = val.trim() || 'Kịch bản';
    }
