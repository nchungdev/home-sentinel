    async function loadEngines() {
      try {
        const res = await fetch('/api/llm/engines');
        const data = await res.json();
        if (data.ok) {
          enginesList = data.engines;
          renderEngines();
        }
      } catch (e) { console.error(e); }
      loadBridgeAccounts();
    }

    async function loadBridgeAccounts() {
      const container = document.getElementById('bridgeAccountsList');
      if (!container) return;

      try {
        const res = await fetch('/api/bridge/accounts');
        const groups = await res.json();
        if (!groups || !Array.isArray(groups) || groups.length === 0) {
          container.innerHTML = `
            <div style="font-size:12px; color:var(--text-muted); text-align:center; padding:12px; border:1px dashed var(--border); border-radius:8px;">
              Chưa kết nối được Agent Bridge (:8088). Hãy mở <a href="http://${window.location.hostname || 'localhost'}:8000/#settings" target="_blank" style="color:var(--accent);">Agent Bridge Web</a> để đăng nhập tài khoản CLI.
            </div>
          `;
          return;
        }

        let html = '';
        groups.forEach(g => {
          const engineName = g.engine === 'agy' ? 'Google Antigravity (AGY)' : (g.engine === 'claude' ? 'Claude Code (Anthropic)' : 'OpenAI Codex');
          html += `<div style="background:rgba(255,255,255,0.02); border:1px solid var(--border); border-radius:8px; padding:10px 14px;">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:6px;">
              <span style="font-size:12px; font-weight:600; color:var(--text-bright);">${engineName}</span>
              <span style="font-size:10px; color:var(--text-muted);">Engine: <code>${g.engine}</code></span>
            </div>`;

          if (g.accounts && g.accounts.length > 0) {
            html += `<div style="display:flex; flex-direction:column; gap:4px;">`;
            g.accounts.forEach(acc => {
              const activeDot = acc.active ? '<span style="color:#10b981; font-size:10px;">● Đang dùng</span>' : '<span style="color:var(--text-muted); font-size:10px;">(Hồ sơ phụ)</span>';
              html += `
                <div style="font-size:11px; display:flex; justify-content:space-between; align-items:center; background:rgba(0,0,0,0.2); padding:5px 8px; border-radius:6px;">
                  <span style="color:#d1d5db; display:inline-flex; align-items:center; gap:5px;"><span class="mi" style="font-size:14px; color:var(--accent);">person</span> <b>${escapeHtml(acc.label)}</b>: ${escapeHtml(acc.detail || 'Đã đăng nhập')}</span>
                  <span>${activeDot}</span>
                </div>
              `;
            });
            html += `</div>`;
          } else {
            html += `<div style="font-size:11px; color:#ef4444;">Chưa đăng nhập tài khoản nào cho ${g.engine}.</div>`;
          }

          html += `</div>`;
        });

        container.innerHTML = html;
      } catch (e) {
        container.innerHTML = `<div style="font-size:12px; color:#ef4444; padding:8px;">Lỗi đọc tài khoản: ${e.message}</div>`;
      }
    }

    function renderEngines() {
      const container = document.getElementById('engineContainer');
      container.innerHTML = '';
      enginesList.sort((a,b) => a.priority - b.priority);

      enginesList.forEach((e, idx) => {
        const row = document.createElement('div');
        row.className = 'engine-row';
        row.innerHTML = `
          <div class="engine-info">
            <h4>${e.name || e.id} ${e.available ? '<span style="color:var(--accent);">●</span>' : '<span style="color:var(--text-muted);">○</span>'}</h4>
            <p>${e.status_desc || e.statusDesc || (e.available ? 'Sẵn sàng hoạt động' : 'Chưa khả dụng')}</p>
          </div>
          <div style="display:flex; align-items:center; gap:6px;">
            <button class="btn-action" style="padding:5px 8px;" onclick="moveRank(${idx}, -1)" title="Tăng ưu tiên"><span class="mi" style="font-size:16px;">keyboard_arrow_up</span></button>
            <button class="btn-action" style="padding:5px 8px;" onclick="moveRank(${idx}, 1)" title="Giảm ưu tiên"><span class="mi" style="font-size:16px;">keyboard_arrow_down</span></button>
            <span class="badge-rank">Ưu tiên ${idx + 1}</span>
          </div>
        `;
        container.appendChild(row);
      });
    }

    function moveRank(idx, dir) {
      const target = idx + dir;
      if (target < 0 || target >= enginesList.length) return;
      const tmp = enginesList[idx];
      enginesList[idx] = enginesList[target];
      enginesList[target] = tmp;
      renderEngines();
    }

    async function saveAIConfig() {
      const order = enginesList.map(e => e.id);
      const gemini = document.getElementById('geminiKeyInput').value.trim();
      const localUrl = document.getElementById('localUrlInput').value.trim();
      const resDiv = document.getElementById('aiTestResult');
      const btn = document.getElementById('btnSaveAI');
      btn.disabled = true;
      btn.innerHTML = '<span class="mi">hourglass_top</span> Đang lưu...';
      resDiv.innerHTML = '<span style="color:var(--text-muted);">Đang lưu cấu hình AI Engines...</span>';

      try {
        const res = await fetch('/api/llm/engines', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ priority_order: order, gemini_key: gemini, local_llm_url: localUrl })
        });
        const data = await res.json();
        await loadEngines();
        resDiv.innerHTML = `<span style="color:#10b981;"><span class="mi" style="font-size:15px;">check_circle</span> ${data.message || 'Đã lưu cấu hình AI Engines!'}</span>`;
      } catch (e) {
        resDiv.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi lưu cấu hình: ${e.message}</span>`;
      } finally {
        btn.disabled = false;
        btn.innerHTML = '<span class="mi">save</span> Lưu Cấu Hình';
      }
    }

    async function testAIConnection() {
      const resDiv = document.getElementById('aiTestResult');
      resDiv.innerHTML = '<span style="color:var(--text-muted);"><span class="mi" style="font-size:15px;">hourglass_top</span> Đang kiểm tra kết nối các AI Engine...</span>';

      try {
        const localUrl = document.getElementById('localUrlInput').value.trim();
        const gemini = document.getElementById('geminiKeyInput').value.trim();

        // Lưu tạm rồi test
        await fetch('/api/llm/engines', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ priority_order: enginesList.map(e => e.id), gemini_key: gemini, local_llm_url: localUrl })
        });

        const testRes = await fetch('/api/llm/test', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ engine_id: 'local_qwen' })
        });
        const testData = await testRes.json();
        await loadEngines();

        if (testData.ok) {
          resDiv.innerHTML = `<span style="color:#10b981;"><span class="mi" style="font-size:15px;">check_circle</span> Local LLM: ${testData.message}</span>`;
        } else {
          resDiv.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Local LLM: ${testData.message}</span>`;
        }
      } catch (e) {
        resDiv.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi kiểm tra: ${e.message}</span>`;
      }
    }

    // HOME ASSISTANT CONFIG
    async function loadSavedHAConfig() {
      try {
        const res = await fetch('/api/ha/config');
        const data = await res.json();
        if (data.ok && data.config) {
          if (data.config.url) document.getElementById('cfgHaUrl').value = data.config.url;
          if (data.config.token) document.getElementById('cfgHaToken').value = data.config.token;
        }
      } catch (e) { console.error(e); }
    }

    async function saveHAConfig() {
      const url = document.getElementById('cfgHaUrl').value.trim();
      const token = document.getElementById('cfgHaToken').value.trim();
      const resDiv = document.getElementById('haTestResult');
      const btn = document.getElementById('btnSaveHA');

      if (!url) {
        resDiv.innerHTML = '<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Vui lòng nhập URL Home Assistant!</span>';
        return;
      }

      btn.disabled = true;
      btn.innerHTML = '<span class="mi">hourglass_top</span> Đang kiểm tra & lưu...';
      resDiv.innerHTML = '<span style="color:var(--text-muted);">Đang kết nối tới Home Assistant REST API...</span>';

      try {
        const res = await fetch('/api/ha/config', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ url: url, token: token })
        });
        const data = await res.json();
        if (data.ok) {
          resDiv.innerHTML = `<span style="color:#10b981;"><span class="mi" style="font-size:15px;">check_circle</span> ${data.message}</span>`;
        } else {
          resDiv.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Kết nối thất bại: ${data.message}</span>`;
        }
      } catch (e) {
        resDiv.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi: ${e.message}</span>`;
      } finally {
        btn.disabled = false;
        btn.innerHTML = '<span class="mi">save</span> Kiểm Tra & Lưu Cấu Hình';
      }
    }

    async function testHAConnection() {
      saveHAConfig();
    }

    async function generateToken() {
      const res = await fetch('/api/token/generate', { method: 'POST' });
      const data = await res.json();
      if (data.ok) {
        document.getElementById('tokenOutput').innerHTML = `<b>Token mới:</b><br>${data.token}`;
      }
    }

    // CAMERA SCAN & CONFIG
