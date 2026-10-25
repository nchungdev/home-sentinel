import json
import os
from pathlib import Path
import unittest

from contracts import (
    ActionPlan,
    ActionStatus,
    ActionStep,
    ContextSnapshot,
    ExecutionResult,
    InputEvent,
    Intent,
    PresentationResponse,
    ProactiveChip,
    RequestContext,
    Suggestion,
)


class ContractsInitializationTest(unittest.TestCase):
    def test_action_status_members(self):
        self.assertEqual(ActionStatus.ACCEPTED.value, "accepted")
        self.assertEqual(ActionStatus.RUNNING.value, "running")
        self.assertEqual(ActionStatus.NEEDS_INPUT.value, "needs_input")
        self.assertEqual(ActionStatus.SUCCEEDED.value, "succeeded")
        self.assertEqual(ActionStatus.FAILED.value, "failed")
        self.assertEqual(ActionStatus.CANCELLED.value, "cancelled")
        self.assertEqual(ActionStatus.UNKNOWN.value, "unknown")
        self.assertEqual(ActionStatus.from_value("running"), ActionStatus.RUNNING)
        self.assertEqual(ActionStatus.from_value("invalid_status"), ActionStatus.UNKNOWN)

    def test_input_event_defaults_and_serialization(self):
        event = InputEvent()
        self.assertEqual(event.request_id, "")
        self.assertEqual(event.locale, "vi-VN")
        self.assertEqual(event.input_type, "voice")

        custom_event = InputEvent(
            request_id="req-123",
            session_id="sess-456",
            utterance_id="utt-789",
            text="bật đèn",
            alternatives=["mở đèn"],
            locale="vi-VN",
            input_type="voice",
        )
        serialized = custom_event.to_dict()
        self.assertEqual(serialized["request_id"], "req-123")
        restored = InputEvent.from_dict(serialized)
        self.assertEqual(restored.request_id, "req-123")
        self.assertEqual(restored.alternatives, ["mở đèn"])

    def test_request_context_defaults_and_serialization(self):
        context = RequestContext()
        self.assertFalse(context.is_authenticated)
        self.assertIsNone(context.viewer_profile)

        custom_context = RequestContext(
            client_id="launcher-living-room",
            is_authenticated=True,
            deadline_epoch_ms=1729849200000,
            client_capabilities=["voice", "cards", "quick_chips"],
            viewer_profile="husband",
        )
        serialized = custom_context.to_dict()
        restored = RequestContext.from_dict(serialized)
        self.assertTrue(restored.is_authenticated)
        self.assertEqual(restored.viewer_profile, "husband")
        self.assertEqual(len(restored.client_capabilities), 3)

    def test_intent_defaults_and_serialization(self):
        intent = Intent()
        self.assertEqual(intent.confidence, 0.0)

        custom_intent = Intent(
            name="turn_on_device",
            domain="home_assistant",
            confidence=0.98,
            slots={"entity_id": "light.living_room"},
            explicit_source="rule_engine",
            missing_slots=[],
        )
        serialized = custom_intent.to_dict()
        restored = Intent.from_dict(serialized)
        self.assertEqual(restored.name, "turn_on_device")
        self.assertEqual(restored.slots["entity_id"], "light.living_room")

    def test_action_step_and_plan_roundtrip(self):
        step = ActionStep(
            step_id="step-1",
            action_type="service_call",
            target_id="light.turn_on",
            parameters={"entity_id": "light.living_room"},
            runs_on="backend",
            timeout_ms=3000,
            is_idempotent=True,
        )
        plan = ActionPlan(
            plan_id="plan-1",
            intent_name="turn_on_device",
            steps=[step],
            confirmation_required=False,
            evidence={"matched_alias": "đèn phòng khách"},
        )
        serialized = plan.to_dict()
        self.assertEqual(len(serialized["steps"]), 1)
        restored = ActionPlan.from_dict(serialized)
        self.assertEqual(restored.plan_id, "plan-1")
        self.assertEqual(len(restored.steps), 1)
        self.assertEqual(restored.steps[0].step_id, "step-1")
        self.assertTrue(restored.steps[0].is_idempotent)

    def test_execution_result_serialization(self):
        result = ExecutionResult(
            step_id="step-1",
            status=ActionStatus.SUCCEEDED,
            error_code=None,
            detail="Bật đèn thành công",
            actual_state={"state": "on"},
            can_retry=False,
        )
        serialized = result.to_dict()
        self.assertEqual(serialized["status"], "succeeded")
        restored = ExecutionResult.from_dict(serialized)
        self.assertEqual(restored.status, ActionStatus.SUCCEEDED)
        self.assertEqual(restored.actual_state["state"], "on")

    def test_presentation_response_serialization(self):
        response = PresentationResponse(
            speech_text="Đã bật đèn phòng khách",
            cards=[{"type": "device_card", "title": "Đèn phòng khách"}],
            quick_chips=["Tắt đèn", "Chỉnh 50%"],
            client_actions=[],
            mood="happy",
        )
        serialized = response.to_dict()
        restored = PresentationResponse.from_dict(serialized)
        self.assertEqual(restored.speech_text, "Đã bật đèn phòng khách")
        self.assertEqual(len(restored.cards), 1)
        self.assertEqual(len(restored.quick_chips), 2)
        self.assertEqual(restored.mood, "happy")

    def test_suggestion_serialization(self):
        suggestion = Suggestion(
            id="sug-101",
            kind="media_playback",
            title="Xem tiếp tập mới",
            message="Tập mới của Breaking Bad đã sẵn sàng",
            target_id="plex.tv",
            action=ActionStep(
                step_id="step-play",
                action_type="play_media",
                target_id="plex.play",
                parameters={"media_id": "12345"},
            ),
            evidence={"source": "radarr", "downloaded_at": 1729850000000},
            generated_at=1729850100000,
            expires_at=1729853700000,
            dedupe_key="media_breaking_bad_s05e16",
        )
        serialized = suggestion.to_dict()
        self.assertEqual(serialized["id"], "sug-101")
        self.assertEqual(serialized["action"]["target_id"], "plex.play")

        restored = Suggestion.from_dict(serialized)
        self.assertEqual(restored.id, "sug-101")
        self.assertEqual(restored.kind, "media_playback")
        self.assertEqual(restored.title, "Xem tiếp tập mới")
        self.assertEqual(restored.action.action_type, "play_media")
        self.assertEqual(restored.action.parameters["media_id"], "12345")
        self.assertEqual(restored.evidence["source"], "radarr")
        self.assertEqual(restored.generated_at, 1729850100000)
        self.assertEqual(restored.expires_at, 1729853700000)
        self.assertEqual(restored.dedupe_key, "media_breaking_bad_s05e16")

    def test_context_snapshot_serialization(self):
        snapshot = ContextSnapshot(
            timestamp_epoch_ms=1729850000000,
            time_of_day="evening",
            viewer_profile="husband",
            environment={"temperature": 26.5, "ac_mode": "cool"},
            recent_media=[{"title": "Breaking Bad", "season": 5, "episode": 16}],
        )
        serialized = snapshot.to_dict()
        self.assertEqual(serialized["time_of_day"], "evening")

        restored = ContextSnapshot.from_dict(serialized)
        self.assertEqual(restored.timestamp_epoch_ms, 1729850000000)
        self.assertEqual(restored.time_of_day, "evening")
        self.assertEqual(restored.viewer_profile, "husband")
        self.assertEqual(restored.environment["temperature"], 26.5)
        self.assertEqual(len(restored.recent_media), 1)

    def test_proactive_chip_serialization(self):
        chip = ProactiveChip(
            chip_id="chip-coffee",
            label="Pha cà phê sáng",
            icon_key="coffee",
            action_payload={"entity_id": "switch.coffee_maker"},
        )
        serialized = chip.to_dict()
        restored = ProactiveChip.from_dict(serialized)
        self.assertEqual(restored.chip_id, "chip-coffee")
        self.assertEqual(restored.label, "Pha cà phê sáng")
        self.assertEqual(restored.icon_key, "coffee")
        self.assertEqual(restored.action_payload["entity_id"], "switch.coffee_maker")


class ContractFixturesTest(unittest.TestCase):
    def setUp(self):
        self.fixtures_dir = Path(__file__).parent / "pkg" / "contracts" / "fixtures"

    def _read_fixture(self, filename: str) -> dict:
        with open(self.fixtures_dir / filename, "r", encoding="utf-8") as f:
            return json.load(f)

    def test_input_event_fixtures(self):
        valid_data = self._read_fixture("input_event_valid.json")
        event = InputEvent.from_dict(valid_data)
        self.assertEqual(event.request_id, "req-1001")
        self.assertEqual(event.text, "bật quạt phòng khách")
        self.assertEqual(len(event.alternatives), 2)
        self.assertEqual(event.locale, "vi-VN")

        invalid_data = self._read_fixture("input_event_invalid.json")
        invalid_event = InputEvent.from_dict(invalid_data)
        self.assertEqual(invalid_event.request_id, "")
        self.assertEqual(invalid_event.text, "")

    def test_intent_fixture(self):
        data = self._read_fixture("intent_valid.json")
        intent = Intent.from_dict(data)
        self.assertEqual(intent.name, "turn_on")
        self.assertEqual(intent.domain, "home_assistant")
        self.assertAlmostEqual(intent.confidence, 0.95)
        self.assertEqual(intent.slots["device"], "fan")

    def test_action_plan_fixture(self):
        data = self._read_fixture("action_plan_valid.json")
        plan = ActionPlan.from_dict(data)
        self.assertEqual(plan.plan_id, "plan-101")
        self.assertEqual(plan.intent_name, "turn_on")
        self.assertEqual(len(plan.steps), 1)
        self.assertEqual(plan.steps[0].step_id, "step-1")
        self.assertEqual(plan.steps[0].action_type, "service_call")
        self.assertTrue(plan.steps[0].is_idempotent)

    def test_presentation_response_fixture(self):
        data = self._read_fixture("presentation_response_valid.json")
        response = PresentationResponse.from_dict(data)
        self.assertEqual(response.speech_text, "Đã bật quạt phòng khách")
        self.assertEqual(len(response.cards), 1)
        self.assertEqual(len(response.quick_chips), 2)
        self.assertEqual(response.mood, "friendly")

    def test_suggestion_fixture(self):
        data = self._read_fixture("suggestion_valid.json")
        sug = Suggestion.from_dict(data)
        self.assertEqual(sug.id, "sug-901")
        self.assertEqual(sug.kind, "turn_off_reminder")
        self.assertEqual(sug.action.step_id, "step-sug-1")
        self.assertEqual(sug.generated_at, 1729850000000)

    def test_context_snapshot_fixture(self):
        data = self._read_fixture("context_snapshot_valid.json")
        snapshot = ContextSnapshot.from_dict(data)
        self.assertEqual(snapshot.timestamp_epoch_ms, 1729850000000)
        self.assertEqual(snapshot.time_of_day, "evening")
        self.assertEqual(snapshot.viewer_profile, "husband")
        self.assertEqual(len(snapshot.recent_media), 1)

    def test_proactive_chip_fixture(self):
        data = self._read_fixture("proactive_chip_valid.json")
        chip = ProactiveChip.from_dict(data)
        self.assertEqual(chip.chip_id, "chip-quick-1")
        self.assertEqual(chip.label, "Tắt đèn phòng khách")



if __name__ == "__main__":
    unittest.main()
