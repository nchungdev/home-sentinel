const chatHistoryWideLayout = window.matchMedia('(min-width: 1200px)');

function updateChatHistoryLayout() {
  const panel = document.getElementById('chatHistoryPopover');
  const sidebar = document.getElementById('chatHistorySidebar');
  const toggle = document.getElementById('chatHistoryToggle');
  const newSession = document.getElementById('chatNewSessionButton');
  const heading = document.getElementById('chatHistoryHeading');
  if (!panel || !sidebar || !toggle || !newSession || !heading) return;

  const wide = chatHistoryWideLayout.matches;
  const destination = wide ? sidebar : toggle.parentElement;
  destination.appendChild(panel);
  if (wide) heading.appendChild(newSession);
  else toggle.parentElement.insertBefore(newSession, toggle);
  panel.setAttribute('role', wide ? 'navigation' : 'dialog');
  panel.hidden = !wide;
  toggle.setAttribute('aria-expanded', 'false');
}

chatHistoryWideLayout.addEventListener('change', updateChatHistoryLayout);
updateChatHistoryLayout();
