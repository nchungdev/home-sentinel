    // GENERALIZED AUTOMATION RENDERER (CHUẨN HOME ASSISTANT: WHEN / AND IF / THEN DO)
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

      const devices = haDevicesCache.filter(d => d.domain === 'light' || d.domain === 'fan' || d.domain === 'switch');
      const scripts = haDevicesCache.filter(d => d.domain === 'script');

      // 1. Render Subtabs
      tabsBar.innerHTML = rules.map(rule => `
        <button class="subtab-btn auto-tab-btn" id="btn-autotab-${escapeHtml(rule.id)}" onclick="switchAutoTab('${escapeHtml(rule.id)}', this)">
          <span class="mi">${escapeHtml(rule.icon || 'bolt')}</span>
          <span id="tab-label-${escapeHtml(rule.id)}">${escapeHtml(rule.name)}</span>
        </button>
      `).join('');

      // 2. Render Panels
      container.innerHTML = rules.map(rule => {
        const isEditing = editingRulesSet.has(rule.id);
        const selectedSet = new Set(rule.target_entities || []);
        const triggerLabel = getTriggerBadgeLabel(rule.trigger_type);

        const devicesHtml = devices.length === 0
          ? `<div style="font-size:12px; color:var(--text-muted); grid-column:span 3;">Đang tải thiết bị từ Home Assistant...</div>`
          : devices.map(d => {
              const isChecked = selectedSet.has(d.entity_id);
              const stateColor = d.state === 'on' ? '#10b981' : '#64748b';
              const stateText = d.state === 'on' ? 'ĐANG BẬT' : 'TẮT';
              return `
                <label class="ha-device-item ${isChecked ? 'checked' : ''}" data-device-name="${escapeHtml((d.name + ' ' + d.entity_id).toLowerCase())}">
                  <span style="display:flex; align-items:center; gap:7px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">
                    <input type="checkbox" class="device-checkbox-${escapeHtml(rule.id)}" value="${escapeHtml(d.entity_id)}" ${isChecked ? 'checked' : ''} onchange="onDeviceCheckboxChange(this, '${escapeHtml(rule.id)}')">
                    <span title="${escapeHtml(d.entity_id)}">${escapeHtml(d.name)}</span>
                  </span>
                  <span style="font-size:10px; color:${stateColor}; font-weight:500; flex-shrink:0;">${stateText}</span>
                </label>
              `;
            }).join('');

        return `
          <div id="autotab-${escapeHtml(rule.id)}" class="subtab-panel auto-tab-panel">
            <div class="card ha-auto-card ${isEditing ? 'editing' : ''}" id="auto-card-${escapeHtml(rule.id)}">
              <div style="display:flex; justify-content:space-between; align-items:center; gap:12px; flex-wrap:wrap;">
                <div style="display:flex; align-items:center; gap:10px; flex-wrap:wrap;">
                  <div class="card-title" style="font-size:15px; display:flex; align-items:center; gap:8px;">
                    <span class="mi" style="color:var(--accent); font-size:20px;">${escapeHtml(rule.icon || 'bolt')}</span>
                    ${isEditing
                      ? `<input type="text" id="rule-name-${escapeHtml(rule.id)}" value="${escapeHtml(rule.name)}" style="background:var(--surface); border:1px solid var(--border); color:var(--text-bright); padding:4px 10px; border-radius:6px; font-size:14px; font-weight:600; width:260px;" oninput="updateTabTitleLive('${escapeHtml(rule.id)}', this.value)">`
                      : `<span>${escapeHtml(rule.name)}</span>`}
                  </div>
                  <span class="ha-section-label"><span class="mi" style="font-size:13px;">bolt</span> ${escapeHtml(triggerLabel)}</span>
                  <span id="badge-count-${escapeHtml(rule.id)}" class="ha-badge-count">${selectedSet.size} thiết bị</span>
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
                  ${rules.length > 1 ? `
                    <button class="btn-action btn-delete" style="padding:6px 10px; font-size:12px;" onclick="deleteAutomationRule('${escapeHtml(rule.id)}')">
                      <span class="mi" style="font-size:15px;">delete</span>
                    </button>
                  ` : ''}
                </div>
              </div>

              <div class="ha-auto-body">
                <!-- CỘT TRÁI: 1. WHEN, 2. AND IF, 3. THEN DO -->
                <div class="ha-auto-left ${isEditing ? '' : 'auto-editable-field'}">
                  ${rule.description ? `<p style="font-size:12px; color:var(--text-muted); line-height:1.4; margin:0 0 4px 0;">${escapeHtml(rule.description)}</p>` : ''}
                  
                  <!-- 1. KHI NÀO (WHEN - TRIGGER) -->
                  <div class="ha-group-box">
                    <div class="ha-group-title"><span class="mi">sensors</span> 1. Khi sự kiện xảy ra (When)</div>
                    <div class="form-group">
                      <label>Sự kiện kích hoạt:</label>
                      <select id="rule-trigger-${escapeHtml(rule.id)}" onchange="onTriggerTypeChange('${escapeHtml(rule.id)}', this.value)">
                        <option value="idle_room" ${rule.trigger_type === 'idle_room' ? 'selected' : ''}>💤 Vắng người liên tục trong phòng</option>
                        <option value="person_detected" ${rule.trigger_type === 'person_detected' ? 'selected' : ''}>👤 Camera phát hiện có người</option>
                        <option value="clap_sound" ${rule.trigger_type === 'clap_sound' ? 'selected' : ''}>👏 Ngồi Sofa + Tiếng vỗ tay</option>
                        <option value="time_schedule" ${rule.trigger_type === 'time_schedule' ? 'selected' : ''}>⏰ Theo lịch trình khung giờ</option>
                      </select>
                    </div>
                    <div id="sub-trigger-opts-${escapeHtml(rule.id)}" style="display:flex; flex-direction:column; gap:8px;">
                      ${renderSubTriggerOpts(rule)}
                    </div>
                  </div>

                  <!-- 2. ĐIỀU KIỆN (AND IF - CONDITIONS) -->
                  <div class="ha-group-box">
                    <div class="ha-group-title"><span class="mi">tune</span> 2. Điều kiện lọc (And If)</div>
                    <div class="form-group">
                      <label>Khung giờ áp dụng:</label>
                      <select id="rule-time-cond-${escapeHtml(rule.id)}" onchange="onTimeCondChange('${escapeHtml(rule.id)}', this.value)">
                        <option value="all_day" ${rule.time_condition !== 'range' ? 'selected' : ''}>Cả ngày (24/24)</option>
                        <option value="range" ${rule.time_condition === 'range' ? 'selected' : ''}>Chỉ trong khung giờ cụ thể</option>
                      </select>
                    </div>
                    <div id="sub-time-range-${escapeHtml(rule.id)}" style="display:${rule.time_condition === 'range' ? 'grid' : 'none'}; grid-template-columns:1fr 1fr; gap:10px;">
                      <div class="form-group">
                        <label>Từ giờ (0-23):</label>
                        <input type="number" id="rule-start-hour-${escapeHtml(rule.id)}" value="${rule.start_hour ?? 18}" min="0" max="23">
                      </div>
                      <div class="form-group">
                        <label>Đến giờ (0-23):</label>
                        <input type="number" id="rule-end-hour-${escapeHtml(rule.id)}" value="${rule.end_hour ?? 6}" min="0" max="23">
                      </div>
                    </div>
                    <div class="form-group">
                      <label>Điều kiện Android TV:</label>
                      <select id="rule-tv-cond-${escapeHtml(rule.id)}">
                        <option value="any" ${rule.tv_condition === 'any' || !rule.tv_condition ? 'selected' : ''}>Bất kể TV bật hay tắt</option>
                        <option value="off" ${rule.tv_condition === 'off' ? 'selected' : ''}>Chỉ khi TV đang TẮT</option>
                        <option value="on" ${rule.tv_condition === 'on' ? 'selected' : ''}>Chỉ khi TV đang BẬT</option>
                      </select>
                    </div>
                  </div>

                  <!-- 3. HÀNH ĐỘNG (THEN DO - ACTIONS) -->
                  <div class="ha-group-box">
                    <div class="ha-group-title"><span class="mi">play_circle</span> 3. Hành động thực thi (Then Do)</div>
                    <div class="form-group">
                      <label>Thao tác thiết bị:</label>
                      <select id="rule-action-type-${escapeHtml(rule.id)}">
                        <option value="turn_off" ${rule.action_type !== 'turn_on' ? 'selected' : ''}>🔴 Tắt các thiết bị được chọn</option>
                        <option value="turn_on" ${rule.action_type === 'turn_on' ? 'selected' : ''}>🟢 Bật các thiết bị được chọn</option>
                      </select>
                    </div>
                    <div class="form-group">
                      <label>Đánh thức Android TV (ADB):</label>
                      <input type="text" id="rule-target-tv-${escapeHtml(rule.id)}" value="${escapeHtml(rule.target_tv || '')}" placeholder="192.168.1.8 (bỏ trống nếu không dùng)">
                    </div>
                    <div class="form-group">
                      <label>Xoay Camera về Preset:</label>
                      <select id="rule-cam-preset-${escapeHtml(rule.id)}">
                        <option value="" ${!rule.camera_preset ? 'selected' : ''}>-- Không xoay camera --</option>
                        <option value="1" ${rule.camera_preset === '1' ? 'selected' : ''}>Preset 1 (Sofa)</option>
                        <option value="2" ${rule.camera_preset === '2' ? 'selected' : ''}>Preset 2 (Bàn ăn)</option>
                        <option value="3" ${rule.camera_preset === '3' ? 'selected' : ''}>Preset 3 (Cửa chính)</option>
                      </select>
                    </div>
                    <div class="form-group">
                      <label>Kịch bản Home Assistant:</label>
                      <select id="rule-script-id-${escapeHtml(rule.id)}">
                        <option value="">-- Không dùng Script --</option>
                        ${scripts.map(s => `<option value="${escapeHtml(s.entity_id)}" ${rule.script_id === s.entity_id ? 'selected' : ''}>${escapeHtml(s.name)} (${escapeHtml(s.entity_id)})</option>`).join('')}
                      </select>
                    </div>
                  </div>
                </div>

                <!-- CỘT PHẢI: LƯỚI THIẾT BỊ HOME ASSISTANT TRÀN RỘNG TOÀN MÀN HÌNH -->
                <div class="ha-auto-right">
                  <div style="display:flex; justify-content:space-between; align-items:center; gap:10px; flex-wrap:wrap;">
                    <label style="font-size:13px; font-weight:600; color:var(--text-bright);">Danh sách thiết bị (Home Assistant):</label>
                    <div style="display:flex; align-items:center; gap:6px; flex-wrap:wrap;">
                      <input type="text" placeholder="Tìm nhanh thiết bị..." oninput="filterDeviceGrid('rule-grid-${escapeHtml(rule.id)}', this.value)" style="padding:5px 11px; font-size:12px; border-radius:6px; background:var(--surface); border:1px solid var(--border); color:var(--text-bright); width:170px;">
                      <button class="ha-filter-chip" onclick="filterSelectedOnly('rule-grid-${escapeHtml(rule.id)}', this)">Chỉ hiện đã chọn</button>
                      <div class="${isEditing ? '' : 'ha-edit-toolbar'}" style="display:inline-flex; gap:6px;">
                        <button class="ha-filter-chip" onclick="quickSelectByDomain('${escapeHtml(rule.id)}', 'light')">Chọn tất cả đèn</button>
                        <button class="ha-filter-chip" onclick="quickSelectClear('${escapeHtml(rule.id)}')">Bỏ chọn hết</button>
                      </div>
                    </div>
                  </div>
                  <div id="rule-grid-${escapeHtml(rule.id)}" class="${isEditing ? '' : 'auto-editable-field'} ha-checklist-grid">
                    ${devicesHtml}
                  </div>
                </div>
              </div>
            </div>
          </div>
        `;
      }).join('');

      // 3. Switch to active tab
      const targetTab = focusRuleId || localStorage.getItem('sentinel_active_autotab') || rules[0].id;
      const btn = document.getElementById('btn-autotab-' + targetTab);
      if (btn) {
        switchAutoTab(targetTab, btn);
      } else if (rules[0]) {
        const firstBtn = document.getElementById('btn-autotab-' + rules[0].id);
        if (firstBtn) switchAutoTab(rules[0].id, firstBtn);
      }
    }

    function renderSubTriggerOpts(rule) {
      if (rule.trigger_type === 'idle_room') {
        return `
          <div class="form-group">
            <label>Phút vắng người liên tục:</label>
            <input type="number" id="rule-idle-min-${escapeHtml(rule.id)}" value="${rule.idle_minutes || 1}" min="1" max="180">
          </div>
        `;
      }
      if (rule.trigger_type === 'person_detected') {
        return `
          <div class="form-group">
            <label>Khu vực phát hiện người:</label>
            <select id="rule-trigger-preset-${escapeHtml(rule.id)}">
              <option value="" ${!rule.trigger_preset ? 'selected' : ''}>Bất kỳ khu vực nào</option>
              <option value="1" ${rule.trigger_preset === '1' ? 'selected' : ''}>Preset 1 (Sofa)</option>
              <option value="2" ${rule.trigger_preset === '2' ? 'selected' : ''}>Preset 2 (Bàn ăn)</option>
              <option value="3" ${rule.trigger_preset === '3' ? 'selected' : ''}>Preset 3 (Cửa chính)</option>
            </select>
          </div>
        `;
      }
      if (rule.trigger_type === 'clap_sound') {
        return `
          <div class="form-group">
            <label>Vị trí nhận diện:</label>
            <select id="rule-trigger-preset-${escapeHtml(rule.id)}">
              <option value="1" selected>Preset 1 (Khu vực Sofa)</option>
            </select>
          </div>
        `;
      }
      return '';
    }

    function onTriggerTypeChange(ruleId, newType) {
      const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
      if (rule) {
        rule.trigger_type = newType;
        const sub = document.getElementById(`sub-trigger-opts-${ruleId}`);
        if (sub) sub.innerHTML = renderSubTriggerOpts(rule);
      }
    }

    function onTimeCondChange(ruleId, val) {
      const rangeEl = document.getElementById(`sub-time-range-${ruleId}`);
      if (rangeEl) rangeEl.style.display = val === 'range' ? 'grid' : 'none';
    }

    function updateTabTitleLive(ruleId, val) {
      const label = document.getElementById(`tab-label-${ruleId}`);
      if (label) label.textContent = val.trim() || 'Kịch bản';
    }

    function getTriggerBadgeLabel(triggerType) {
      switch (triggerType) {
        case 'idle_room': return 'KHI VẮNG NGƯỜI';
        case 'person_detected': return 'KHI CÓ NGƯỜI';
        case 'clap_sound': return 'KHI VỖ TAY';
        case 'time_schedule': return 'THEO LỊCH TRÌNH';
        default: return 'TỰ ĐỘNG';
      }
    }
