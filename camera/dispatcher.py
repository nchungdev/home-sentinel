import json
import logging
import time
import urllib.error
import urllib.request
from typing import Any, Dict, Optional

class EventDispatcher:
    def __init__(self, webhookEndpointUrl: str, requestTimeoutSeconds: float = 3.0):
        self.webhookEndpointUrl = webhookEndpointUrl
        self.requestTimeoutSeconds = requestTimeoutSeconds

    def dispatchEvent(
        self,
        zoneIdentifier: str,
        eventType: str,
        metadata: Optional[Dict[str, Any]] = None
    ):
        payloadDictionary = {
            "zone_id": zoneIdentifier,
            "event_type": eventType,
            "timestamp": int(time.time())
        }
        if metadata:
            payloadDictionary.update(metadata)

        encodedPayload = json.dumps(payloadDictionary).encode("utf-8")
        requestObject = urllib.request.Request(
            self.webhookEndpointUrl,
            data=encodedPayload,
            headers={"Content-Type": "application/json"}
        )

        try:
            with urllib.request.urlopen(
                requestObject,
                timeout=self.requestTimeoutSeconds
            ) as response:
                if response.status >= 400:
                    logging.warning(
                        "Webhook responded with HTTP status %d", response.status
                    )
        except (urllib.error.URLError, TimeoutError, OSError) as error:
            logging.error(
                "Failed to dispatch event %s for zone %s: %s",
                eventType,
                zoneIdentifier,
                str(error)
            )
