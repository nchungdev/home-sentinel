    // REST API TESTER PLAYGROUND
    const API_PRESETS = [
      {
        group: 'Conversational & AI Agent',
        routes: [
          { name: 'AI Chat Stream (v2)', method: 'POST', path: '/v2/ask', body: JSON.stringify({ text: 'Thời tiết hôm nay thế nào?', session: 'test-session' }, null, 2) },
          { name: 'Live Request Flow', method: 'POST', path: '/v2/flow', body: JSON.stringify({ text: 'Naruto' }, null, 2) },
          { name: 'Request Flow Preview', method: 'POST', path: '/v2/flow/preview', body: JSON.stringify({ text: 'Bật đèn phòng khách' }, null, 2) },
          { name: 'Contextual Suggestions', method: 'GET', path: '/v2/suggestions' },
          { name: 'Test LLM Engine', method: 'POST', path: '/api/llm/test', body: JSON.stringify({ text: 'Xin chào Home Sentinel' }, null, 2) },
          { name: 'Get LLM Engines Config', method: 'GET', path: '/api/llm/engines' },
          { name: 'Save LLM Engines Config', method: 'POST', path: '/api/llm/engines', body: JSON.stringify({ selected_engine: 'gemini', gemini_key: 'AIzaSy...' }, null, 2) }
        ]
      },
      {
        group: 'Media & Catalog (Chuẩn thiết kế)',
        routes: [
          { name: '1. API Tìm kiếm (TMDb + YouTube)', method: 'POST', path: '/v2/flow', body: JSON.stringify({ text: 'Loki' }, null, 2) },
          { name: '2. API Chi tiết & Đối soát Nguồn', method: 'GET', path: '/v2/media/tmdb/{kind}/{id}', pathParams: { kind: 'tv', id: '84958' } },
          { name: '3. Fallback AI Search Discovery', method: 'POST', path: '/v2/media/tmdb/{kind}/{id}/discover', pathParams: { kind: 'tv', id: '84958' } },
          { name: '4. API Danh sách Tập (Catalog)', method: 'GET', path: '/v2/catalog/titles/{id}', pathParams: { id: 'phimapi:loki-phan-1' } },
          { name: '5. API Giải mã Link phát Stream', method: 'POST', path: '/v2/catalog/episodes/{id}/playback', pathParams: { id: 'phimapi:loki-phan-1:ep-1' } },
          { name: '6. API Yêu cầu Tải phim (Radarr/Sonarr)', method: 'POST', path: '/api/media/request', body: JSON.stringify({ source_id: 'sonarr', item_id: '84958' }, null, 2) },
          { name: '7. Trạng thái Kết nối các Dịch vụ', method: 'GET', path: '/api/media/sources', queryParams: { refresh: '1' } },
          { name: '8. Lấy Cấu hình API Keys', method: 'GET', path: '/api/media/config' },
          { name: '9. Lưu Cấu hình API Keys', method: 'POST', path: '/api/media/config', body: JSON.stringify({ tmdb_api_key: '', youtube_api_key: '' }, null, 2) }
        ]
      },
      {
        group: 'Home Assistant & Automation',
        routes: [
          { name: 'Get Live HA Entities', method: 'GET', path: '/api/ha/entities' },
          { name: 'Call HA Service', method: 'POST', path: '/api/ha/entities', body: JSON.stringify({ domain: 'light', service: 'turn_on', entity_id: 'light.living_room' }, null, 2) },
          { name: 'Get Automation Config', method: 'GET', path: '/api/automation/config' },
          { name: 'Save Automation Config', method: 'POST', path: '/api/automation/config', body: JSON.stringify({ rules: [] }, null, 2) },
          { name: 'Get TV Modes List', method: 'GET', path: '/v2/modes' },
          { name: 'Activate TV Mode', method: 'POST', path: '/v2/modes/{id}/activate', pathParams: { id: 'movie' } }
        ]
      },
      {
        group: 'Camera & Vision',
        routes: [
          { name: 'Scan Cameras (ONVIF)', method: 'POST', path: '/api/camera/scan' },
          { name: 'Get Camera Config', method: 'GET', path: '/api/camera/config' },
          { name: 'Save Camera Config', method: 'POST', path: '/api/camera/config', body: JSON.stringify({ cameras: [] }, null, 2) },
          { name: 'Test Camera RTSP', method: 'POST', path: '/api/camera/test-rtsp', body: JSON.stringify({ rtsp_url: 'rtsp://127.0.0.1:554/live' }, null, 2) },
          { name: 'Camera PTZ Control', method: 'POST', path: '/api/camera/ptz', body: JSON.stringify({ preset: 'center', action: 'goto' }, null, 2) },
          { name: 'Camera Speaker TTS', method: 'POST', path: '/api/camera/speak', body: JSON.stringify({ text: 'Xin chào, đây là camera' }, null, 2) },
          { name: 'Camera Recent Events', method: 'GET', path: '/api/camera/events' }
        ]
      },
      {
        group: 'System & Status',
        routes: [
          { name: 'Health Check', method: 'GET', path: '/health' },
          { name: 'NAS System Status', method: 'GET', path: '/api/nas/status' },
          { name: 'Auth Status Check', method: 'GET', path: '/api/auth/status' },
          { name: 'Web Auth Login', method: 'POST', path: '/api/auth/login', body: JSON.stringify({ token: 'YOUR_BEARER_TOKEN' }, null, 2) },
          { name: 'Web Auth Logout', method: 'POST', path: '/api/auth/logout' },
          { name: 'Bridge Accounts', method: 'GET', path: '/api/bridge/accounts' },
          { name: 'Send System Notification', method: 'POST', path: '/api/notify', body: JSON.stringify({ title: 'Cảnh báo', message: 'Phát hiện chuyển động' }, null, 2) }
        ]
      }
    ];

    let currentApiRoute = null;
    let activePathParams = {};
    let activeQueryParams = {};
    let lastResponseRaw = '';

    function renderApiRoutesSidebar() {
      const container = document.getElementById('apiRoutesList');
      if (!container) return;
      container.innerHTML = '';

      API_PRESETS.forEach(group => {
        const groupEl = document.createElement('div');
        groupEl.className = 'api-route-group';

        const title = document.createElement('div');
        title.className = 'api-group-title';
        title.textContent = group.group;
        groupEl.appendChild(title);

        group.routes.forEach(route => {
          const btn = document.createElement('button');
          btn.type = 'button';
          btn.className = 'api-route-item';
          btn.onclick = () => selectApiPreset(route, btn);

          const methodTag = document.createElement('span');
          methodTag.className = 'method-tag ' + route.method.toLowerCase();
          methodTag.textContent = route.method;

          const pathSpan = document.createElement('span');
          pathSpan.className = 'route-path';
          pathSpan.textContent = route.name;

          btn.append(methodTag, pathSpan);
          groupEl.appendChild(btn);
        });

        container.appendChild(groupEl);
      });

      if (API_PRESETS[0] && API_PRESETS[0].routes[0]) {
        const firstBtn = container.querySelector('.api-route-item');
        selectApiPreset(API_PRESETS[0].routes[0], firstBtn);
      }
    }

    function selectApiPreset(route, btn) {
      document.querySelectorAll('.api-route-item').forEach(el => el.classList.remove('active'));
      if (btn) btn.classList.add('active');

      currentApiRoute = route;
      activePathParams = { ...(route.pathParams || {}) };
      activeQueryParams = { ...(route.queryParams || {}) };

      document.getElementById('apiMethodSelect').value = route.method;
      document.getElementById('apiBodyInput').value = route.body || '';

      renderApiParamsUI();
      recomputeApiUrl();
    }

    function renderApiParamsUI() {
      const container = document.getElementById('apiParamsContainer');
      if (!container) return;
      container.innerHTML = '';

      const hasPath = Object.keys(activePathParams).length > 0;
      const hasQuery = Object.keys(activeQueryParams).length > 0;

      if (!hasPath && !hasQuery) {
        container.innerHTML = '<div class="api-params-empty">Không có tham số Path/Query mặc định cho route này</div>';
        return;
      }

      if (hasPath) {
        const section = document.createElement('div');
        section.className = 'api-param-section';

        const title = document.createElement('div');
        title.className = 'api-param-section-title';
        title.textContent = 'Path Parameters (Tham số đường dẫn)';
        section.appendChild(title);

        Object.keys(activePathParams).forEach(key => {
          const row = document.createElement('div');
          row.className = 'api-param-row';

          const keyLabel = document.createElement('div');
          keyLabel.className = 'api-param-key';
          keyLabel.textContent = `{${key}}`;

          const valInput = document.createElement('input');
          valInput.type = 'text';
          valInput.className = 'api-param-value';
          valInput.value = activePathParams[key];
          valInput.ariaLabel = `Path param ${key}`;
          valInput.oninput = (e) => {
            activePathParams[key] = e.target.value.trim();
            recomputeApiUrl();
          };

          row.append(keyLabel, valInput);
          section.appendChild(row);
        });

        container.appendChild(section);
      }

      if (hasQuery) {
        const section = document.createElement('div');
        section.className = 'api-param-section';

        const title = document.createElement('div');
        title.className = 'api-param-section-title';
        title.textContent = 'Query Parameters (Tham số truy vấn ?key=value)';
        section.appendChild(title);

        Object.keys(activeQueryParams).forEach(key => {
          const row = document.createElement('div');
          row.className = 'api-param-row';

          const keyLabel = document.createElement('div');
          keyLabel.className = 'api-param-key';
          keyLabel.textContent = key;

          const valInput = document.createElement('input');
          valInput.type = 'text';
          valInput.className = 'api-param-value';
          valInput.value = activeQueryParams[key];
          valInput.ariaLabel = `Query param ${key}`;
          valInput.oninput = (e) => {
            activeQueryParams[key] = e.target.value.trim();
            recomputeApiUrl();
          };

          row.append(keyLabel, valInput);
          section.appendChild(row);
        });

        container.appendChild(section);
      }
    }

    function recomputeApiUrl() {
      if (!currentApiRoute) return;

      let computedPath = currentApiRoute.path;
      Object.keys(activePathParams).forEach(key => {
        computedPath = computedPath.replace(`{${key}}`, activePathParams[key]);
      });

      const queryParts = [];
      Object.keys(activeQueryParams).forEach(key => {
        if (activeQueryParams[key] !== '') {
          queryParts.push(`${encodeURIComponent(key)}=${encodeURIComponent(activeQueryParams[key])}`);
        }
      });

      if (queryParts.length > 0) {
        computedPath += '?' + queryParts.join('&');
      }

      document.getElementById('apiUrlInput').value = computedPath;
      updateApiCurlPreview();
    }

    function switchApiReqTab(tabName) {
      document.getElementById('apiTabBtnParams').classList.toggle('active', tabName === 'params');
      document.getElementById('apiTabBtnBody').classList.toggle('active', tabName === 'body');
      document.getElementById('apiTabBtnHeaders').classList.toggle('active', tabName === 'headers');
      document.getElementById('apiTabBtnCurl').classList.toggle('active', tabName === 'curl');

      document.getElementById('apiTabParams').hidden = tabName !== 'params';
      document.getElementById('apiTabReqBody').hidden = tabName !== 'body';
      document.getElementById('apiTabReqHeaders').hidden = tabName !== 'headers';
      document.getElementById('apiTabCurl').hidden = tabName !== 'curl';

      if (tabName === 'curl') updateApiCurlPreview();
    }

    function updateApiCurlPreview() {
      const method = document.getElementById('apiMethodSelect').value;
      const urlPath = document.getElementById('apiUrlInput').value.trim() || '/';
      const bodyText = document.getElementById('apiBodyInput').value.trim();
      const headersText = document.getElementById('apiHeadersInput').value.trim();

      const fullUrl = window.location.origin + urlPath;
      let curl = `curl -X ${method} "${fullUrl}"`;

      if (headersText) {
        headersText.split('\n').forEach(line => {
          if (line.trim()) curl += ` \\\n  -H "${line.trim()}"`;
        });
      }

      if (['POST', 'PUT', 'PATCH'].includes(method) && bodyText) {
        const singleLineBody = bodyText.replace(/"/g, '\\"').replace(/\n/g, '');
        curl += ` \\\n  -d "${singleLineBody}"`;
      }

      const preview = document.getElementById('apiCurlPreview');
      if (preview) preview.textContent = curl;
    }

    function copyApiCurl() {
      const preview = document.getElementById('apiCurlPreview');
      if (!preview || !preview.textContent) return;
      navigator.clipboard.writeText(preview.textContent).then(() => {
        const btn = document.querySelector('.api-copy-btn');
        if (btn) {
          const orig = btn.innerHTML;
          btn.innerHTML = '<span class="mi">check</span> Đã chép!';
          setTimeout(() => { btn.innerHTML = orig; }, 1500);
        }
      });
    }

    function copyJsonResponse() {
      if (!lastResponseRaw) return;
      navigator.clipboard.writeText(lastResponseRaw).then(() => {
        const btn = document.getElementById('btnCopyJson');
        if (btn) {
          const orig = btn.innerHTML;
          btn.innerHTML = '<span class="mi" style="font-size:15px;">check</span> Đã chép!';
          setTimeout(() => { btn.innerHTML = orig; }, 1500);
        }
      });
    }

    function createJSONNode(value, key = null, isLast = true) {
      const node = document.createElement('div');
      node.className = 'json-tree-node';

      if (key !== null) {
        const keySpan = document.createElement('span');
        keySpan.className = 'json-tree-key';
        keySpan.textContent = `"${key}": `;
        node.appendChild(keySpan);
      }

      if (value === null) {
        const valSpan = document.createElement('span');
        valSpan.className = 'json-tree-null';
        valSpan.textContent = 'null' + (isLast ? '' : ',');
        node.appendChild(valSpan);
        return node;
      }

      const type = typeof value;

      if (type === 'boolean') {
        const valSpan = document.createElement('span');
        valSpan.className = 'json-tree-boolean';
        valSpan.textContent = value + (isLast ? '' : ',');
        node.appendChild(valSpan);
        return node;
      }

      if (type === 'number') {
        const valSpan = document.createElement('span');
        valSpan.className = 'json-tree-number';
        valSpan.textContent = value + (isLast ? '' : ',');
        node.appendChild(valSpan);
        return node;
      }

      if (type === 'string') {
        const valSpan = document.createElement('span');
        valSpan.className = 'json-tree-string';
        valSpan.textContent = `"${value}"` + (isLast ? '' : ',');
        node.appendChild(valSpan);
        return node;
      }

      if (type === 'object') {
        const isArray = Array.isArray(value);
        const keys = isArray ? value : Object.keys(value);
        const count = isArray ? value.length : keys.length;

        if (count === 0) {
          const emptySpan = document.createElement('span');
          emptySpan.className = 'json-tree-bracket';
          emptySpan.textContent = (isArray ? '[]' : '{}') + (isLast ? '' : ',');
          node.appendChild(emptySpan);
          return node;
        }

        const toggle = document.createElement('span');
        toggle.className = 'json-tree-toggle expanded';
        toggle.textContent = '▼';

        const openBracket = document.createElement('span');
        openBracket.className = 'json-tree-bracket';
        openBracket.textContent = isArray ? '[' : '{';

        const countBadge = document.createElement('span');
        countBadge.className = 'json-tree-count';
        countBadge.textContent = isArray ? `${count} items` : `${count} keys`;

        const childrenContainer = document.createElement('div');
        childrenContainer.className = 'json-tree-children';

        if (isArray) {
          value.forEach((item, index) => {
            childrenContainer.appendChild(createJSONNode(item, null, index === value.length - 1));
          });
        } else {
          const entries = Object.entries(value);
          entries.forEach(([k, v], index) => {
            childrenContainer.appendChild(createJSONNode(v, k, index === entries.length - 1));
          });
        }

        const closeBracket = document.createElement('span');
        closeBracket.className = 'json-tree-bracket';
        closeBracket.textContent = (isArray ? ']' : '}') + (isLast ? '' : ',');

        toggle.onclick = (e) => {
          e.stopPropagation();
          const isExpanded = toggle.classList.contains('expanded');
          toggle.classList.toggle('expanded', !isExpanded);
          toggle.textContent = !isExpanded ? '▼' : '▶';
          childrenContainer.style.display = !isExpanded ? 'block' : 'none';
          countBadge.style.display = !isExpanded ? 'none' : 'inline-block';
        };

        node.append(toggle, openBracket, countBadge, childrenContainer, closeBracket);
        return node;
      }

      return node;
    }

    function expandAllJsonTree() {
      const container = document.getElementById('apiResponseBody');
      if (!container) return;
      container.querySelectorAll('.json-tree-toggle').forEach(toggle => {
        toggle.classList.add('expanded');
        toggle.textContent = '▼';
      });
      container.querySelectorAll('.json-tree-children').forEach(child => {
        child.style.display = 'block';
      });
      container.querySelectorAll('.json-tree-count').forEach(count => {
        count.style.display = 'none';
      });
    }

    function collapseAllJsonTree() {
      const container = document.getElementById('apiResponseBody');
      if (!container) return;
      container.querySelectorAll('.json-tree-toggle').forEach(toggle => {
        toggle.classList.remove('expanded');
        toggle.textContent = '▶';
      });
      container.querySelectorAll('.json-tree-children').forEach(child => {
        child.style.display = 'none';
      });
      container.querySelectorAll('.json-tree-count').forEach(count => {
        count.style.display = 'inline-block';
      });
    }

    function renderResponseJSON(text) {
      const container = document.getElementById('apiResponseBody');
      if (!container) return;
      container.innerHTML = '';
      lastResponseRaw = text;

      if (!text || !text.trim()) {
        container.textContent = '(Phản hồi rỗng)';
        return;
      }

      const trimmed = text.trim();

      // Case 1: Standard single JSON
      try {
        const parsed = JSON.parse(trimmed);
        lastResponseRaw = JSON.stringify(parsed, null, 2);
        container.appendChild(createJSONNode(parsed));
        return;
      } catch (_) {}

      // Case 2: NDJSON stream lines
      const lines = trimmed.split('\n').filter(line => line.trim());
      if (lines.length > 1) {
        let allValid = true;
        const parsedList = [];
        for (const line of lines) {
          try {
            parsedList.push(JSON.parse(line.trim()));
          } catch (_) {
            allValid = false;
            break;
          }
        }
        if (allValid) {
          lastResponseRaw = parsedList.map(item => JSON.stringify(item, null, 2)).join('\n\n');
          parsedList.forEach((item, index) => {
            if (index > 0) {
              const divider = document.createElement('div');
              divider.style.cssText = 'border-top:1px dashed #3b3f58; margin:8px 0;';
              container.appendChild(divider);
            }
            container.appendChild(createJSONNode(item));
          });
          return;
        }
      }

      // Case 3: Plain text / HTML fallback
      container.textContent = text;
    }

    async function sendApiTestRequest() {
      const method = document.getElementById('apiMethodSelect').value;
      const urlPath = document.getElementById('apiUrlInput').value.trim();
      const bodyText = document.getElementById('apiBodyInput').value.trim();
      const headersText = document.getElementById('apiHeadersInput').value.trim();

      const btnSend = document.getElementById('btnApiSend');
      const statusBadge = document.getElementById('apiStatusBadge');
      const timeBadge = document.getElementById('apiTimeBadge');

      if (!urlPath) return;

      btnSend.disabled = true;
      statusBadge.className = 'api-status-badge api-status-idle';
      statusBadge.textContent = 'Đang gửi...';

      const container = document.getElementById('apiResponseBody');
      if (container) container.textContent = 'Đang chờ phản hồi từ server...';

      const headers = {};
      if (headersText) {
        headersText.split('\n').forEach(line => {
          const parts = line.split(':');
          if (parts.length >= 2) {
            headers[parts[0].trim()] = parts.slice(1).join(':').trim();
          }
        });
      }

      const options = { method, headers };
      if (['POST', 'PUT', 'PATCH'].includes(method) && bodyText) {
        options.body = bodyText;
      }

      const startTime = performance.now();
      try {
        const res = await fetch(urlPath, options);
        const duration = Math.round(performance.now() - startTime);
        timeBadge.textContent = duration + ' ms';

        statusBadge.textContent = `${res.status} ${res.statusText || ''}`;
        if (res.ok) {
          statusBadge.className = 'api-status-badge api-status-2xx';
        } else if (res.status >= 400 && res.status < 500) {
          statusBadge.className = 'api-status-badge api-status-4xx';
        } else {
          statusBadge.className = 'api-status-badge api-status-5xx';
        }

        const text = await res.text();
        renderResponseJSON(text);
      } catch (err) {
        const duration = Math.round(performance.now() - startTime);
        timeBadge.textContent = duration + ' ms';
        statusBadge.className = 'api-status-badge api-status-5xx';
        statusBadge.textContent = 'Lỗi kết nối';
        lastResponseRaw = 'Lỗi: ' + (err.message || 'Không thể gửi request');
        if (container) container.textContent = lastResponseRaw;
      } finally {
        btnSend.disabled = false;
      }
    }
