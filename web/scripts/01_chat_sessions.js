const CHAT_SESSIONS_STORAGE_KEY = 'sentinel_chat_sessions_v1';
const LEGACY_CHAT_STORAGE_KEY = 'sentinel_chat_history_v1';
let chatSessions = [];
let currentChatSessionID = '';

function createChatSessionID() {
  const identifier = typeof crypto !== 'undefined' && crypto.randomUUID
    ? crypto.randomUUID()
    : Math.random().toString(36).slice(2) + Date.now().toString(36);
  return `web_${identifier}`;
}

function activeChatSessionID() {
  return currentChatSessionID;
}

function showChatSessionStatus(message) {
  const status = document.getElementById('chatSessionStatus');
  if (status) status.textContent = message;
}

function createEmptyChatSession() {
  const timestamp = Date.now();
  return {
    id: createChatSessionID(),
    title: 'Cuộc trò chuyện mới',
    messages: [],
    createdAt: timestamp,
    updatedAt: timestamp
  };
}

function persistChatSessions() {
  try {
    localStorage.setItem(CHAT_SESSIONS_STORAGE_KEY, JSON.stringify({
      activeSessionId: currentChatSessionID,
      sessions: chatSessions
    }));
    showChatSessionStatus('');
    return true;
  } catch (error) {
    showChatSessionStatus('Không thể lưu session. Hãy kiểm tra dung lượng trình duyệt.');
    return false;
  }
}

function normalizeChatSession(session) {
  if (!session || typeof session.id !== 'string' || !Array.isArray(session.messages)) {
    return null;
  }

  const messages = session.messages
    .filter(message => message && ['user', 'agent'].includes(message.sender) &&
      typeof message.text === 'string')
    .slice(-50);
  const firstUserMessage = messages.find(message => message.sender === 'user');
  return {
    id: session.id,
    title: typeof session.title === 'string' && session.title
      ? session.title
      : firstUserMessage?.text.slice(0, 40) || 'Cuộc trò chuyện mới',
    messages,
    createdAt: Number(session.createdAt) || Date.now(),
    updatedAt: Number(session.updatedAt) || Date.now()
  };
}

function initializeChatSessions() {
  try {
    const stored = localStorage.getItem(CHAT_SESSIONS_STORAGE_KEY);
    if (stored) {
      const state = JSON.parse(stored);
      chatSessions = Array.isArray(state.sessions)
        ? state.sessions.map(normalizeChatSession).filter(Boolean)
        : [];
      currentChatSessionID = state.activeSessionId || '';
    } else {
      const legacy = JSON.parse(localStorage.getItem(LEGACY_CHAT_STORAGE_KEY) || '[]');
      const messages = Array.isArray(legacy) ? legacy : [];
      const firstUserMessage = messages.find(message => message &&
        message.sender === 'user' && typeof message.text === 'string');
      chatSessions = messages.length ? [normalizeChatSession({
        id: createChatSessionID(),
        title: firstUserMessage?.text.slice(0, 40) ||
          'Cuộc trò chuyện cũ',
        messages,
        createdAt: Date.now(),
        updatedAt: Date.now()
      })].filter(Boolean) : [];
    }
  } catch (error) {
    chatSessions = [];
    showChatSessionStatus('Không đọc được lịch sử chat đã lưu.');
  }

  if (!chatSessions.length) chatSessions = [createEmptyChatSession()];
  if (!chatSessions.some(session => session.id === currentChatSessionID)) {
    currentChatSessionID = chatSessions[0].id;
  }

  if (persistChatSessions()) {
    try {
      localStorage.removeItem(LEGACY_CHAT_STORAGE_KEY);
    } catch (error) {
      showChatSessionStatus('Không thể dọn dữ liệu chat cũ khỏi trình duyệt.');
    }
  }
  renderActiveChatSession();
}

function getChatHistory(sessionID = currentChatSessionID) {
  return chatSessions.find(session => session.id === sessionID)?.messages.slice() || [];
}

function saveChatHistory(messages, sessionID = currentChatSessionID) {
  const session = chatSessions.find(item => item.id === sessionID);
  if (!session) return;

  session.messages = messages.slice(-50);
  session.updatedAt = Date.now();
  const firstUserMessage = session.messages.find(message => message.sender === 'user');
  if (firstUserMessage && session.title === 'Cuộc trò chuyện mới') {
    session.title = firstUserMessage.text.trim().slice(0, 40) || 'Cuộc trò chuyện mới';
  }

  persistChatSessions();
  renderChatHeader();
  renderChatSessionList();
}

function renderActiveChatSession() {
  const chatList = document.getElementById('chatList');
  if (!chatList) return;

  chatList.replaceChildren();
  const messages = getChatHistory();
  messages.forEach(message => appendBubble(message.sender, message.text, false));

  chatList.scrollTop = chatList.scrollHeight;
  renderChatHeader();
  renderChatSessionList();
}

function renderChatHeader() {
  const session = chatSessions.find(item => item.id === currentChatSessionID);
  const title = document.getElementById('chatHeaderTitle');
  const subtitle = document.getElementById('chatHeaderSubtitle');
  if (!session || !title || !subtitle) return;

  const hasMessages = session.messages.length > 0;
  title.textContent = hasMessages ? session.title : 'Xin chào 👋';
  subtitle.textContent = hasMessages
    ? 'Cuộc trò chuyện'
    : 'Bạn muốn bắt đầu từ đâu?';
}

function renderChatSessionList() {
  const list = document.getElementById('chatSessionList');
  if (!list) return;

  list.replaceChildren();
  [...chatSessions]
    .filter(session => session.messages.length > 0 ||
      session.id === currentChatSessionID)
    .sort((first, second) => second.updatedAt - first.updatedAt)
    .forEach(session => {
      const item = document.createElement('div');
      item.className = 'chat-session-item';
      item.classList.toggle('active', session.id === currentChatSessionID);

      const select = document.createElement('button');
      select.type = 'button';
      select.className = 'chat-session-select';
      const icon = document.createElement('span');
      icon.className = 'mi chat-session-icon';
      icon.setAttribute('aria-hidden', 'true');
      icon.textContent = 'chat_bubble_outline';
      const title = document.createElement('span');
      title.className = 'chat-session-title';
      title.textContent = session.messages.length
        ? session.title
        : 'Cuộc trò chuyện mới';
      const time = document.createElement('span');
      time.className = 'chat-session-time';
      time.textContent = formatChatSessionTime(session);
      select.append(icon, title, time);
      select.title = session.title;
      select.setAttribute('aria-current',
        session.id === currentChatSessionID ? 'true' : 'false');
      select.addEventListener('click', () => {
        if (chatInFlight && session.id !== currentChatSessionID) {
          selectChatSession(session.id);
          return;
        }
        selectChatSession(session.id);
        closeChatHistory();
      });

      const remove = document.createElement('button');
      remove.type = 'button';
      remove.className = 'chat-session-delete';
      remove.title = `Xóa: ${session.title}`;
      remove.setAttribute('aria-label', `Xóa cuộc trò chuyện: ${session.title}`);
      remove.innerHTML = '<span class="mi" aria-hidden="true">delete</span>';
      remove.addEventListener('click', () => deleteChatSession(session.id));

      item.append(select, remove);
      list.appendChild(item);
    });
}

function formatChatSessionTime(session) {
  if (session.id === currentChatSessionID) return 'Đang mở';

  const minutes = Math.max(0, Math.floor((Date.now() - session.updatedAt) / 60000));
  if (minutes < 1) return 'Vừa xong';
  if (minutes < 60) return `${minutes} phút`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} giờ`;
  return `${Math.floor(hours / 24)} ngày`;
}

function toggleChatHistory(event) {
  event.stopPropagation();
  const popover = document.getElementById('chatHistoryPopover');
  const toggle = document.getElementById('chatHistoryToggle');
  if (!popover || !toggle) return;

  popover.hidden = !popover.hidden;
  toggle.setAttribute('aria-expanded', String(!popover.hidden));
}

function closeChatHistory() {
  const popover = document.getElementById('chatHistoryPopover');
  const toggle = document.getElementById('chatHistoryToggle');
  if (!popover || !toggle) return;

  popover.hidden = true;
  toggle.setAttribute('aria-expanded', 'false');
}

function createChatSession() {
  if (chatInFlight) {
    showChatSessionStatus('Đợi phản hồi hiện tại hoàn tất trước khi chuyển session.');
    return;
  }

  const session = createEmptyChatSession();
  chatSessions.push(session);
  currentChatSessionID = session.id;
  persistChatSessions();
  renderActiveChatSession();
  closeChatHistory();
  document.getElementById('promptInput')?.focus();
}

function selectChatSession(sessionID) {
  if (sessionID === currentChatSessionID) return;
  if (chatInFlight) {
    showChatSessionStatus('Đợi phản hồi hiện tại hoàn tất trước khi chuyển session.');
    return;
  }
  if (!chatSessions.some(session => session.id === sessionID)) return;

  currentChatSessionID = sessionID;
  persistChatSessions();
  renderActiveChatSession();
}

function deleteChatSession(sessionID) {
  const session = chatSessions.find(item => item.id === sessionID);
  if (!session || chatInFlight) return;
  if (!confirm(`Xóa cuộc trò chuyện "${session.title}"? Thao tác này không thể hoàn tác.`)) {
    return;
  }

  chatSessions = chatSessions.filter(item => item.id !== sessionID);
  if (!chatSessions.length) chatSessions.push(createEmptyChatSession());
  if (currentChatSessionID === sessionID) {
    chatSessions.sort((first, second) => second.updatedAt - first.updatedAt);
    currentChatSessionID = chatSessions[0].id;
  }
  persistChatSessions();
  renderActiveChatSession();
}

document.addEventListener('click', event => {
  if (!(event.target instanceof Element) ||
      !event.target.closest('.chat-header-actions')) {
    closeChatHistory();
  }
});

document.addEventListener('keydown', event => {
  if (event.key === 'Escape') closeChatHistory();
});
