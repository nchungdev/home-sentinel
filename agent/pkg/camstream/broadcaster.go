package camstream

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

type Broadcaster struct {
	mu           sync.RWMutex
	clients      map[chan []byte]struct{}
	latestFrame  []byte
	rtspURL      string
	cmd          *exec.Cmd
	running      bool
	stopCh       chan struct{}
	lastAccess   time.Time
	idleTimer    *time.Timer
}

var (
	globalBroadcaster     *Broadcaster
	globalBroadcasterOnce sync.Once
)

func GetBroadcaster() *Broadcaster {
	globalBroadcasterOnce.Do(func() {
		globalBroadcaster = &Broadcaster{
			clients: make(map[chan []byte]struct{}),
		}
	})
	return globalBroadcaster
}

func (b *Broadcaster) UpdateRTSP(rtspURL string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.rtspURL != rtspURL {
		b.rtspURL = rtspURL
		if b.running {
			b.stopLocked()
			b.startLocked()
		}
	}
}

func (b *Broadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request, rtspURL string) {
	b.UpdateRTSP(rtspURL)

	b.mu.Lock()
	b.lastAccess = time.Now()
	if !b.running {
		b.startLocked()
	}

	frameCh := make(chan []byte, 30)
	b.clients[frameCh] = struct{}{}
	initFrame := b.latestFrame
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.clients, frameCh)
		close(frameCh)
		clientCount := len(b.clients)
		if clientCount == 0 {
			// Nếu không còn ai xem sau 45s thì tắt ffmpeg để tiết kiệm CPU NAS
			if b.idleTimer != nil {
				b.idleTimer.Stop()
			}
			b.idleTimer = time.AfterFunc(45*time.Second, func() {
				b.mu.Lock()
				defer b.mu.Unlock()
				if len(b.clients) == 0 && b.running {
					b.stopLocked()
				}
			})
		}
		b.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=--ffmpeg")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusOK)

	flusher, hasFlusher := w.(http.Flusher)

	// Bắn ngay frame gần nhất trong cache nếu có để F5 hiển thị ngay lập tức (0 delay)
	if len(initFrame) > 0 {
		_, _ = w.Write([]byte("--ffmpeg\r\nContent-Type: image/jpeg\r\n\r\n"))
		_, _ = w.Write(initFrame)
		_, _ = w.Write([]byte("\r\n"))
		if hasFlusher {
			flusher.Flush()
		}
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case frame, ok := <-frameCh:
			if !ok {
				return
			}
			_, err := w.Write([]byte("--ffmpeg\r\nContent-Type: image/jpeg\r\n\r\n"))
			if err != nil {
				return
			}
			if _, err := w.Write(frame); err != nil {
				return
			}
			if _, err := w.Write([]byte("\r\n")); err != nil {
				return
			}
			if hasFlusher {
				flusher.Flush()
			}
		}
	}
}

func (b *Broadcaster) startLocked() {
	if b.running || b.rtspURL == "" {
		return
	}

	if b.idleTimer != nil {
		b.idleTimer.Stop()
		b.idleTimer = nil
	}

	b.stopCh = make(chan struct{})
	b.running = true

	// Tối ưu hóa cực đại tham số khởi động RTSP:
	// -rtsp_transport tcp: chống giật frame packet drop
	// -analyzeduration 100000 -probesize 100000: đọc header cực nhanh
	// -fflags nobuffer+flush_packets: xuất stream ngay tức khắc
	// -flags low_delay: loại bỏ buffer trễ
	cmd := exec.Command("ffmpeg",
		"-rtsp_transport", "tcp",
		"-analyzeduration", "100000",
		"-probesize", "100000",
		"-fflags", "nobuffer+flush_packets",
		"-flags", "low_delay",
		"-i", b.rtspURL,
		"-an",
		"-f", "image2pipe",
		"-c:v", "mjpeg",
		"-q:v", "5",
		"-r", "14",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		b.running = false
		return
	}

	if err := cmd.Start(); err != nil {
		b.running = false
		return
	}

	b.cmd = cmd

	go b.readLoop(stdout, b.stopCh)
}

func (b *Broadcaster) stopLocked() {
	if !b.running {
		return
	}
	b.running = false
	if b.stopCh != nil {
		close(b.stopCh)
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
		_ = b.cmd.Wait()
		b.cmd = nil
	}
}

// Đọc luồng JPEG phân đoạn từ image2pipe (nhận diện bằng mốc JPEG SOI 0xFFD8 và EOI 0xFFD9)
func (b *Broadcaster) readLoop(reader io.Reader, stopCh chan struct{}) {
	defer func() {
		b.mu.Lock()
		if b.running {
			b.stopLocked()
		}
		b.mu.Unlock()
	}()

	buffer := make([]byte, 16384)
	var currentFrame bytes.Buffer

	soi := []byte{0xFF, 0xD8}
	eoi := []byte{0xFF, 0xD9}

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		n, err := reader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("[camstream] read error: %v", err)
			}
			return
		}

		chunk := buffer[:n]
		currentFrame.Write(chunk)

		frameData := currentFrame.Bytes()
		for {
			soiIdx := bytes.Index(frameData, soi)
			if soiIdx == -1 {
				// Không có đầu frame, bỏ bớt phần dư thừa
				if len(frameData) > 1024 {
					currentFrame.Reset()
					currentFrame.Write(frameData[len(frameData)-2:])
				}
				break
			}

			eoiIdx := bytes.Index(frameData[soiIdx+2:], eoi)
			if eoiIdx == -1 {
				// Chưa nhận đủ đuôi frame
				if soiIdx > 0 {
					trimmed := frameData[soiIdx:]
					currentFrame.Reset()
					currentFrame.Write(trimmed)
				}
				break
			}

			endPos := soiIdx + 2 + eoiIdx + 2
			jpegData := make([]byte, endPos-soiIdx)
			copy(jpegData, frameData[soiIdx:endPos])

			// Cập nhật frame mới nhất cho instant preview
			b.mu.Lock()
			b.latestFrame = jpegData
			// Phân phối frame tới toàn bộ client web / TV
			for ch := range b.clients {
				select {
				case ch <- jpegData:
				default:
					// Tránh nghẽn nếu client đọc chậm
				}
			}
			b.mu.Unlock()

			// Cắt sang frame kế tiếp trong buffer
			remaining := frameData[endPos:]
			currentFrame.Reset()
			currentFrame.Write(remaining)
			frameData = currentFrame.Bytes()
		}
	}
}
