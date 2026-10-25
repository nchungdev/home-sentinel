import cv2
import threading
import time

class VideoStreamReader:
    def __init__(self, streamUrl: str, reconnectDelaySeconds: float = 3.0):
        self.streamUrl = streamUrl
        self.reconnectDelaySeconds = reconnectDelaySeconds
        self.latestFrame = None
        self.isRunning = False
        self.lock = threading.Lock()
        self.workerThread = None

    def start(self):
        self.isRunning = True
        self.workerThread = threading.Thread(target=self._captureLoop, daemon=True)
        self.workerThread.start()

    def stop(self):
        self.isRunning = False
        if self.workerThread is not None:
            self.workerThread.join(timeout=2.0)

    def readLatestFrame(self):
        with self.lock:
            if self.latestFrame is None:
                return None
            return self.latestFrame.copy()

    def _captureLoop(self):
        while self.isRunning:
            capture = cv2.VideoCapture(self.streamUrl, cv2.CAP_FFMPEG)
            capture.set(cv2.CAP_PROP_BUFFERSIZE, 1)

            if not capture.isOpened():
                time.sleep(self.reconnectDelaySeconds)
                continue

            while self.isRunning:
                hasFrame, frame = capture.read()
                if not hasFrame:
                    break

                with self.lock:
                    self.latestFrame = frame

            capture.release()
            time.sleep(self.reconnectDelaySeconds)
