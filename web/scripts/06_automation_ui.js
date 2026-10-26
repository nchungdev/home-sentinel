    // AUTOMATION UI LOGIC & STATE (CHUẨN HOME ASSISTANT: WHEN / AND IF / THEN DO)
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

        if (cfgData.ok && cfgData.config) { currentAutoCfg = cfgData.config; ensureNormalizedRules(); }
        if (entData.ok && Array.isArray(entData.entities)) haDevicesCache = entData.entities;
        renderAllAutomationUI();
      } catch (e) {
        if (container) {
          container.innerHTML = `<div style="color:#ef4444; padding:24px; font-size:13px;">Lỗi tải dữ liệu tự động hóa: ${escapeHtml(e.message)}</div>`;
        }
      }
    }

    function ensureNormalizedRules() {
      if (!Array.isArray(currentAutoCfg.rules) || currentAutoCfg.rules.length === 0) {
        currentAutoCfg.rules = [
          {
            id: 'idle', name: 'Tắt đèn khi vắng người', icon: 'bedtime', enabled: true,
            description: 'Tự động tắt các thiết bị khi phòng khách không có người và TV tắt.',
            triggers: [{ type: 'idle_room', idle_minutes: 1 }], conditions: [{ type: 'tv_state', tv_state: 'off' }],
            actions: [{ type: 'entities', service: 'turn_off', target_entities: [] }]
          },
          {
            id: 'clap', name: 'Ngồi Sofa vỗ tay', icon: 'front_hand', enabled: true,
            description: 'Camera phát hiện người ở Sofa kết hợp tiếng vỗ tay sẽ bật thiết bị và mở TV.',
            triggers: [{ type: 'clap_sound', preset: '1' }], conditions: [],
            actions: [{ type: 'entities', service: 'turn_on', target_entities: [] }, { type: 'tv', target_tv: '192.168.1.50' }, { type: 'camera', camera_preset: '1' }]
          },
          {
            id: 'evening', name: 'Phát hiện người buổi tối', icon: 'wb_twilight', enabled: true,
            description: 'Phát hiện có người vào khung giờ buổi tối sẽ tự động bật đèn đón và quay camera.',
            triggers: [{ type: 'person_detected', preset: '3' }], conditions: [{ type: 'time_range', start_hour: 18, end_hour: 6 }],
            actions: [{ type: 'entities', service: 'turn_on', target_entities: [] }, { type: 'camera', camera_preset: '3' }]
          },
          {
            id: 'turn_off_tv_idle', name: 'Tự động tắt TV khi vắng người', icon: 'tv_off', enabled: true,
            description: 'Tự động đưa Android TV vào chế độ ngủ (Sleep) khi phòng khách không có người sau thời gian chờ.',
            triggers: [{ type: 'idle_room', idle_minutes: 5 }], conditions: [{ type: 'tv_state', tv_state: 'on' }],
            actions: [{ type: 'tv', service: 'turn_off', target_tv: '192.168.1.50' }]
          }
        ];
      }
      currentAutoCfg.rules.forEach(r => {
        if (!Array.isArray(r.triggers)) r.triggers = [{ type: r.trigger_type || 'idle_room', idle_minutes: r.idle_minutes || 1, preset: r.trigger_preset || '' }];
        if (!Array.isArray(r.conditions)) r.conditions = [];
        if (!Array.isArray(r.actions)) r.actions = [{ type: 'entities', service: r.action_type || 'turn_off', target_entities: r.target_entities || [] }];
      });
    }

    function collectRuleFromDOM(rule) {
      const id = rule.id;
      const nameInput = document.getElementById(`rule-name-${id}`);
      if (nameInput) rule.name = nameInput.value.trim() || rule.name;

      // Collect Triggers
      (rule.triggers || []).forEach((trig, idx) => {
        const typeSel = document.getElementById(`trig-type-${id}-${idx}`);
        if (typeSel) trig.type = typeSel.value;
        const idleInput = document.getElementById(`trig-idle-${id}-${idx}`);
        if (idleInput) trig.idle_minutes = parseInt(idleInput.value, 10) || 1;
        const presetSel = document.getElementById(`trig-preset-${id}-${idx}`);
        if (presetSel) trig.preset = presetSel.value;
      });

      // Collect Conditions
      (rule.conditions || []).forEach((cond, idx) => {
        const typeSel = document.getElementById(`cond-type-${id}-${idx}`);
        if (typeSel) cond.type = typeSel.value;
        const startInput = document.getElementById(`cond-start-${id}-${idx}`);
        if (startInput) cond.start_hour = parseInt(startInput.value, 10) || 0;
        const endInput = document.getElementById(`cond-end-${id}-${idx}`);
        if (endInput) cond.end_hour = parseInt(endInput.value, 10) || 0;
        const tvSel = document.getElementById(`cond-tv-${id}-${idx}`);
        if (tvSel) cond.tv_state = tvSel.value;
      });

      // Collect Actions
      (rule.actions || []).forEach((act, idx) => {
        const typeSel = document.getElementById(`act-type-${id}-${idx}`);
        if (typeSel) act.type = typeSel.value;
        const srvSel = document.getElementById(`act-srv-${id}-${idx}`);
        if (srvSel) act.service = srvSel.value;
        const tvCmdSel = document.getElementById(`act-tv-cmd-${id}-${idx}`);
        if (tvCmdSel) act.service = tvCmdSel.value;
        const tvInput = document.getElementById(`act-tv-${id}-${idx}`);
        if (tvInput) act.target_tv = tvInput.value.trim();
        const camSel = document.getElementById(`act-cam-${id}-${idx}`);
        if (camSel) act.camera_preset = camSel.value;
        const scriptSel = document.getElementById(`act-script-${id}-${idx}`);
        if (scriptSel) act.script_id = scriptSel.value;

        if (act.type === 'entities') {
          const selected = [];
          document.querySelectorAll(`.device-checkbox-${id}-${idx}:checked`).forEach(cb => {
            selected.push(cb.value);
          });
          act.target_entities = selected;
        }
      });
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
        alert('Lỗi chạy thử: ' + e.message);
      }
    }

    function toggleEditScenario(ruleId) {
      const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
      if (editingRulesSet.has(ruleId)) {
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
      if (rule) { rule.enabled = isChecked; saveAllAutomationRulesUI(); }
    }

    function deleteAutomationRule(ruleId) {
      if (!confirm('Bạn có chắc muốn xóa kịch bản này không?')) return;
      currentAutoCfg.rules = (currentAutoCfg.rules || []).filter(r => r.id !== ruleId);
      editingRulesSet.delete(ruleId);
      saveAllAutomationRulesUI();
      renderAllAutomationUI(currentAutoCfg.rules[0] ? currentAutoCfg.rules[0].id : '');
    }

    function addNewAutomationRule() {
      const newId = 'rule-' + Date.now();
      const count = (currentAutoCfg.rules || []).length + 1;
      const newRule = {
        id: newId, name: `Kịch bản ${count}: Tự động hóa mới`, icon: 'auto_awesome', enabled: true,
        description: 'Tùy chỉnh sự kiện kích hoạt, điều kiện lọc và chuỗi hành động thực thi.',
        triggers: [{ type: 'person_detected', preset: '3' }],
        conditions: [{ type: 'time_range', start_hour: 18, end_hour: 6 }],
        actions: [{ type: 'entities', service: 'turn_on', target_entities: [] }]
      };
      if (!Array.isArray(currentAutoCfg.rules)) currentAutoCfg.rules = [];
      currentAutoCfg.rules.push(newRule);
      editingRulesSet.add(newId);
      renderAllAutomationUI(newId);
    }

    function addStepToRule(ruleId, stepType) {
      const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
      if (!rule) return;
      collectRuleFromDOM(rule);
      if (!Array.isArray(rule[stepType])) rule[stepType] = [];
      if (stepType === 'triggers') rule.triggers.push({ type: 'idle_room', idle_minutes: 5 });
      if (stepType === 'conditions') rule.conditions.push({ type: 'tv_state', tv_state: 'off' });
      if (stepType === 'actions') rule.actions.push({ type: 'entities', service: 'turn_off', target_entities: [] });
      renderAllAutomationUI(ruleId);
    }

    function removeStepFromRule(ruleId, stepType, idx) {
      const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
      if (!rule || !Array.isArray(rule[stepType])) return;
      collectRuleFromDOM(rule);
      rule[stepType].splice(idx, 1);
      renderAllAutomationUI(ruleId);
    }

    function onActionTypeChange(ruleId, idx, newType) {
      const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
      if (!rule || !Array.isArray(rule.actions) || !rule.actions[idx]) return;
      collectRuleFromDOM(rule);
      rule.actions[idx].type = newType;
      renderAllAutomationUI(ruleId);
    }

    function onTriggerTypeChange(ruleId, idx, newType) {
      const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
      if (!rule || !Array.isArray(rule.triggers) || !rule.triggers[idx]) return;
      collectRuleFromDOM(rule);
      rule.triggers[idx].type = newType;
      renderAllAutomationUI(ruleId);
    }

    function onConditionTypeChange(ruleId, idx, newType) {
      const rule = (currentAutoCfg.rules || []).find(r => r.id === ruleId);
      if (!rule || !Array.isArray(rule.conditions) || !rule.conditions[idx]) return;
      collectRuleFromDOM(rule);
      rule.conditions[idx].type = newType;
      renderAllAutomationUI(ruleId);
    }

    function onDeviceCheckboxChange(checkboxEl, ruleId, actionIdx) {
      const label = checkboxEl.closest('.ha-device-item');
      if (label) label.classList.toggle('checked', checkboxEl.checked);
      const count = document.querySelectorAll(`.device-checkbox-${ruleId}-${actionIdx}:checked`).length;
      const badge = document.getElementById(`badge-act-${ruleId}-${actionIdx}`);
      if (badge) badge.textContent = `${count} thiết bị đã chọn`;
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
        item.style.display = (isActive ? (cb && cb.checked) : true) ? 'flex' : 'none';
      });
    }

    function quickSelectByDomain(ruleId, actionIdx, domain) {
      document.querySelectorAll(`.device-checkbox-${ruleId}-${actionIdx}`).forEach(cb => {
        if ((cb.value || '').startsWith(domain + '.')) {
          cb.checked = true;
          const label = cb.closest('.ha-device-item');
          if (label) label.classList.add('checked');
        }
      });
      const count = document.querySelectorAll(`.device-checkbox-${ruleId}-${actionIdx}:checked`).length;
      const badge = document.getElementById(`badge-act-${ruleId}-${actionIdx}`);
      if (badge) badge.textContent = `${count} thiết bị đã chọn`;
    }

    function quickSelectClear(ruleId, actionIdx) {
      document.querySelectorAll(`.device-checkbox-${ruleId}-${actionIdx}`).forEach(cb => {
        cb.checked = false;
        const label = cb.closest('.ha-device-item');
        if (label) label.classList.remove('checked');
      });
      const badge = document.getElementById(`badge-act-${ruleId}-${actionIdx}`);
      if (badge) badge.textContent = `0 thiết bị đã chọn`;
    }
