    // HOME ASSISTANT CONFIG
    async function loadSavedHAConfig() {
      try {
        const res = await fetch('/api/ha/config');
        const data = await res.json();
        if (data.ok && data.config) {
          if (data.config.url) document.getElementById('cfgHaUrl').value = data.config.url;
          if (data.config.token_set) {
            document.getElementById('cfgHaToken').placeholder = 'Đã lưu token (để trống để giữ nguyên)';
          }
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
