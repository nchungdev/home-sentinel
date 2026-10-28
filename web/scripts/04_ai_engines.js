    let selectedEngine = '';
    let savedEngineConfig = {};
    let lastEngineUsage = null;
    let engineDrafts = {};
    let engineRequestPending = false;
    const engineKeyNames = {
      gemini: 'gemini_key', openai_api: 'openai_key', anthropic_api: 'anthropic_key'
    };
    const engineModelNames = {
      openai_api: 'openai_model', anthropic_api: 'anthropic_model'
    };

    async function engineRequest(path, body) {
      const options = body === undefined ? {} : {
        method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(body)
      };
      const response = await fetch(path, options);
      const data = await response.json();
      if (!response.ok || !data.ok) {
        throw new Error(data.error || data.message || 'Yêu cầu thất bại');
      }
      return data;
    }

    async function loadEngines() {
      if (engineRequestPending) return;
      try {
        if (savedEngineConfig.selected_engine) rememberEngineDraft();
        const data = await engineRequest('/api/llm/engines');
        enginesList = data.engines;
        savedEngineConfig = data.config;
        lastEngineUsage = data.last_used;
        if (!selectedEngine) selectedEngine = savedEngineConfig.selected_engine;
        renderEngines();
        renderEngineConfig();
      } catch (error) {
        showEngineResult(error.message, true);
      }
    }

    function rememberEngineDraft() {
      if (!selectedEngine) return;
      engineDrafts[selectedEngine] = {
        key: document.getElementById('engineApiKey').value,
        model: document.getElementById('engineModel').value,
        url: document.getElementById('localUrlInput').value
      };
    }

    function chooseEngine(engineID) {
      if (engineRequestPending) return;
      rememberEngineDraft();
      selectedEngine = engineID;
      showEngineResult('');
      renderEngines();
      renderEngineConfig();
    }

    function renderEngines() {
      const container = document.getElementById('engineOptions');
      container.replaceChildren();
      document.getElementById('engineContainer').disabled = engineRequestPending;
      enginesList.forEach(engine => {
        const option = document.createElement('button');
        option.type = 'button';
        option.className = 'engine-picker-option';
        option.setAttribute('role', 'option');
        option.setAttribute('aria-selected', String(engine.id === selectedEngine));
        option.tabIndex = -1;
        option.textContent = engine.name;
        option.onclick = () => {
          closeEnginePicker();
          chooseEngine(engine.id);
          document.getElementById('engineContainer').focus();
        };
        option.onkeydown = engineOptionKey;
        container.appendChild(option);
      });
      const selected = enginesList.find(engine => engine.id === selectedEngine);
      document.getElementById('engineSelectionStatus').textContent = selected?.status_desc || '';
      document.getElementById('engineSelectedName').textContent = selected?.name || 'Chọn engine';
      const saved = enginesList.find(engine => engine.id === savedEngineConfig.selected_engine);
      const banner = document.getElementById('activeEngineBanner');
      banner.textContent = `Đã lưu: ${saved ? saved.name : 'Chưa chọn engine'}`;
      if (lastEngineUsage) {
        const last = enginesList.find(engine => engine.id === lastEngineUsage.engine_id);
        const usage = document.createElement('div');
        usage.className = 'engine-help';
        usage.textContent = `Trả lời gần nhất: ${last ? last.name : lastEngineUsage.engine_id}`
          + ` · ${new Date(lastEngineUsage.at).toLocaleString('vi-VN')}`;
        banner.appendChild(usage);
      }
    }

    function renderEngineConfig() {
      const engine = enginesList.find(item => item.id === selectedEngine);
      if (!engine) return;
      const keyName = engineKeyNames[selectedEngine];
      const modelName = engineModelNames[selectedEngine];
      const isCLI = ['codex', 'claude', 'agy'].includes(selectedEngine);
      document.getElementById('engineConfigTitle').textContent = engine.name;
      document.getElementById('engineKeyFields').hidden = !keyName;
      document.getElementById('engineModelField').hidden = !modelName;
      document.getElementById('engineLocalFields').hidden = selectedEngine !== 'local_qwen';
      document.getElementById('engineLoginFields').hidden = !isCLI;
      const draft = engineDrafts[selectedEngine] || {};
      const keyInput = document.getElementById('engineApiKey');
      keyInput.value = draft.key || '';
      keyInput.placeholder = savedEngineConfig[keyName + '_set']
        ? 'Đã lưu key · để trống để giữ nguyên' : 'Nhập API key';
      document.getElementById('engineModel').value =
        draft.model ?? savedEngineConfig[modelName] ?? '';
      document.getElementById('localUrlInput').value =
        draft.url ?? savedEngineConfig.local_llm_url ?? '';
      const help = selectedEngine === 'rule_fallback'
        ? 'Tắt phần trò chuyện AI.'
        : isCLI ? 'Sử dụng phiên đăng nhập CLI trên máy chạy Sentinel.'
        : keyName ? 'Dùng API key của nhà cung cấp. Kiểm tra sẽ gửi một yêu cầu nhỏ tới model.'
        : 'Endpoint truy cập từ máy chạy Sentinel; 127.0.0.1 là chính máy đó.';
      document.getElementById('engineConfigHelp').textContent = help;
      resetEngineLoginUI();
      if (isCLI) loadEngineAuth();
    }


    function enginePayload() {
      const payload = {selected_engine: selectedEngine};
      if (['codex', 'claude', 'agy'].includes(selectedEngine)) {
        if (engineAuthLoading || !engineAuthReport) {
          throw new Error('Chờ kiểm tra phiên xác thực trên máy chủ.');
        }
        if (document.getElementById('engineAuthSecret').value.trim()) {
          throw new Error('Lưu API key/token mới trước khi lưu engine.');
        }
      }
      if (selectedAuthMethod) {
        if (!engineAuthReport?.status.authenticated) {
          throw new Error('Vui lòng xác thực phương thức đã chọn trước khi sử dụng.');
        }
        payload.auth_method = selectedAuthMethod;
      }
      const keyName = engineKeyNames[selectedEngine];
      const modelName = engineModelNames[selectedEngine];
      if (keyName) payload[keyName] = document.getElementById('engineApiKey').value.trim();
      if (modelName) payload[modelName] = document.getElementById('engineModel').value.trim();
      if (modelName && !payload[modelName]) throw new Error('Vui lòng nhập model ID');
      if (keyName && !payload[keyName] && !savedEngineConfig[keyName + '_set']) {
        throw new Error('Vui lòng nhập API key');
      }
      if (selectedEngine === 'local_qwen') {
        payload.local_llm_url = document.getElementById('localUrlInput').value.trim();
        if (!payload.local_llm_url) throw new Error('Vui lòng nhập endpoint');
      }
      return payload;
    }

    function showEngineResult(message, failed = false) {
      const result = document.getElementById('aiTestResult');
      result.textContent = message;
      result.style.color = failed ? '#fca5a5' : 'var(--text-muted)';
    }

    async function runEngineAction(save) {
      if (engineRequestPending) return;
      try {
        const payload = enginePayload();
        engineRequestPending = true;
        document.getElementById('btnSaveAI').disabled = true;
        document.getElementById('btnTestAI').disabled = true;
        renderEngines();
        showEngineResult(save ? 'Đang lưu…' : 'Đang gửi yêu cầu kiểm tra…');
        const data = await engineRequest(save ? '/api/llm/engines' : '/api/llm/test', payload);
        if (save) {
          delete engineDrafts[selectedEngine];
          document.getElementById('engineApiKey').value = '';
          engineRequestPending = false;
          await loadEngines();
        }
        showEngineResult(data.message || 'Đã lưu và sử dụng engine đã chọn');
      } catch (error) {
        showEngineResult(error.message, true);
      } finally {
        engineRequestPending = false;
        document.getElementById('btnSaveAI').disabled = false;
        document.getElementById('btnTestAI').disabled = false;
        renderEngines();
      }
    }

    function saveAIConfig() { return runEngineAction(true); }
    function testAIConnection() { return runEngineAction(false); }
