    let engineLoginTimer = null;
    let engineLoginWindow = null;
    let openedEngineLoginURL = '';
    let engineLoginGeneration = 0;

    function closePendingLoginWindow() {
      if (engineLoginWindow && !openedEngineLoginURL && !engineLoginWindow.closed) {
        engineLoginWindow.close();
      }
      engineLoginWindow = null;
      openedEngineLoginURL = '';
    }

    async function startEngineLogin() {
      if (!engineAuthReport) { await loadEngineAuth(); return; }
      if (engineAuthReport.status.authenticated
        && !document.getElementById('engineAuthSecret').value.trim()) return;
      const method = engineAuthReport.methods.find(item => item.id === selectedAuthMethod);
      if (method.kind === 'environment') { await loadEngineAuth(selectedAuthMethod); return; }
      const secretInput = document.getElementById('engineAuthSecret');
      const secret = secretInput.value.trim();
      if (method.kind === 'secret' && !secret) {
        showEngineResult('Vui lòng nhập API key hoặc token.', true);
        return;
      }
      const engineID = selectedEngine;
      const generation = ++engineLoginGeneration;
      closePendingLoginWindow();
      if (method.kind === 'browser') {
        engineLoginWindow = window.open('about:blank', '_blank');
        if (engineLoginWindow) {
          engineLoginWindow.opener = null;
          engineLoginWindow.document.body.textContent = 'Đang chuẩn bị trang đăng nhập…';
        }
      }
      clearTimeout(engineLoginTimer);
      setLoginPending(true);
      showEngineResult('Đang khởi chạy luồng xác thực trên máy chủ…');
      document.getElementById('engineLoginLink').hidden = true;
      document.getElementById('engineLoginCodeFields').hidden = true;
      try {
        const data = await engineRequest('/api/llm/login', {
          engine_id: engineID, method: selectedAuthMethod, secret,
          email: document.getElementById('engineAuthEmail').value.trim()
        });
        if (generation !== engineLoginGeneration) return;
        secretInput.value = '';
        renderLoginState(engineID, data.login, generation);
      } catch (error) {
        if (generation !== engineLoginGeneration) return;
        showEngineResult(error.message, true);
        setLoginPending(false);
        closePendingLoginWindow();
      }
    }

    function setLoginPending(pending) {
      document.getElementById('btnEngineLogin').disabled = pending;
      document.getElementById('engineAuthTrigger').disabled = pending;
      document.getElementById('btnCancelEngineLogin').hidden = !pending;
    }

    function renderLoginState(engineID, state, generation) {
      if (generation !== engineLoginGeneration || selectedEngine !== engineID) return;
      document.getElementById('engineLoginOutput').textContent = (state.output || '').slice(-6000);
      document.getElementById('engineLoginNavigation').hidden =
        !state.running || engineID !== 'agy' || state.method !== 'google';
      if (state.running && engineID === 'agy' && state.output && !state.login_url) {
        document.getElementById('engineLoginDetails').open = true;
      }
      setLoginPending(state.running);
      if (state.login_url && state.running) {
        const link = document.getElementById('engineLoginLink');
        link.href = state.login_url;
        link.hidden = false;
        showEngineResult('Hoàn tất xác thực trên trang đăng nhập của nhà cung cấp.');
        if (openedEngineLoginURL !== state.login_url) {
          openedEngineLoginURL = state.login_url;
          if (engineLoginWindow && !engineLoginWindow.closed) {
            engineLoginWindow.location.href = state.login_url;
          }
        }
      }
      document.getElementById('engineLoginCodeFields').hidden =
        !state.running || !state.login_url || engineID === 'codex';
      if (!state.running) {
        closePendingLoginWindow();
        document.getElementById('engineLoginLink').hidden = true;
        showEngineResult(state.message || 'Phiên đăng nhập đã kết thúc.', !state.succeeded);
        if (state.succeeded) loadEngineAuth(selectedAuthMethod);
        return;
      }
      engineLoginTimer = setTimeout(async () => {
        try {
          const data = await engineRequest('/api/llm/login?engine_id=' + engineID);
          renderLoginState(engineID, data.login, generation);
        } catch (error) {
          if (generation !== engineLoginGeneration) return;
          showEngineResult(error.message, true);
          setLoginPending(false);
          closePendingLoginWindow();
        }
      }, 1500);
    }

    async function cancelEngineLogin() {
      const engineID = selectedEngine;
      engineLoginGeneration++;
      clearTimeout(engineLoginTimer);
      closePendingLoginWindow();
      try {
        await engineRequest('/api/llm/login', {engine_id: engineID, action: 'cancel'});
        if (selectedEngine === engineID) {
          setLoginPending(false);
          await loadEngineAuth(selectedAuthMethod);
          showEngineResult('Đã hủy phiên đăng nhập.');
        }
      } catch (error) { showEngineResult(error.message, true); }
    }

    async function submitEngineLoginCode() {
      const input = document.getElementById('engineLoginCode');
      const code = input.value.trim();
      if (!code) { showEngineResult('Vui lòng nhập mã xác thực.', true); return; }
      const engineID = selectedEngine;
      const generation = engineLoginGeneration;
      try {
        await engineRequest('/api/llm/login', {engine_id: engineID, code});
        if (generation !== engineLoginGeneration || engineID !== selectedEngine) return;
        input.value = '';
        showEngineResult('Đã gửi thông tin xác thực tới CLI trên máy chủ.');
      } catch (error) { showEngineResult(error.message, true); }
    }

    async function navigateEngineLogin(key) {
      try {
        await engineRequest('/api/llm/login', {engine_id: selectedEngine, key});
      } catch (error) { showEngineResult(error.message, true); }
    }
