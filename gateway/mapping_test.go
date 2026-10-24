package main

import (
	"errors"
	"testing"
)

var lightAttributes = Attributes{
	"friendly_name":         "Đèn trần",
	"supported_color_modes": []any{"color_temp"},
	"min_color_temp_kelvin": 2700.0,
	"max_color_temp_kelvin": 6500.0,
	"brightness":            180.0,
}

var climateAttributes = Attributes{
	"friendly_name":       "Điều hòa",
	"supported_features":  1.0,
	"min_temp":            16.0,
	"max_temp":            30.0,
	"temperature":         24.0,
	"current_temperature": 27.0,
	"hvac_modes":          []any{"off", "cool", "heat"},
}

func capabilityIDs(device Device) []string {
	ids := []string{}
	for _, capability := range device.Capabilities {
		ids = append(ids, capability.ID)
	}
	return ids
}

func describe(t *testing.T, entityID, state string, attributes Attributes) Device {
	t.Helper()

	device, describeErr := DescribeDevice(entityID, state, attributes)
	if describeErr != nil {
		t.Fatalf("describe %s: %v", entityID, describeErr)
	}
	return device
}

func expectActionError(t *testing.T, code string, resolveErr error) {
	t.Helper()

	var failure *ActionError
	if !errors.As(resolveErr, &failure) || failure.Code != code {
		t.Fatalf("expected %s error, got %v", code, resolveErr)
	}
}

func TestDimmableLightExposesPowerBrightnessAndColorTemperature(t *testing.T) {
	device := describe(t, "light.den_tran", "on", lightAttributes)

	ids := capabilityIDs(device)
	if len(ids) != 3 || ids[0] != "power" || ids[1] != "brightness" || ids[2] != "color_temp" {
		t.Fatalf("unexpected capabilities %v", ids)
	}
	if device.State["power"] != true || device.State["brightness"] != 180.0 {
		t.Fatalf("unexpected state %v", device.State)
	}
	if device.Type != "light" || device.Name != "Đèn trần" {
		t.Fatalf("unexpected device %+v", device)
	}
}

func TestOnOffLightOnlyHasPower(t *testing.T) {
	device := describe(t, "light.hanh_lang", "off", Attributes{
		"supported_color_modes": []any{"onoff"},
	})

	if ids := capabilityIDs(device); len(ids) != 1 || ids[0] != "power" {
		t.Fatalf("unexpected capabilities %v", ids)
	}
	if device.State["power"] != false {
		t.Fatalf("light should be off")
	}
}

func TestClimateModesSkipOffAndTargetHasRange(t *testing.T) {
	device := describe(t, "climate.phong_khach", "cool", climateAttributes)

	byID := map[string]CapabilityInfo{}
	for _, capability := range device.Capabilities {
		byID[capability.ID] = capability
	}
	if modes := byID["mode"].Options; len(modes) != 2 || modes[0] != "cool" {
		t.Fatalf("unexpected modes %v", modes)
	}
	target := byID["target_temp"]
	if *target.Min != 16 || *target.Max != 30 || target.Unit != "°C" {
		t.Fatalf("unexpected target range %+v", target)
	}
	if byID["current_temp"].Writable || device.State["current_temp"] != 27.0 {
		t.Fatalf("current temperature must be a read-only readout")
	}
}

func TestUnavailableDeviceIsFlagged(t *testing.T) {
	if describe(t, "switch.o_cam", "unavailable", Attributes{}).Available {
		t.Fatalf("device should be unavailable")
	}
}

func TestSensorIsReadOnlyWithUnit(t *testing.T) {
	device := describe(t, "sensor.nhiet_do", "26.5", Attributes{"unit_of_measurement": "°C"})

	readout := device.Capabilities[0]
	if readout.Kind != kindReadout || readout.Writable || readout.Unit != "°C" {
		t.Fatalf("unexpected readout %+v", readout)
	}
}

func TestCameraStreamPointsAtGateway(t *testing.T) {
	device := describe(t, "camera.cua", "idle", Attributes{})

	if device.Capabilities[0].URL != "/api/v1/devices/camera.cua/stream" {
		t.Fatalf("unexpected stream %q", device.Capabilities[0].URL)
	}
}

func TestUnknownDomainIsNotSupported(t *testing.T) {
	if isSupported("vacuum.robot") {
		t.Fatalf("vacuum is not in the catalog")
	}
}

func TestBrightnessCallsTurnOnWithValue(t *testing.T) {
	call, resolveErr := ResolveAction("light.den_tran", "brightness", 120.0, lightAttributes)

	if resolveErr != nil || call.Domain != "light" || call.Service != "turn_on" {
		t.Fatalf("unexpected call %+v %v", call, resolveErr)
	}
	if call.Data["entity_id"] != "light.den_tran" || call.Data["brightness"] != 120 {
		t.Fatalf("unexpected data %v", call.Data)
	}
}

func TestPowerFalseTurnsOff(t *testing.T) {
	call, _ := ResolveAction("light.den_tran", "power", false, lightAttributes)

	if call.Service != "turn_off" {
		t.Fatalf("expected turn_off, got %s", call.Service)
	}
}

func TestClimateTargetTemperature(t *testing.T) {
	call, _ := ResolveAction("climate.pk", "target_temp", 22.0, climateAttributes)

	if call.Service != "set_temperature" || call.Data["temperature"] != 22.0 {
		t.Fatalf("unexpected call %+v", call)
	}
}

func TestCoverOpenCommand(t *testing.T) {
	call, _ := ResolveAction("cover.rem", "command", "open", Attributes{})

	if call.Domain != "cover" || call.Service != "open_cover" {
		t.Fatalf("unexpected call %+v", call)
	}
}

func TestLockToggleTrueMeansLocked(t *testing.T) {
	call, _ := ResolveAction("lock.cua", "locked", true, Attributes{})

	if call.Service != "lock" {
		t.Fatalf("expected lock, got %s", call.Service)
	}
}

func TestSceneTrigger(t *testing.T) {
	call, _ := ResolveAction("scene.xem_phim", "run", nil, Attributes{})

	if call.Domain != "scene" || call.Service != "turn_on" {
		t.Fatalf("unexpected call %+v", call)
	}
}

func TestMediaVolumeIsScaledToFraction(t *testing.T) {
	call, _ := ResolveAction(
		"media_player.tv", "volume", 50.0, Attributes{"supported_features": 4.0},
	)

	if call.Data["volume_level"] != 0.5 {
		t.Fatalf("unexpected volume %v", call.Data["volume_level"])
	}
}

func TestInvalidActionsAreRejected(t *testing.T) {
	_, outOfRange := ResolveAction("light.den_tran", "brightness", 999.0, lightAttributes)
	_, wrongType := ResolveAction("light.den_tran", "power", "on", lightAttributes)
	_, badOption := ResolveAction("climate.pk", "mode", "dry", climateAttributes)
	_, readOnly := ResolveAction("sensor.nhiet_do", "value", 1.0, Attributes{})
	_, missing := ResolveAction("light.hanh_lang", "brightness", 10.0, Attributes{
		"supported_color_modes": []any{"onoff"},
	})
	_, unsupported := ResolveAction("vacuum.robot", "power", true, Attributes{})

	expectActionError(t, errorInvalidValue, outOfRange)
	expectActionError(t, errorInvalidValue, wrongType)
	expectActionError(t, errorInvalidValue, badOption)
	expectActionError(t, errorReadOnly, readOnly)
	expectActionError(t, errorUnsupported, missing)
	expectActionError(t, errorUnsupported, unsupported)
}
