    async function scanNetworkCameras() {
      const btn = document.getElementById('btnScanCam');
      const list = document.getElementById('scanResultList');
      btn.disabled = true;
      btn.innerHTML = '<span class="mi">hourglass_top</span> Đang quét ONVIF...';
      list.innerHTML = `
        <div style="padding:14px; text-align:center; color:var(--text-muted); font-size:13px;">
          Đang phát sóng ONVIF WS-Discovery dò tìm thiết bị trên mạng...
        </div>`;

      try {
        const res = await fetch('/api/camera/scan');
        const data = await res.json();
        list.innerHTML = '';

        if (!data.ok || !data.devices || data.devices.length === 0) {
          list.innerHTML = `
            <div style="padding:14px; text-align:center; color:#f59e0b; font-size:13px; border:1px solid rgba(245,158,11,0.2); border-radius:8px;">
              Không tìm thấy thiết bị ONVIF tự động. Bạn có thể nhập địa chỉ IP thủ công ở bảng bên dưới.
            </div>`;
          return;
        }

        data.devices.forEach(dev => {
          const item = document.createElement('div');
          item.className = 'engine-row';
          item.style.cursor = 'pointer';
          item.innerHTML = `
            <div class="engine-info">
              <h4>${dev.manufacturer} - ${dev.name} (${dev.ip})</h4>
              <p>Hardware: ${dev.hardware || 'IP Camera'} | ONVIF Port: ${dev.onvif_port || 2020}</p>
            </div>
            <button class="btn-save" style="padding:7px 14px; font-size:12px;" onclick="selectDevice('${dev.ip}', '${dev.name}', '${dev.manufacturer}')">
              <span class="mi" style="font-size:15px;">check</span> Chọn & Cấu Hình
            </button>
          `;
          list.appendChild(item);
        });
      } catch (e) {
        list.innerHTML = `<div style="color:#ef4444; font-size:13px;">Lỗi quét thiết bị: ${e.message}</div>`;
      } finally {
        btn.disabled = false;
        btn.innerHTML = '<span class="mi">radar</span> Quét Mạng LAN';
      }
    }

    function selectDevice(ip, name, mfg) {
      document.getElementById('cfgCamIp').value = ip;
      document.getElementById('cfgCamModel').value = name;
      
      const brandSelect = document.getElementById('cfgCamBrand');
      const lower = (mfg + ' ' + name).toLowerCase();
      if (lower.includes('tapo')) brandSelect.value = 'tapo';
      else if (lower.includes('imou') || lower.includes('dahua')) brandSelect.value = 'imou';
      else if (lower.includes('ezviz') || lower.includes('hik')) brandSelect.value = 'ezviz';
      else brandSelect.value = 'onvif';

      if (brandSelect.value === 'tapo') {
        document.getElementById('cfgCamStreamPath').value = '/stream1';
      } else if (brandSelect.value === 'imou' || brandSelect.value === 'ezviz') {
        document.getElementById('cfgCamStreamPath').value = '/h264Preview_01_main';
      }

      const resDiv = document.getElementById('camTestResult');
      resDiv.innerHTML = `<span style="color:#10b981;"><span class="mi" style="font-size:15px;">touch_app</span> Đã chọn thiết bị <b>${name} (${ip})</b>. Nhập tài khoản/mật khẩu rồi bấm Lưu hoặc Thử Kết Nối.</span>`;
    }

    async function loadSavedCameraConfig() {
      try {
        const res = await fetch('/api/camera/config');
        const data = await res.json();
        if (data.ok && data.config && data.config.ip) {
          const c = data.config;
          document.getElementById('cfgCamIp').value = c.ip || '';
          document.getElementById('cfgCamBrand').value = c.brand || 'tapo';
          document.getElementById('cfgCamModel').value = c.model || '';
          document.getElementById('cfgCamUser').value = c.username || '';
          document.getElementById('cfgCamPass').value = c.password || '';
          document.getElementById('cfgCamCloudPass').value = c.cloud_password || '';
          document.getElementById('cfgCamStreamPath').value = c.stream_path || '/stream1';
        }
      } catch (e) { console.error(e); }
    }

    async function saveCameraConfig() {
      const resDiv = document.getElementById('camTestResult');
      const payload = {
        ip: document.getElementById('cfgCamIp').value.trim(),
        brand: document.getElementById('cfgCamBrand').value,
        model: document.getElementById('cfgCamModel').value.trim(),
        username: document.getElementById('cfgCamUser').value.trim(),
        password: document.getElementById('cfgCamPass').value.trim(),
        cloud_password: document.getElementById('cfgCamCloudPass').value.trim(),
        stream_path: document.getElementById('cfgCamStreamPath').value.trim()
      };

      if (!payload.ip) {
        resDiv.innerHTML = '<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Vui lòng nhập địa chỉ IP camera!</span>';
        return;
      }

      resDiv.innerHTML = '<span style="color:var(--text-muted);">Đang lưu cấu hình camera...</span>';
      try {
        const res = await fetch('/api/camera/config', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        resDiv.innerHTML = `<span style="color:#10b981;"><span class="mi" style="font-size:15px;">check_circle</span> ${data.message || 'Đã lưu cấu hình camera!'}</span>`;
      } catch (e) {
        resDiv.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi lưu cấu hình: ${e.message}</span>`;
      }
    }

    async function testCameraConnection() {
      const ip = document.getElementById('cfgCamIp').value.trim();
      const user = document.getElementById('cfgCamUser').value.trim();
      const pass = document.getElementById('cfgCamPass').value.trim();
      const path = document.getElementById('cfgCamStreamPath').value.trim();
      const resDiv = document.getElementById('camTestResult');
      const btn = document.getElementById('btnTestCam');

      if (!ip) {
        resDiv.innerHTML = '<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Vui lòng nhập địa chỉ IP!</span>';
        return;
      }

      btn.disabled = true;
      btn.innerHTML = '<span class="mi">hourglass_top</span> Đang thử RTSP...';
      resDiv.innerHTML = `<span style="color:var(--text-muted);">Đang kết nối RTSP rtsp://${ip}:554${path}...</span>`;

      try {
        const res = await fetch('/api/camera/test-rtsp', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({ ip: ip, port: 554, user: user, password: pass, path: path })
        });
        const data = await res.json();
        if (data.success) {
          resDiv.innerHTML = `<span style="color:#10b981;"><span class="mi" style="font-size:15px;">check_circle</span> Kết nối thành công! ${data.message}</span>`;
        } else {
          resDiv.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Kết nối thất bại: ${data.message || 'Không thể liên lạc'}</span>`;
        }
      } catch (e) {
        resDiv.innerHTML = `<span style="color:#ef4444;"><span class="mi" style="font-size:15px;">error</span> Lỗi kiểm tra RTSP: ${e.message}</span>`;
      } finally {
        btn.disabled = false;
        btn.innerHTML = '<span class="mi">bolt</span> Thử Kết Nối';
      }
    }

