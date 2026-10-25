    // AUTOMATION UI LOGIC & STATE (MÔ HÌNH KỊCH BẢN TỰ ĐỘNG HÓA TỔNG QUÁT)
    let currentAutoCfg = { rules: [] };
    let haDevicesCache = [];
    const editingRulesSet = new Set();

    async function loadAutomationConfigAndDevices() {
      const container = document.getElementById('autoPanelsContainer');
      try {
        const [cfgRes, entRes] = await Promise.all([
          fetch('/api/automation/config'),
          fetch('/api/ha/entities')
        ]);
        const cfgData = await cfgRes.json();
        const entData = await entRes.json();

        if (cfgData.ok && cfgData.config) {
          currentAutoCfg = cfgData.config;
          if (!Array.isArray(currentAutoCfg.rules) || currentAutoCfg.rules.length === 0) {
            initDefaultRulesOnClient();
          }
        }
        if (entData.ok && Array.isArray(entData.entities)) {
          haDevicesCache = entData.entities;
        }
        renderAllAutomationUI();
      } catch (e) {
        if (container) {
          container.innerHTML = `<div style="color:#ef4444; padding:24px; font-size:13px;">Lỗi tải dữ liệu tự động hóa: ${escapeHtml(e.message)}</div>`;
        }
      }
    }

    function initDefaultRulesOnClient() {
      currentAutoCfg.rules = [
        {
          id: 'idle',
          name: 'Kịch bản 1: Tắt đèn khi vắng người',
          description: 'Khi phòng khách không có người quá số phút quy định và TV đang tắt, tự động tắt toàn bộ thiết bị.',
          icon: 'bedtime',
          enabled: true,
          trigger_type: 'idle_room',
          idle_minutes: 1,
          time_condition: 'all_day',
          tv_condition: 'off',
          action_type: 'turn_off',
          target_entities: [],
          script_id: ''
        },
        {
          id: 'clap',
          name: 'Kịch bản 2: Ngồi Sofa vỗ tay',
          description: 'Khi Camera phát hiện người ngồi ở Sofa và thu được tiếng vỗ tay, đánh thức Android TV và bật thiết bị.',
          icon: 'front_hand',
          enabled: true,
          trigger_type: 'clap_sound',
          trigger_preset: '1',
          cooldown_sec: 60,
          time_condition: 'all_day',
          tv_condition: 'any',
          action_type: 'turn_on',
          target_entities: [],
          target_tv: '192.168.1.8',
          camera_preset: '1'
        },
        {
          id: 'evening',
          name: 'Kịch bản 3: Đèn đón buổi tối',
          description: 'Khi phát hiện có người vào khung giờ buổi tối, tự động bật đèn đón và xoay Camera về cửa chính.',
          icon: 'wb_twilight',
          enabled: true,
          trigger_type: 'person_detected',
          trigger_preset: '3',
          time_condition: 'range',
          start_hour: 18,
          end_hour: 6,
          tv_condition: 'any',
          action_type: 'turn_on',
          target_entities: [],
          camera_preset: '3'
        }
      ];
    }

    function collectRuleFromDOM(rule) {
      const id = rule.id;
      const nameInput = document.getElementById(`rule-name-${id}`);
      if (nameInput) rule.name = nameInput.value.trim() || rule.name;

      const triggerSel = document.getElementById(`rule-trigger-${id}`);
      if (triggerSel) rule.trigger_type = triggerSel.value;

      const idleInput = document.getElementById(`rule-idle-min-${id}`);
      if (idleInput) rule.idle_minutes = parseInt(idleInput.value, 10) || 1;

      const trigPresetSel = document.getElementById(`rule-trigger-preset-${id}`);
      if (trigPresetSel) rule.trigger_preset = trigPresetSel.value;

      const timeCondSel = document.getElementById(`rule-time-cond-${id}`);
      if (timeCondSel) rule.time_condition = timeCondSel.value;

      const startInput = document.getElementById(`rule-start-hour-${id}`);
      if (startInput) rule.start_hour = parseInt(startInput.value, 10) || 0;

      const endInput = document.getElementById(`rule-end-hour-${id}`);
      if (endInput) rule.end_hour = parseInt(endInput.value, 10) || 0;

      const tvCondSel = document.getElementById(`rule-tv-cond-${id}`);
      if (tvCondSel) rule.tv_condition = tvCondSel.value;

      const actionSel = document.getElementById(`rule-action-type-${id}`);
      if (actionSel) rule.action_type = actionSel.value;

      const tvInput = document.getElementById(`rule-target-tv-${id}`);
      if (tvInput) rule.target_tv = tvInput.value.trim();

      const camPresetSel = document.getElementById(`rule-cam-preset-${id}`);
      if (camPresetSel) rule.camera_preset = camPresetSel.value;

      const scriptSel = document.getElementById(`rule-script-id-${id}`);
      if (scriptSel) rule.script_id = scriptSel.value;

      const selectedEntities = [];
      document.querySelectorAll(`.device-checkbox-${id}:checked`).forEach(cb => {
        selectedEntities.push(cb.value);
      });
      rule.target_entities = selectedEntities;
    }

    async function saveAllAutomationRulesUI() {
      const statusEl = document.getElementById('autoSaveStatus');
      const btn = document.getElementById('btnSaveAutomation');
      if (btn) btn.disabled = true;
      if (statusEl) statusEl.textContent = 'Đang lưu kịch bản...';

      if (Array.isArray(currentAutoCfg.rules)) {
        currentAutoCfg.rules.forEach(r => collectRuleFromDOM(r));
      }

      try {
        const res = await fetch('/api/automation/config', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ config: currentAutoCfg })
        });
        const data = await res.json();
        if (data.ok) {
          if (statusEl) statusEl.textContent = '✓ Đã lưu kịch bản thành công!';
          setTimeout(() => {
            if (statusEl) statusEl.textContent = 'Lưu vào configs/automation.json';
          }, 3000);
        } else {
          if (statusEl) statusEl.textContent = 'Lỗi lưu: ' + (data.message || 'thất bại');
        }
      } catch (e) {
        if (statusEl) statusEl.textContent = 'Lỗi kết nối: ' + e.message;
      } finally {
        if (btn) btn.disabled = false;
      }
    }

    async function testAutomationRule(ruleId) {
      try {
        const res = await fetch('/api/automation/config', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ test_rule: ruleId })
        });
        const data = await res.json();
        const affected = data.affected !== undefined ? data.affected : 0;
        alert(`Đã chạy thử kịch bản thành công! Số thiết bị tác động: ${affected}`);
      } catch (e) {
        alert('Lỗi chạy thử kịch bản: ' + e.message);
      }
    }

    function toggleEditScenario(ruleId) {
      if (editingRulesSet.has(ruleId)) {
        const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
        if (rule) collectRuleFromDOM(rule);
        editingRulesSet.delete(ruleId);
        saveAllAutomationRulesUI();
      } else {
        editingRulesSet.add(ruleId);
      }
      renderAllAutomationUI(ruleId);
    }

    function toggleRuleEnabled(ruleId, isChecked) {
      const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
      if (rule) {
        rule.enabled = isChecked;
        saveAllAutomationRulesUI();
      }
    }

    function deleteAutomationRule(ruleId) {
      if (!confirm('Bạn có chắc muốn xóa kịch bản này không?')) return;
      currentAutoCfg.rules = (currentAutoCfg.rules || []).filter(r => r.id !== ruleId);
      editingRulesSet.delete(ruleId);
      saveAllAutomationRulesUI();
      const firstTab = currentAutoCfg.rules[0] ? currentAutoCfg.rules[0].id : '';
      renderAllAutomationUI(firstTab);
    }

    function addNewAutomationRule() {
      const newId = 'rule-' + Date.now();
      const count = (currentAutoCfg.rules || []).length + 1;
      const newRule = {
        id: newId,
        name: `Kịch bản ${count}: Tùy chỉnh mới`,
        description: 'Tùy chỉnh hành động khi sự kiện xảy ra theo chuẩn Home Assistant.',
        icon: 'auto_awesome',
        enabled: true,
        trigger_type: 'person_detected',
        trigger_preset: '3',
        time_condition: 'all_day',
        start_hour: 18,
        end_hour: 6,
        tv_condition: 'any',
        action_type: 'turn_on',
        target_entities: [],
        camera_preset: '3'
      };
      if (!Array.isArray(currentAutoCfg.rules)) currentAutoCfg.rules = [];
      currentAutoCfg.rules.push(newRule);
      editingRulesSet.add(newId);
      renderAllAutomationUI(newId);
    }

    function onDeviceCheckboxChange(checkboxEl, ruleId) {
      const label = checkboxEl.closest('.ha-device-item');
      if (label) label.classList.toggle('checked', checkboxEl.checked);
      updateBadgeCount(ruleId);
    }

    function updateBadgeCount(ruleId) {
      const count = document.querySelectorAll(`.device-checkbox-${ruleId}:checked`).length;
      const badge = document.getElementById(`badge-count-${ruleId}`);
      if (badge) badge.textContent = `${count} thiết bị`;
    }

    function filterDeviceGrid(gridId, query) {
      const term = (query || '').trim().toLowerCase();
      const grid = document.getElementById(gridId);
      if (!grid) return;
      grid.querySelectorAll('.ha-device-item').forEach(item => {
        const name = item.getAttribute('data-device-name') || '';
        item.style.display = (!term || name.includes(term)) ? 'flex' : 'none';
      });
    }

    function filterSelectedOnly(gridId, btnEl) {
      const grid = document.getElementById(gridId);
      if (!grid) return;
      const isActive = btnEl.classList.toggle('active');
      grid.querySelectorAll('.ha-device-item').forEach(item => {
        const cb = item.querySelector('input[type="checkbox"]');
        if (isActive) {
          item.style.display = (cb && cb.checked) ? 'flex' : 'none';
        } else {
          item.style.display = 'flex';
        }
      });
    }

    function quickSelectByDomain(ruleId, domain) {
      document.querySelectorAll(`.device-checkbox-${ruleId}`).forEach(cb => {
        const val = cb.value || '';
        if (val.startsWith(domain + '.')) {
          cb.checked = true;
          const label = cb.closest('.ha-device-item');
          if (label) label.classList.add('checked');
        }
      });
      updateBadgeCount(ruleId);
    }

    function quickSelectClear(ruleId) {
      document.querySelectorAll(`.device-checkbox-${ruleId}`).forEach(cb => {
        cb.checked = false;
        const label = cb.closest('.ha-device-item');
        if (label) label.classList.remove('checked');
      });
      updateBadgeCount(ruleId);
    }
