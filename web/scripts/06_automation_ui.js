    // AUTOMATION CONFIG & REAL HOME ASSISTANT DEVICES
    let currentAutoCfg = null;
    let haDevicesCache = [];

    async function loadAutomationConfigAndDevices() {
      const checklistEl = document.getElementById('autoIdleLightsChecklist');
      try {
        const [cfgRes, entRes] = await Promise.all([
          fetch('/api/automation/config'),
          fetch('/api/ha/entities')
        ]);
        const cfgData = await cfgRes.json();
        const entData = await entRes.json();

        if (cfgData.ok && cfgData.config) {
          currentAutoCfg = cfgData.config;
        }
        if (entData.ok && Array.isArray(entData.entities)) {
          haDevicesCache = entData.entities;
        }
        renderAutomationUI();
      } catch (e) {
        if (checklistEl) {
          checklistEl.innerHTML = `<div style="color:#ef4444; font-size:12px;">Lỗi tải danh sách thiết bị: ${e.message}</div>`;
        }
      }
    }

    function renderAutomationUI() {
      if (!currentAutoCfg) return;
      const idle = currentAutoCfg.idle_living_room || {};
      const clap = currentAutoCfg.sofa_clap_action || {};
      const eve = currentAutoCfg.evening_arrival || {};

      document.getElementById('autoIdleEnabled').checked = !!idle.enabled;
      document.getElementById('autoIdleMinutes').value = idle.idle_minutes || 1;
      document.getElementById('autoIdleCheckTV').value = idle.check_tv_playback !== false ? 'true' : 'false';

      document.getElementById('autoClapEnabled').checked = !!clap.enabled;
      document.getElementById('autoClapTV').value = clap.target_tv || '192.168.1.8';
      document.getElementById('autoClapWindow').value = clap.window_seconds || 60;

      document.getElementById('autoEveningEnabled').checked = !!eve.enabled;
      document.getElementById('autoEveningStart').value = eve.start_hour ?? 18;
      document.getElementById('autoEveningEnd').value = eve.end_hour ?? 6;
      document.getElementById('autoEveningPreset').value = eve.target_preset || '3';

      const lights = haDevicesCache.filter(d => d.domain === 'light');
      const fans = haDevicesCache.filter(d => d.domain === 'fan' || d.domain === 'switch');
      const controllableDevices = haDevicesCache.filter(d => d.domain === 'light' || d.domain === 'fan' || d.domain === 'switch');
      const scripts = haDevicesCache.filter(d => d.domain === 'script');

      // 1. Render checklist đèn cần tắt (Tab 1)
      const selectedIdleLights = new Set(idle.target_lights || []);
      renderDeviceGridContainer('autoIdleLightsChecklist', lights, selectedIdleLights, 'auto-light-item', 'badge-count-idle');

      // 2. Render Script dropdown (Tab 1)
      const scriptSel = document.getElementById('autoIdleScript');
      scriptSel.innerHTML = `<option value="">-- Không dùng Script --</option>` +
        scripts.map(s => `<option value="${escapeHtml(s.entity_id)}" ${idle.turn_off_script === s.entity_id ? 'selected' : ''}>${escapeHtml(s.name)} (${escapeHtml(s.entity_id)})</option>`).join('');

      // 3. Render Fan dropdown & checklist thiết bị cần bật (Tab 2)
      const fanSel = document.getElementById('autoClapFan');
      fanSel.innerHTML = `<option value="">-- Chọn thiết bị quạt chính --</option>` +
        fans.map(f => `<option value="${escapeHtml(f.entity_id)}" ${clap.target_fan === f.entity_id ? 'selected' : ''}>[${f.domain.toUpperCase()}] ${escapeHtml(f.name)} (${f.state.toUpperCase()})</option>`).join('');
      const clapTargets = (Array.isArray(clap.target_entities) && clap.target_entities.length > 0)
        ? clap.target_entities
        : (clap.target_fan ? [clap.target_fan] : []);
      renderDeviceGridContainer('autoClapDevicesChecklist', controllableDevices, new Set(clapTargets), 'auto-clap-item', 'badge-count-clap');

      // 4. Render checklist đèn đón buổi tối (Tab 3)
      const eveLightInput = document.getElementById('autoEveningLight');
      if (eveLightInput) eveLightInput.value = eve.target_light || '';
      const eveTargets = (Array.isArray(eve.target_entities) && eve.target_entities.length > 0)
        ? eve.target_entities
        : (eve.target_light ? [eve.target_light] : []);
      renderDeviceGridContainer('autoEveningLightsChecklist', lights, new Set(eveTargets), 'auto-evening-item', 'badge-count-evening');

      // 5. Render Custom Rules Tabs & Panels
      renderCustomRulesUI();
    }

    function renderDeviceGridContainer(containerId, devices, selectedSet, checkboxClass, badgeId) {
      const container = document.getElementById(containerId);
      if (!container) return;
      if (!devices || devices.length === 0) {
        container.innerHTML = `<div style="font-size:12px; color:var(--text-muted); grid-column:span 3;">Chưa tải được danh sách thiết bị từ Home Assistant.</div>`;
        updateRuleBadgeCount(checkboxClass, badgeId);
        return;
      }
      container.innerHTML = devices.map(d => {
        const isChecked = selectedSet.has(d.entity_id);
        const stateColor = d.state === 'on' ? '#10b981' : '#64748b';
        const stateText = d.state === 'on' ? 'ĐANG BẬT' : 'TẮT';
        return `
          <label class="ha-device-item ${isChecked ? 'checked' : ''}" data-device-name="${escapeHtml((d.name + ' ' + d.entity_id).toLowerCase())}">
            <span style="display:flex; align-items:center; gap:7px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">
              <input type="checkbox" class="${escapeHtml(checkboxClass)}" value="${escapeHtml(d.entity_id)}" ${isChecked ? 'checked' : ''} onchange="onDeviceCheckboxChange(this, '${escapeHtml(checkboxClass)}', '${escapeHtml(badgeId)}')">
              <span title="${escapeHtml(d.entity_id)}">${escapeHtml(d.name)}</span>
            </span>
            <span style="font-size:10px; color:${stateColor}; font-weight:500; flex-shrink:0;">${stateText}</span>
          </label>
        `;
      }).join('');
      updateRuleBadgeCount(checkboxClass, badgeId);
    }

    function onDeviceCheckboxChange(checkboxEl, checkboxClass, badgeId) {
      const label = checkboxEl.closest('.ha-device-item');
      if (label) {
        label.classList.toggle('checked', checkboxEl.checked);
      }
      updateRuleBadgeCount(checkboxClass, badgeId);
    }

    function updateRuleBadgeCount(checkboxClass, badgeId) {
      const badge = document.getElementById(badgeId);
      if (!badge) return;
      const count = document.querySelectorAll('.' + checkboxClass + ':checked').length;
      badge.textContent = `${count} thiết bị đã chọn`;
      badge.style.color = count > 0 ? '#818cf8' : 'var(--text-muted)';
    }

    function filterDeviceGrid(containerId, query) {
      const container = document.getElementById(containerId);
      if (!container) return;
      const q = (query || '').trim().toLowerCase();
      container.querySelectorAll('.ha-device-item').forEach(item => {
        const name = item.getAttribute('data-device-name') || '';
        item.style.display = (!q || name.includes(q)) ? 'flex' : 'none';
      });
    }

    function filterSelectedOnly(containerId, btnEl) {
      const container = document.getElementById(containerId);
      if (!container) return;
      const active = btnEl.classList.toggle('active');
      btnEl.textContent = active ? 'Hiện tất cả' : 'Chỉ hiện đã chọn';
      container.querySelectorAll('.ha-device-item').forEach(item => {
        const cb = item.querySelector('input[type="checkbox"]');
        item.style.display = (!active || (cb && cb.checked)) ? 'flex' : 'none';
      });
    }

    function quickSelectByKeywords(checkboxClass, keywords) {
      document.querySelectorAll('.' + checkboxClass).forEach(cb => {
        const label = cb.closest('.ha-device-item');
        const text = (label ? label.getAttribute('data-device-name') : '') || '';
        const shouldCheck = keywords.length > 0 && keywords.some(kw => text.includes(kw));
        cb.checked = shouldCheck;
        if (label) label.classList.toggle('checked', shouldCheck);
      });
      if (checkboxClass === 'auto-light-item') updateRuleBadgeCount(checkboxClass, 'badge-count-idle');
    }

    function toggleEditScenario(scenarioKey) {
      const card = document.getElementById('auto-card-' + scenarioKey);
      const btn = document.getElementById('btn-edit-' + scenarioKey);
      if (!card || !btn) return;
      const isEditing = card.classList.toggle('editing');
      if (isEditing) {
        btn.innerHTML = `<span class="mi" style="font-size:15px; color:#10b981;">check</span> Xong &amp; Lưu`;
        btn.style.borderColor = 'rgba(16, 185, 129, 0.45)';
        btn.style.background = 'rgba(16, 185, 129, 0.12)';
      } else {
        btn.innerHTML = `<span class="mi" style="font-size:15px;">edit</span> Chỉnh sửa`;
        btn.style.borderColor = '';
        btn.style.background = '';
        saveAutomationConfigUI();
      }
    }

    async function testAutomationRule(ruleKey) {
      const statusEl = document.getElementById('autoSaveStatus');
      if (statusEl) {
        statusEl.innerHTML = `<span style="color:#38bdf8;"><span class="mi" style="font-size:15px;">play_circle</span> Đang chạy thử kịch bản...</span>`;
      }
      try {
        const res = await fetch('/api/automation/config', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ test_rule: ruleKey })
        });
        const data = await res.json();
        if (data.ok && statusEl) {
          statusEl.innerHTML = `<span style="color:#10b981;"><span class="mi" style="font-size:15px;">check_circle</span> Đã chạy thử thành công (${data.affected || 0} thiết bị thực thi)!</span>`;
        }
      } catch (e) {
        if (statusEl) {
          statusEl.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi chạy thử: ${e.message}</span>`;
        }
      }
    }

