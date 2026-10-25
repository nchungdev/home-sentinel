from __future__ import annotations

from dataclasses import asdict, dataclass, field
from enum import Enum
from typing import Any


class ActionStatus(str, Enum):
    ACCEPTED = "accepted"
    RUNNING = "running"
    NEEDS_INPUT = "needs_input"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CANCELLED = "cancelled"
    UNKNOWN = "unknown"

    accepted = "accepted"
    running = "running"
    needs_input = "needs_input"
    succeeded = "succeeded"
    failed = "failed"
    cancelled = "cancelled"
    unknown = "unknown"

    @classmethod
    def from_value(cls, raw_value: Any) -> "ActionStatus":
        if isinstance(raw_value, cls):
            return raw_value
        if isinstance(raw_value, str):
            normalized = raw_value.strip().lower()
            for member in cls:
                if member.value == normalized:
                    return member
        return cls.UNKNOWN


@dataclass
class InputEvent:
    request_id: str = ""
    session_id: str = ""
    utterance_id: str = ""
    text: str = ""
    alternatives: list[str] = field(default_factory=list)
    locale: str = "vi-VN"
    input_type: str = "voice"

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, payload: dict[str, Any] | None) -> "InputEvent":
        if not payload:
            return cls()
        return cls(
            request_id=str(payload.get("request_id", payload.get("requestId", ""))),
            session_id=str(payload.get("session_id", payload.get("sessionId", ""))),
            utterance_id=str(payload.get("utterance_id", payload.get("utteranceId", ""))),
            text=str(payload.get("text", "")),
            alternatives=list(payload.get("alternatives", [])),
            locale=str(payload.get("locale", "vi-VN")),
            input_type=str(payload.get("input_type", payload.get("inputType", "voice"))),
        )


@dataclass
class RequestContext:
    client_id: str = ""
    is_authenticated: bool = False
    deadline_epoch_ms: int = 0
    client_capabilities: list[str] = field(default_factory=list)
    viewer_profile: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, payload: dict[str, Any] | None) -> "RequestContext":
        if not payload:
            return cls()
        authenticated = payload.get("is_authenticated", payload.get("isAuthenticated", False))
        deadline = payload.get("deadline_epoch_ms", payload.get("deadlineEpochMs", 0))
        capabilities = payload.get("client_capabilities", payload.get("clientCapabilities", []))
        profile = payload.get("viewer_profile", payload.get("viewerProfile"))
        return cls(
            client_id=str(payload.get("client_id", payload.get("clientId", ""))),
            is_authenticated=bool(authenticated),
            deadline_epoch_ms=int(deadline),
            client_capabilities=list(capabilities),
            viewer_profile=str(profile) if profile is not None else None,
        )


@dataclass
class Intent:
    name: str = ""
    domain: str = ""
    confidence: float = 0.0
    slots: dict[str, Any] = field(default_factory=dict)
    explicit_source: str = ""
    missing_slots: list[str] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, payload: dict[str, Any] | None) -> "Intent":
        if not payload:
            return cls()
        confidence_val = payload.get("confidence", 0.0)
        source = payload.get("explicit_source", payload.get("explicitSource", ""))
        missing = payload.get("missing_slots", payload.get("missingSlots", []))
        return cls(
            name=str(payload.get("name", "")),
            domain=str(payload.get("domain", "")),
            confidence=float(confidence_val),
            slots=dict(payload.get("slots", {})),
            explicit_source=str(source),
            missing_slots=list(missing),
        )


@dataclass
class ActionStep:
    step_id: str = ""
    action_type: str = ""
    target_id: str = ""
    parameters: dict[str, Any] = field(default_factory=dict)
    runs_on: str = "backend"
    timeout_ms: int = 5000
    is_idempotent: bool = False

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, payload: dict[str, Any] | None) -> "ActionStep":
        if not payload:
            return cls()
        step_identifier = payload.get("step_id", payload.get("stepId", ""))
        action_name = payload.get("action_type", payload.get("actionType", ""))
        target_identifier = payload.get("target_id", payload.get("targetId", ""))
        execution_target = payload.get("runs_on", payload.get("runsOn", "backend"))
        timeout_value = payload.get("timeout_ms", payload.get("timeoutMs", 5000))
        idempotent_flag = payload.get("is_idempotent", payload.get("isIdempotent", False))
        return cls(
            step_id=str(step_identifier),
            action_type=str(action_name),
            target_id=str(target_identifier),
            parameters=dict(payload.get("parameters", {})),
            runs_on=str(execution_target),
            timeout_ms=int(timeout_value),
            is_idempotent=bool(idempotent_flag),
        )


@dataclass
class ActionPlan:
    plan_id: str = ""
    intent_name: str = ""
    steps: list[ActionStep] = field(default_factory=list)
    confirmation_required: bool = False
    evidence: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "plan_id": self.plan_id,
            "intent_name": self.intent_name,
            "steps": [step.to_dict() for step in self.steps],
            "confirmation_required": self.confirmation_required,
            "evidence": self.evidence,
        }

    @classmethod
    def from_dict(cls, payload: dict[str, Any] | None) -> "ActionPlan":
        if not payload:
            return cls()
        plan_identifier = payload.get("plan_id", payload.get("planId", ""))
        resolved_intent = payload.get("intent_name", payload.get("intentName", ""))
        raw_steps = payload.get("steps", [])
        parsed_steps = [
            ActionStep.from_dict(step) if isinstance(step, dict) else step
            for step in raw_steps
        ]
        require_confirmation = payload.get(
            "confirmation_required", payload.get("confirmationRequired", False)
        )
        return cls(
            plan_id=str(plan_identifier),
            intent_name=str(resolved_intent),
            steps=parsed_steps,
            confirmation_required=bool(require_confirmation),
            evidence=dict(payload.get("evidence", {})),
        )


@dataclass
class ExecutionResult:
    step_id: str = ""
    status: ActionStatus = ActionStatus.UNKNOWN
    error_code: str | None = None
    detail: str | None = None
    actual_state: dict[str, Any] = field(default_factory=dict)
    can_retry: bool = False

    def to_dict(self) -> dict[str, Any]:
        return {
            "step_id": self.step_id,
            "status": self.status.value,
            "error_code": self.error_code,
            "detail": self.detail,
            "actual_state": self.actual_state,
            "can_retry": self.can_retry,
        }

    @classmethod
    def from_dict(cls, payload: dict[str, Any] | None) -> "ExecutionResult":
        if not payload:
            return cls()
        step_identifier = payload.get("step_id", payload.get("stepId", ""))
        status_value = ActionStatus.from_value(payload.get("status"))
        code = payload.get("error_code", payload.get("errorCode"))
        description = payload.get("detail")
        state_data = payload.get("actual_state", payload.get("actualState", {}))
        retryable = payload.get("can_retry", payload.get("canRetry", False))
        return cls(
            step_id=str(step_identifier),
            status=status_value,
            error_code=str(code) if code is not None else None,
            detail=str(description) if description is not None else None,
            actual_state=dict(state_data),
            can_retry=bool(retryable),
        )


@dataclass
class PresentationResponse:
    speech_text: str = ""
    cards: list[dict[str, Any]] = field(default_factory=list)
    quick_chips: list[str] = field(default_factory=list)
    client_actions: list[dict[str, Any]] = field(default_factory=list)
    mood: str = "neutral"

    def to_dict(self) -> dict[str, Any]:
        return {
            "speech_text": self.speech_text,
            "cards": self.cards,
            "quick_chips": self.quick_chips,
            "client_actions": self.client_actions,
            "mood": self.mood,
        }

    @classmethod
    def from_dict(cls, payload: dict[str, Any] | None) -> "PresentationResponse":
        if not payload:
            return cls()
        speech = payload.get("speech_text", payload.get("speechText", ""))
        card_list = payload.get("cards", [])
        chips = payload.get("quick_chips", payload.get("quickChips", []))
        actions = payload.get("client_actions", payload.get("clientActions", []))
        tone = payload.get("mood", "neutral")
        return cls(
            speech_text=str(speech),
            cards=list(card_list),
            quick_chips=list(chips),
            client_actions=list(actions),
            mood=str(tone),
        )
