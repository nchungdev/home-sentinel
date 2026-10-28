import time


class InferenceSchedule:
    def __init__(self, intervalSeconds=2.0, clock=time.monotonic):
        self.intervalSeconds = max(0.1, float(intervalSeconds))
        self.clock = clock
        self.lastInferenceAt = None

    def shouldInfer(self, hasMotion):
        now = self.clock()
        due = self.lastInferenceAt is None or now - self.lastInferenceAt >= self.intervalSeconds
        if not hasMotion and not due:
            return False

        self.lastInferenceAt = now
        return True
