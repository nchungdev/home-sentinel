POSTURE_STANDING = "standing"
POSTURE_SITTING = "sitting"
POSTURE_LYING = "lying"
POSTURE_UNKNOWN = "unknown"

class PostureClassifier:
    def __init__(
        self,
        standingRatioThreshold: float = 0.50,
        lyingRatioThreshold: float = 1.15
    ):
        self.standingRatioThreshold = standingRatioThreshold
        self.lyingRatioThreshold = lyingRatioThreshold

    def classifyByBoundingBox(
        self,
        boxWidth: float,
        boxHeight: float
    ) -> str:
        if boxHeight <= 0.0 or boxWidth <= 0.0:
            return POSTURE_UNKNOWN

        aspectRatio = boxWidth / boxHeight
        if aspectRatio < self.standingRatioThreshold:
            return POSTURE_STANDING
        if aspectRatio > self.lyingRatioThreshold:
            return POSTURE_LYING
        return POSTURE_SITTING
