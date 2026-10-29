function switchCameraMobilePanel(panel) {
  const cameraView = document.getElementById('view-camera');
  if (!cameraView || !['controls', 'logs'].includes(panel)) return;

  cameraView.classList.toggle('camera-panel-controls', panel === 'controls');
  cameraView.classList.toggle('camera-panel-logs', panel === 'logs');

  document.querySelectorAll('.camera-mobile-tab').forEach(button => {
    const isSelected = button.dataset.cameraPanel === panel;
    button.classList.toggle('active', isSelected);
    button.setAttribute('aria-pressed', String(isSelected));
  });
}
