import time
import cv2
import numpy

class MotionPreFilter:
    def __init__(
        self,
        motionPixelThresholdRatio: float = 0.005,
        motionCooldownSeconds: float = 3.0,
        downscaleWidth: int = 320,
        downscaleHeight: int = 180
    ):
        self.motionPixelThresholdRatio = motionPixelThresholdRatio
        self.motionCooldownSeconds = motionCooldownSeconds
        self.downscaleWidth = downscaleWidth
        self.downscaleHeight = downscaleHeight
        self.backgroundSubtractor = cv2.createBackgroundSubtractorMOG2(
            history=200,
            varThreshold=25,
            detectShadows=False
        )
        self.lastMotionTimestamp = 0.0

    def evaluateMotion(self, frameImage: numpy.ndarray) -> bool:
        currentTime = time.time()
        resizedFrame = cv2.resize(
            frameImage,
            (self.downscaleWidth, self.downscaleHeight),
            interpolation=cv2.INTER_AREA
        )
        foregroundMask = self.backgroundSubtractor.apply(resizedFrame)
        nonzeroPixels = cv2.countNonZero(foregroundMask)
        totalPixels = self.downscaleWidth * self.downscaleHeight

        motionRatio = nonzeroPixels / float(totalPixels)
        if motionRatio >= self.motionPixelThresholdRatio:
            self.lastMotionTimestamp = currentTime
            return True

        timeSinceLastMotion = currentTime - self.lastMotionTimestamp
        return timeSinceLastMotion <= self.motionCooldownSeconds
