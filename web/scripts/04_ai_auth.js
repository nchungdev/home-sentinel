    let selectedAuthMethod = '';
    let engineAuthReport = null;
    let engineAuthGeneration = 0;
    let engineAuthLoading = false;

    function resetEngineLoginUI() {
      engineAuthGeneration++;
      selectedAuthMethod = '';
      engineAuthReport = null;
      clearTimeout(engineLoginTimer);
      engineLoginGeneration++;
      closePendingLoginWindow();
      document.getElementById('engineLoginLink').hidden = true;
      document.getElementById('engineLoginCodeFields').hidden = true;
      document.getElementById('engineLoginNavigation').hidden = true;
      document.getElementById('engineLoginDetails').open = false;
      document.getElementById('btnCancelEngineLogin').hidden = true;
      document.getElementById('engineAuthSecretFields').hidden = true;
      document.getElementById('engineAuthSecret').value = '';
      document.getElementById('engineAuthEmail').value = '';
      document.getElementById('engineLoginCode').value = '';
      document.getElementById('engineLoginOutput').textContent = '';
      document.getElementById('engineAuthStatus').textContent = '';
      document.getElementById('engineAuthMethodLabel').textContent = 'Đang kiểm tra…';
      document.getElementById('engineAuthOptions').replaceChildren();
      document.getElementById('engineAuthTrigger').disabled = true;
    }

    async function loadEngineAuth(method = '') {
      const engineID = selectedEngine;
      const generation = ++engineAuthGeneration;
      engineAuthLoading = true;
      document.getElementById('engineAuthTrigger').disabled = true;
      const button = document.getElementById('btnEngineLogin');
      button.disabled = true;
      document.getElementById('engineAuthStatus').textContent = 'Đang kiểm tra phiên trên máy chủ…';
      try {
        const query = new URLSearchParams({engine_id: engineID});
        if (method) query.set('method', method);
        const data = await engineRequest('/api/llm/auth?' + query);
        if (generation !== engineAuthGeneration || engineID !== selectedEngine) return;
        engineAuthLoading = false;
        engineAuthReport = data.auth;
        selectedAuthMethod = data.auth.selected_method;
        renderAuthMethods();
        renderAuthStatus();
        if (data.auth.login.running && data.auth.login.method === selectedAuthMethod) {
          engineLoginGeneration++;
          renderLoginState(engineID, data.auth.login, engineLoginGeneration);
        }
      } catch (error) {
        if (generation === engineAuthGeneration) {
          engineAuthLoading = false;
          document.getElementById('engineAuthStatus').textContent = error.message;
          button.textContent = 'Thử kiểm tra lại';
          button.disabled = false;
        }
      }
    }

    function renderAuthMethods() {
      const options = document.getElementById('engineAuthOptions');
      options.replaceChildren();
      engineAuthReport.methods.forEach(method => {
        const option = document.createElement('button');
        option.type = 'button';
        option.className = 'engine-picker-option';
        option.setAttribute('role', 'option');
        option.setAttribute('aria-selected', String(method.id === selectedAuthMethod));
        option.tabIndex = -1;
        option.textContent = method.label;
        option.onkeydown = engineOptionKey;
        option.onclick = () => {
          closePicker('engineAuthTrigger', 'engineAuthOptions');
          document.getElementById('engineAuthTrigger').focus();
          clearTimeout(engineLoginTimer);
          engineLoginGeneration++;
          closePendingLoginWindow();
          document.getElementById('engineAuthSecret').value = '';
          loadEngineAuth(method.id);
        };
        options.appendChild(option);
      });
      const method = engineAuthReport.methods.find(item => item.id === selectedAuthMethod);
      document.getElementById('engineAuthMethodLabel').textContent = method.label;
    }

    function renderAuthStatus() {
      const status = engineAuthReport.status;
      const method = engineAuthReport.methods.find(item => item.id === selectedAuthMethod);
      const details = status.account ? status.message + ' · ' + status.account : status.message;
      document.getElementById('engineAuthStatus').textContent = details;
      const button = document.getElementById('btnEngineLogin');
      button.disabled = status.authenticated;
      button.textContent = status.authenticated ? 'Đã xác thực trên máy chủ'
        : method.kind === 'secret' ? 'Lưu API key/token'
        : method.kind === 'environment' ? 'Kiểm tra cấu hình máy chủ' : 'Đăng nhập';
      document.getElementById('engineAuthSecretFields').hidden =
        method.kind !== 'secret';
      document.getElementById('engineAuthTrigger').disabled = false;
      document.getElementById('btnCancelEngineLogin').hidden = true;
      document.getElementById('engineAuthEmailFields').hidden =
        status.authenticated || selectedAuthMethod !== 'sso';
      const commands = {
        'codex/chatgpt': 'codex login --device-auth',
        'claude/subscription': 'claude auth login --claudeai',
        'claude/console': 'claude auth login --console',
        'claude/sso': 'claude auth login --sso',
        'agy/google': 'agy',
        'agy/adc': 'gcloud auth application-default login --no-launch-browser'
      };
      document.getElementById('engineLoginCommand').textContent =
        commands[selectedEngine + '/' + selectedAuthMethod] || 'Dùng API key/token trong biểu mẫu.';
    }

    function updateAuthSecretButton() {
      if (!engineAuthReport) return;
      const present = !!document.getElementById('engineAuthSecret').value.trim();
      const button = document.getElementById('btnEngineLogin');
      button.disabled = engineAuthReport.status.authenticated && !present;
      button.textContent = present ? 'Lưu API key/token' : engineAuthReport.status.authenticated
        ? 'Đã xác thực trên máy chủ' : 'Lưu API key/token';
    }
