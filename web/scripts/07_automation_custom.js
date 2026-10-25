    function renderCustomRulesUI(focusTabId, startInEditMode) {
      const tabsBar = document.getElementById('customAutoTabsBar');
      const panelsContainer = document.getElementById('customAutoPanelsContainer');
      if (!tabsBar || !panelsContainer) return;

      const customRules = (currentAutoCfg && Array.isArray(currentAutoCfg.custom_rules)) ? currentAutoCfg.custom_rules : [];
      const controllableDevices = haDevicesCache.filter(d => d.domain === 'light' || d.domain === 'fan' || d.domain === 'switch');

      tabsBar.innerHTML = customRules.map((rule, idx) => {
        const tabId = 'custom-' + rule.id;
        const label = rule.name || `Kịch bản ${idx + 4}`;
        return `
          <button class="subtab-btn auto-tab-btn" id="btn-autotab-${escapeHtml(tabId)}" onclick="switchAutoTab('${escapeHtml(tabId)}', this)">
            <span class="mi">auto_awesome</span> <span id="tab-title-${escapeHtml(rule.id)}">${escapeHtml(label)}</span>
          </button>
        `;
      }).join('');

      panelsContainer.innerHTML = customRules.map((rule, idx) => {
        const tabId = 'custom-' + rule.id;
        const isEditing = startInEditMode && focusTabId === tabId;
        const selectedSet = new Set(rule.target_entities || []);
        const cbClass = 'custom-rule-entity-' + rule.id;
        const badgeId = 'badge-count-' + rule.id;
        const gridId = 'customGrid-' + rule.id;
        const devicesHtml = controllableDevices.length === 0
          ? `<div style="font-size:12px; color:var(--text-muted);">Chưa tải được danh sách thiết bị từ Home Assistant.</div>`
          : controllableDevices.map(d => {
              const checked = selectedSet.has(d.entity_id);
              const stateColor = d.state === 'on' ? '#10b981' : '#64748b';
              const stateText = d.state === 'on' ? 'ĐANG BẬT' : 'TẮT';
              return `
                <label class="ha-device-item ${checked ? 'checked' : ''}" data-device-name="${escapeHtml((d.name + ' ' + d.entity_id).toLowerCase())}">
                  <span style="display:flex; align-items:center; gap:7px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">
                    <input type="checkbox" class="${escapeHtml(cbClass)}" value="${escapeHtml(d.entity_id)}" ${checked ? 'checked' : ''} onchange="onDeviceCheckboxChange(this, '${escapeHtml(cbClass)}', '${escapeHtml(badgeId)}')">
                    <span title="${escapeHtml(d.entity_id)}">${escapeHtml(d.name)}</span>
                  </span>
                  <span style="font-size:10px; color:${stateColor}; font-weight:500; flex-shrink:0;">${stateText}</span>
                </label>
              `;
            }).join('');

        return `
          <div id="autotab-${escapeHtml(tabId)}" class="subtab-panel auto-tab-panel" data-rule-id="${escapeHtml(rule.id)}">
            <div class="card ha-auto-card ${isEditing ? 'editing' : ''}" id="auto-card-${escapeHtml(rule.id)}">
              <div style="display:flex; justify-content:space-between; align-items:center; gap:12px; flex-wrap:wrap;">
                <div style="display:flex; align-items:center; gap:10px; flex-wrap:wrap;">
                  <div class="card-title" style="font-size:15px; display:flex; align-items:center; gap:8px;">
                    <span class="mi" style="color:var(--accent);">auto_awesome</span>
                    <span>${escapeHtml(rule.name || `Kịch bản tùy chỉnh #${idx + 4}`)}</span>
                  </div>
                  <span class="ha-section-label"><span class="mi" style="font-size:13px;">bolt</span> TÙY CHỈNH</span>
                  <span id="${escapeHtml(badgeId)}" style="font-size:11.5px; color:#818cf8; background:rgba(255,255,255,0.04); padding:3px 9px; border-radius:6px; border:1px solid var(--border);">${selectedSet.size} thiết bị đã chọn</span>
                </div>
                <div style="display:flex; align-items:center; gap:8px;">
                  <button class="btn-action" style="padding:6px 12px; font-size:12px;" onclick="testAutomationRule('${escapeHtml(rule.id)}')">
                    <span class="mi" style="font-size:15px; color:#38bdf8;">play_arrow</span> Chạy thử
                  </button>
                  <button class="btn-action" id="btn-edit-${escapeHtml(rule.id)}" style="padding:6px 12px; font-size:12px;" onclick="toggleEditScenario('${escapeHtml(rule.id)}')">
                    <span class="mi" style="font-size:15px;">${isEditing ? 'check' : 'edit'}</span> ${isEditing ? 'Xong & Lưu' : 'Chỉnh sửa'}
                  </button>
                  <label style="display:flex; align-items:center; gap:8px; font-size:13px; color:#10b981; cursor:pointer; font-weight:500; background:rgba(16,185,129,0.1); padding:6px 12px; border-radius:8px; border:1px solid rgba(16,185,129,0.25);">
                    <input type="checkbox" id="customEnabled-${escapeHtml(rule.id)}" ${rule.enabled !== false ? 'checked' : ''} onchange="saveAutomationConfigUI()"> Bật kịch bản này
                  </label>
                  <button class="btn-action" style="padding:6px 12px; font-size:12px; background:rgba(239,68,68,0.12); color:#ef4444; border-color:rgba(239,68,68,0.3);" onclick="deleteCustomRule('${escapeHtml(rule.id)}')">
                    <span class="mi" style="font-size:15px;">delete</span> Xóa
                  </button>
                </div>
              </div>

              <div class="ha-auto-body">
                <div class="ha-auto-left auto-editable-field">
                  <div class="form-group">
                    <label>Tên kịch bản hiển thị trên Tab:</label>
                    <input type="text" id="customName-${escapeHtml(rule.id)}" value="${escapeHtml(rule.name || '')}" placeholder="VD: Kịch bản 4: Tắt điện ban đêm" oninput="updateCustomTabTitle('${escapeHtml(rule.id)}', this.value)">
                  </div>
                  <div class="form-group">
                    <label>Khi có sự kiện (When - Trigger):</label>
                    <select id="customTrigger-${escapeHtml(rule.id)}">
                      <option value="idle_room" ${rule.trigger_type === 'idle_room' ? 'selected' : ''}>Phòng vắng người quá X phút</option>
                      <option value="person_detected" ${rule.trigger_type === 'person_detected' ? 'selected' : ''}>Camera phát hiện có người (Person)</option>
                      <option value="clap_detected" ${rule.trigger_type === 'clap_detected' ? 'selected' : ''}>Thu được tiếng vỗ tay (Clapping)</option>
                    </select>
                  </div>
                  <div class="form-group">
                    <label>Thực hiện hành động (Then Do - Action):</label>
                    <select id="customAction-${escapeHtml(rule.id)}">
                      <option value="turn_off" ${rule.action_type !== 'turn_on' ? 'selected' : ''}>Tắt các thiết bị được chọn</option>
                      <option value="turn_on" ${rule.action_type === 'turn_on' ? 'selected' : ''}>Bật các thiết bị được chọn</option>
                    </select>
                  </div>
                  <div style="display:grid; grid-template-columns:1fr 1fr; gap:10px;">
                    <div class="form-group">
                      <label>Phút vắng người:</label>
                      <input type="number" id="customIdleMin-${escapeHtml(rule.id)}" value="${rule.idle_minutes || 5}" min="1" max="240">
                    </div>
                    <div class="form-group">
                      <label>Điều kiện TV:</label>
                      <select id="customCheckTV-${escapeHtml(rule.id)}">
                        <option value="false" ${!rule.check_tv_off ? 'selected' : ''}>Không xét TV</option>
                        <option value="true" ${rule.check_tv_off ? 'selected' : ''}>Khi TV đang TẮT</option>
                      </select>
                    </div>
                  </div>
                  <div style="display:grid; grid-template-columns:1fr 1fr; gap:10px;">
                    <div class="form-group">
                      <label>Từ giờ (0-23):</label>
                      <input type="number" id="customStartHour-${escapeHtml(rule.id)}" value="${rule.start_hour ?? 0}" min="0" max="23">
                    </div>
                    <div class="form-group">
                      <label>Đến giờ (0-23):</label>
                      <input type="number" id="customEndHour-${escapeHtml(rule.id)}" value="${rule.end_hour ?? 0}" min="0" max="23">
                    </div>
                  </div>
                  <div class="form-group">
                    <label>Xoay Camera Preset:</label>
                    <select id="customPreset-${escapeHtml(rule.id)}">
                      <option value="" ${!rule.camera_preset ? 'selected' : ''}>-- Không xoay --</option>
                      <option value="1" ${rule.camera_preset === '1' ? 'selected' : ''}>Preset 1 (Sofa)</option>
                      <option value="2" ${rule.camera_preset === '2' ? 'selected' : ''}>Preset 2 (Bàn ăn)</option>
                      <option value="3" ${rule.camera_preset === '3' ? 'selected' : ''}>Preset 3 (Cửa chính)</option>
                    </select>
                  </div>
                </div>

                <div class="ha-auto-right">
                  <div style="display:flex; justify-content:space-between; align-items:center; gap:10px; flex-wrap:wrap;">
                    <label style="font-size:13px; font-weight:500; color:var(--text-bright);">Chọn thiết bị áp dụng (Home Assistant):</label>
                    <div style="display:flex; align-items:center; gap:6px; flex-wrap:wrap;">
                      <input type="text" placeholder="Tìm nhanh thiết bị..." oninput="filterDeviceGrid('${escapeHtml(gridId)}', this.value)" style="padding:4px 10px; font-size:12px; border-radius:6px; background:var(--surface); border:1px solid var(--border); color:var(--text-bright); width:160px;">
                      <button class="ha-filter-chip" onclick="filterSelectedOnly('${escapeHtml(gridId)}', this)">Chỉ hiện đã chọn</button>
                    </div>
                  </div>
                  <div id="${escapeHtml(gridId)}" class="auto-editable-field ha-checklist-grid">
                    ${devicesHtml}
                  </div>
                </div>
              </div>
            </div>
          </div>
        `;
      }).join('');

      if (focusTabId) {
        const btn = document.getElementById('btn-autotab-' + focusTabId);
        if (btn) switchAutoTab(focusTabId, btn);
      }
    }

    function updateCustomTabTitle(ruleId, newTitle) {
      const span = document.getElementById('tab-title-' + ruleId);
      if (span) {
        span.textContent = newTitle.trim() || 'Kịch bản mới';
      }
    }

    function collectCustomRulesFromDOM() {
      if (!currentAutoCfg || !Array.isArray(currentAutoCfg.custom_rules)) return [];
      return currentAutoCfg.custom_rules.map(rule => {
        const id = rule.id;
        const nameEl = document.getElementById('customName-' + id);
        if (!nameEl) return rule;
        const checkedEntities = Array.from(document.querySelectorAll('.custom-rule-entity-' + id + ':checked')).map(el => el.value);
        return {
          id: id,
          name: nameEl.value.trim() || rule.name,
          enabled: document.getElementById('customEnabled-' + id)?.checked !== false,
          trigger_type: document.getElementById('customTrigger-' + id)?.value || 'idle_room',
          action_type: document.getElementById('customAction-' + id)?.value || 'turn_off',
          idle_minutes: parseInt(document.getElementById('customIdleMin-' + id)?.value, 10) || 5,
          start_hour: parseInt(document.getElementById('customStartHour-' + id)?.value, 10) || 0,
          end_hour: parseInt(document.getElementById('customEndHour-' + id)?.value, 10) || 0,
          check_tv_off: document.getElementById('customCheckTV-' + id)?.value === 'true',
          camera_preset: document.getElementById('customPreset-' + id)?.value || '',
          target_entities: checkedEntities
        };
      });
    }

    function addNewAutomationRule() {
      if (!currentAutoCfg) currentAutoCfg = {};
      currentAutoCfg.custom_rules = collectCustomRulesFromDOM();
      const nextNum = currentAutoCfg.custom_rules.length + 4;
      const newId = 'rule_' + Date.now();
      currentAutoCfg.custom_rules.push({
        id: newId,
        name: `Kịch bản ${nextNum}: Kịch bản mới`,
        enabled: true,
        trigger_type: 'idle_room',
        action_type: 'turn_off',
        idle_minutes: 10,
        start_hour: 0,
        end_hour: 0,
        check_tv_off: true,
        camera_preset: '',
        target_entities: []
      });
      renderCustomRulesUI('custom-' + newId, true);
    }

    function deleteCustomRule(ruleId) {
      if (!currentAutoCfg || !Array.isArray(currentAutoCfg.custom_rules)) return;
      currentAutoCfg.custom_rules = collectCustomRulesFromDOM().filter(r => r.id !== ruleId);
      renderCustomRulesUI('idle', false);
      saveAutomationConfigUI();
    }

    async function saveAutomationConfigUI() {
      const btn = document.getElementById('btnSaveAutomation');
      const statusEl = document.getElementById('autoSaveStatus');
      const checkedLights = Array.from(document.querySelectorAll('.auto-light-item:checked')).map(el => el.value);
      const checkedClapDevices = Array.from(document.querySelectorAll('.auto-clap-item:checked')).map(el => el.value);
      const checkedEveningLights = Array.from(document.querySelectorAll('.auto-evening-item:checked')).map(el => el.value);
      const customRules = collectCustomRulesFromDOM();

      const primaryFan = document.getElementById('autoClapFan').value || checkedClapDevices[0] || '';
      const primaryEveLight = checkedEveningLights[0] || document.getElementById('autoEveningLight').value || '';

      const payload = {
        config: {
          idle_living_room: {
            enabled: document.getElementById('autoIdleEnabled').checked,
            idle_minutes: parseInt(document.getElementById('autoIdleMinutes').value, 10) || 1,
            target_lights: checkedLights,
            turn_off_script: document.getElementById('autoIdleScript').value,
            check_tv_playback: document.getElementById('autoIdleCheckTV').value === 'true'
          },
          sofa_clap_action: {
            enabled: document.getElementById('autoClapEnabled').checked,
            target_fan: primaryFan,
            target_entities: checkedClapDevices,
            target_tv: document.getElementById('autoClapTV').value.trim() || '192.168.1.8',
            window_seconds: parseInt(document.getElementById('autoClapWindow').value, 10) || 60
          },
          evening_arrival: {
            enabled: document.getElementById('autoEveningEnabled').checked,
            target_light: primaryEveLight,
            target_entities: checkedEveningLights,
            target_preset: document.getElementById('autoEveningPreset').value,
            cooldown_sec: (currentAutoCfg && currentAutoCfg.evening_arrival && currentAutoCfg.evening_arrival.cooldown_sec) || 300,
            start_hour: parseInt(document.getElementById('autoEveningStart').value, 10) || 18,
            end_hour: parseInt(document.getElementById('autoEveningEnd').value, 10) || 6,
            sound_keywords: (currentAutoCfg && currentAutoCfg.evening_arrival && currentAutoCfg.evening_arrival.sound_keywords) || []
          },
          custom_rules: customRules
        }
      };

      btn.disabled = true;
      btn.innerHTML = '<span class="mi">hourglass_top</span> Đang lưu...';
      try {
        const res = await fetch('/api/automation/config', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (data.ok) {
          currentAutoCfg = data.config;
          statusEl.innerHTML = `<span style="color:#10b981;"><span class="mi" style="font-size:15px;">check_circle</span> Đã lưu kịch bản vào configs/automation.json (${checkedLights.length} đèn)!</span>`;
        } else {
          statusEl.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi lưu cấu hình</span>`;
        }
      } catch (e) {
        statusEl.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi: ${e.message}</span>`;
      } finally {
        btn.disabled = false;
        btn.innerHTML = '<span class="mi">save</span> Lưu Kịch Bản';
      }
    }

    // Tự động load config khi mở
    loadSavedCameraConfig();
    loadSavedHAConfig();
    loadAutomationConfigAndDevices();
    renderSavedChat();

    // Khôi phục tab đang chọn từ localStorage (tránh bị reset khi F5)
    (function restoreNavigationState() {
      try {
        const savedNav = localStorage.getItem('sentinel_active_nav');
        if (savedNav) {
          const navEl = document.getElementById('nav-' + savedNav);
          if (navEl) switchNav(savedNav, navEl);
        }
        const savedAutoTab = localStorage.getItem('sentinel_active_autotab');
        if (savedAutoTab) {
          const autoBtn = document.getElementById('btn-autotab-' + savedAutoTab);
          if (autoBtn) switchAutoTab(savedAutoTab, autoBtn);
        }
      } catch (e) {}
      initCameraEventStream();
    })();
