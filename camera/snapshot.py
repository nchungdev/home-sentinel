import os
import time
import cv2
import numpy

class SnapshotManager:
    def __init__(
        self,
        snapshotDirectory: str = "snapshots",
        enableFacePrivacyBlur: bool = False
    ):
        self.snapshotDirectory = snapshotDirectory
        self.enableFacePrivacyBlur = enableFacePrivacyBlur
        self.latestSnapshotBytes = None
        self.latestSnapshotTimestamp = 0.0
        os.makedirs(self.snapshotDirectory, exist_ok=True)

    def captureAndStore(
        self,
        frameImage: numpy.ndarray,
        zoneIdentifier: str
    ) -> str:
        processedFrame = frameImage.copy()

        if self.enableFacePrivacyBlur:
            processedFrame = self._applyPrivacyBlur(processedFrame)

        timestampInt = int(time.time())
        fileName = f"presence_{zoneIdentifier}_{timestampInt}.jpg"
        filePath = os.path.join(self.snapshotDirectory, fileName)
        latestFilePath = os.path.join(self.snapshotDirectory, "latest_presence.jpg")

        cv2.imwrite(filePath, processedFrame, [cv2.IMWRITE_JPEG_QUALITY, 85])
        cv2.imwrite(latestFilePath, processedFrame, [cv2.IMWRITE_JPEG_QUALITY, 85])

        hasEncoded, encodedBytes = cv2.imencode(".jpg", processedFrame)
        if hasEncoded:
            self.latestSnapshotBytes = encodedBytes.tobytes()
            self.latestSnapshotTimestamp = time.time()

        return filePath

    def getLatestSnapshotBytes(self) -> bytes:
        return self.latestSnapshotBytes

    def _applyPrivacyBlur(self, image: numpy.ndarray) -> numpy.ndarray:
        frameHeight, frameWidth = image.shape[:2]
        topRegionHeight = int(frameHeight * 0.35)
        topRegion = image[0:topRegionHeight, 0:frameWidth]
        blurredTop = cv2.GaussianBlur(topRegion, (51, 51), 30)
        image[0:topRegionHeight, 0:frameWidth] = blurredTop
        return image
