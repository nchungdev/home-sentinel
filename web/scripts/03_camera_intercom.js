    function initSpeechRecognition() {
      const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
      if (!SpeechRecognition) {
        return null;
      }
      const recognition = new SpeechRecognition();
      recognition.continuous = false;
      recognition.interimResults = true;
      recognition.lang = 'vi-VN';
      return recognition;
    }

    function toggleSpeechRecognition() {
      const btn = document.getElementById('btnCamMic');
      const input = document.getElementById('camVoiceInput');
      const indicator = document.getElementById('callStatusIndicator');
      const statusBadge = document.getElementById('callStatusBadge');
      const statusDot = document.getElementById('callStatusDot');
      const idleView = document.getElementById('intercomIdle');
      const waveView = document.getElementById('intercomActiveWave');
      const transcriptEl = document.getElementById('liveTranscriptText');
      const timerText = document.getElementById('liveCallTimerText');

      if (!speechRecognition) {
        speechRecognition = initSpeechRecognition();
        if (!speechRecognition) {
          alert('Trình duyệt của bạn chưa hỗ trợ nhận diện giọng nói Web Speech API (Hãy dùng Chrome hoặc Edge).');
          return;
        }

        speechRecognition.onresult = (event) => {
          let transcript = '';
          for (let i = event.resultIndex; i < event.results.length; ++i) {
            transcript += event.results[i][0].transcript;
          }
          if (input) input.value = transcript;
          if (transcriptEl) transcriptEl.textContent = `"${transcript}"`;
        };

        speechRecognition.onend = () => {
          isRecordingVoice = false;
          if (btn) {
            btn.classList.remove('recording');
            btn.innerHTML = SVG_ICONS.mic;
          }
          if (!isCalling) {
            if (idleView) idleView.style.display = 'flex';
            if (waveView) waveView.classList.remove('visible');
            if (statusBadge) statusBadge.classList.remove('active-talk');
            if (statusDot) statusDot.style.background = '#6b7280';
            if (indicator) indicator.textContent = 'Đã nghe xong';
          }
          // Tự động phát khi nói xong nếu có nội dung
          if (input && input.value.trim() && !isCalling) {
            sendCameraSpeech();
          }
        };

        speechRecognition.onerror = (event) => {
          console.error('Speech error:', event.error);
          isRecordingVoice = false;
          if (btn) {
            btn.classList.remove('recording');
            btn.innerHTML = SVG_ICONS.mic;
          }
          if (idleView) idleView.style.display = 'flex';
          if (waveView) waveView.classList.remove('visible');
          if (statusBadge) statusBadge.classList.remove('active-talk');
          if (statusDot) statusDot.style.background = '#ef4444';
          if (indicator) indicator.textContent = 'Lỗi mic: ' + event.error;
        };
      }

      if (isRecordingVoice) {
        speechRecognition.stop();
        isRecordingVoice = false;
        if (btn) {
          btn.classList.remove('recording');
          btn.innerHTML = SVG_ICONS.mic;
        }
        if (idleView) idleView.style.display = 'flex';
        if (waveView) waveView.classList.remove('visible');
        if (statusBadge) statusBadge.classList.remove('active-talk');
        if (statusDot) statusDot.style.background = '#6b7280';
        if (indicator) indicator.textContent = 'Sẵn sàng';
      } else {
        try {
          speechRecognition.start();
          isRecordingVoice = true;
          if (btn) {
            btn.classList.add('recording');
            btn.innerHTML = SVG_ICONS.stop;
          }
          if (idleView) idleView.style.display = 'none';
          if (waveView) waveView.classList.add('visible');
          if (transcriptEl) transcriptEl.textContent = 'Đang lắng nghe câu nói của bạn...';
          if (timerText) timerText.textContent = 'Ghi âm phát loa';
          if (statusBadge) statusBadge.classList.add('active-talk');
          if (statusDot) statusDot.style.background = '#ef4444';
          if (indicator) indicator.textContent = 'Đang nghe...';
        } catch (e) {
          console.error(e);
        }
      }
    }

    // TÍNH NĂNG 2: CALL / GỌI ĐÀM THOẠI TRỰC TIẾP TỚI CAMERA
    let isCalling = false;
    let callTimerInterval = null;
    let callSeconds = 0;
    let liveCallRecognition = null;

    function toggleCameraCall() {
      const btn = document.getElementById('btnCamCall');
      const indicator = document.getElementById('callStatusIndicator');
      const statusBadge = document.getElementById('callStatusBadge');
      const statusDot = document.getElementById('callStatusDot');
      const idleView = document.getElementById('intercomIdle');
      const waveView = document.getElementById('intercomActiveWave');
      const transcriptEl = document.getElementById('liveTranscriptText');
      const timerText = document.getElementById('liveCallTimerText');
      const log = document.getElementById('termLog');

      isCalling = !isCalling;

      if (isCalling) {
        // Bắt đầu cuộc gọi
        if (btn) {
          btn.classList.add('in-call');
          btn.innerHTML = SVG_ICONS.phoneEnd;
          btn.title = 'Ngắt cuộc gọi đàm thoại';
        }
        if (statusBadge) statusBadge.classList.add('active-talk');
        if (statusDot) statusDot.style.background = '#10b981';
        if (indicator) indicator.textContent = 'Đang gọi 2 chiều';
        
        if (idleView) idleView.style.display = 'none';
        if (waveView) waveView.classList.add('visible');
        if (transcriptEl) transcriptEl.textContent = 'Đang truyền âm thanh trực tiếp tới Loa Camera...';

        callSeconds = 0;
        if (timerText) timerText.textContent = 'Đang đàm thoại: 00:00';
        callTimerInterval = setInterval(() => {
          callSeconds++;
          const m = String(Math.floor(callSeconds / 60)).padStart(2, '0');
          const s = String(callSeconds % 60).padStart(2, '0');
          if (timerText) timerText.textContent = `Đang đàm thoại: ${m}:${s}`;
        }, 1000);

        log.textContent += `\n[${new Date().toLocaleTimeString()}] Bắt đầu đàm thoại 2 chiều trực tiếp với Camera...`;
        log.scrollTop = log.scrollHeight;

        // Kích hoạt nhận diện liên tục gửi lên loa camera
        startContinuousCallAudio();
      } else {
        // Kết thúc cuộc gọi
        if (btn) {
          btn.classList.remove('in-call');
          btn.innerHTML = SVG_ICONS.phone;
          btn.title = 'Gọi trực tiếp qua loa camera';
        }
        if (statusBadge) statusBadge.classList.remove('active-talk');
        if (statusDot) statusDot.style.background = '#6b7280';
        if (indicator) indicator.textContent = 'Đã kết thúc cuộc gọi';
        
        if (idleView) idleView.style.display = 'flex';
        if (waveView) waveView.classList.remove('visible');

        clearInterval(callTimerInterval);
        stopContinuousCallAudio();

        log.textContent += `\n[${new Date().toLocaleTimeString()}] Đã ngắt cuộc gọi đàm thoại Camera.`;
        log.scrollTop = log.scrollHeight;
      }
    }

    function startContinuousCallAudio() {
      const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
      if (!SpeechRecognition) return;

      const transcriptEl = document.getElementById('liveTranscriptText');
      liveCallRecognition = new SpeechRecognition();
      liveCallRecognition.continuous = true;
      liveCallRecognition.interimResults = true;
      liveCallRecognition.lang = 'vi-VN';

      liveCallRecognition.onresult = (event) => {
        for (let i = event.resultIndex; i < event.results.length; ++i) {
          const text = event.results[i][0].transcript;
          if (transcriptEl) transcriptEl.textContent = `"${text}"`;
          if (event.results[i].isFinal) {
            const spokenText = text.trim();
            if (spokenText && isCalling) {
              sendCameraSpeech(spokenText);
            }
          }
        }
      };

      liveCallRecognition.onend = () => {
        // Tự khởi động lại nếu vẫn đang trong cuộc gọi
        if (isCalling && liveCallRecognition) {
          try { liveCallRecognition.start(); } catch (e) {}
        }
      };

      try {
        liveCallRecognition.start();
      } catch (e) {
        console.warn('Call recognition start error:', e);
      }
    }

    function stopContinuousCallAudio() {
      if (liveCallRecognition) {
        try { liveCallRecognition.stop(); } catch (e) {}
        liveCallRecognition = null;
      }
    }

    function fillVoiceText(t) {
      const input = document.getElementById('camVoiceInput');
      input.value = t;
      input.focus();
    }

    // SMART DETECTION & PRIVACY STATUSES (READ-ONLY)
    const detectionStates = {
      motion: false,
      tracking: false,
      person: false,
      baby_cry: false
    };

    let isPrivacyOn = false;

    async function loadDetectionStates() {
      try {
        const res = await fetch('/api/camera/detection');
        const data = await res.json();
        if (data.ok && data.states) {
          const badgeMap = {
            motion: 'statusMotion',
            tracking: 'statusTracking',
            person: 'statusPerson',
            baby_cry: 'statusBabyCry'
          };
          for (const [k, v] of Object.entries(data.states)) {
            if (k in detectionStates) {
              detectionStates[k] = v;
              const badge = document.getElementById(badgeMap[k]);
              if (badge) {
                if (v) {
                  badge.className = 'status-badge-val on';
                  badge.textContent = 'BẬT';
                } else {
                  badge.className = 'status-badge-val off';
                  badge.textContent = 'TẮT';
                }
              }
            } else if (k === 'privacy') {
              isPrivacyOn = v;
              const pBadge = document.getElementById('statusPrivacy');
              const pIcon = document.getElementById('privacyIcon');
              if (pBadge) {
                if (v) {
                  pBadge.className = 'status-badge-val privacy-on';
                  pBadge.textContent = 'BẬT';
                  if (pIcon) pIcon.textContent = 'lock';
                } else {
                  pBadge.className = 'status-badge-val off';
                  pBadge.textContent = 'TẮT';
                  if (pIcon) pIcon.textContent = 'visibility';
                }
              }
            }
          }
        }
      } catch (e) {
        console.warn('Load detection states error:', e);
      }
    }

    // ENGINES & FALLBACK
    let enginesList = [];
