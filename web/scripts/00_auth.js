    const AUTH_PATHS = ['/api/auth/login', '/api/auth/status', '/api/auth/logout'];
    const nativeFetch = window.fetch.bind(window);
    let loginOverlay = null;

    window.fetch = async function (input, init) {
      const response = await nativeFetch(input, init);
      const url = typeof input === 'string' ? input : input.url;
      const path = new URL(url, location.origin).pathname;
      if (response.status === 401 && !AUTH_PATHS.includes(path)) {
        showLoginOverlay('Phiên đăng nhập đã hết hạn hoặc chưa đăng nhập.');
      }
      return response;
    };

    async function loginWithToken(token) {
      const res = await nativeFetch('/api/auth/login', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ token: token })
      });
      return res.ok;
    }

    function showLoginOverlay(message) {
      if (loginOverlay) return;
      loginOverlay = document.createElement('div');
      loginOverlay.className = 'auth-overlay';
      loginOverlay.innerHTML = `
        <form class="auth-card">
          <div class="auth-title"><span class="mi">lock</span> Đăng nhập Home Sentinel</div>
          <div class="auth-message"></div>
          <input class="auth-input" type="password" autocomplete="current-password"
                 placeholder="Access token (AGENT_TOKEN)" required>
          <button class="btn-modern primary" type="submit">Đăng nhập</button>
        </form>`;
      loginOverlay.querySelector('.auth-message').textContent = message;
      loginOverlay.querySelector('form').addEventListener('submit', async (event) => {
        event.preventDefault();
        const token = loginOverlay.querySelector('.auth-input').value.trim();
        if (await loginWithToken(token)) {
          location.reload();
          return;
        }
        loginOverlay.querySelector('.auth-message').textContent = 'Token không đúng.';
      });
      document.body.appendChild(loginOverlay);
      loginOverlay.querySelector('.auth-input').focus();
    }

    async function initAuth() {
      const fragmentToken = decodeURIComponent(location.hash.slice(1));
      if (fragmentToken) {
        history.replaceState(null, '', location.pathname + location.search);
        if (await loginWithToken(fragmentToken)) return;
      }

      try {
        const res = await nativeFetch('/api/auth/status');
        const status = await res.json();
        if (status.required && !status.authorized) {
          showLoginOverlay('Nhập access token để tiếp tục.');
        }
      } catch (e) { console.error(e); }
    }

    initAuth();
