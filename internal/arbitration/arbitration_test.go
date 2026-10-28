package arbitration

import "testing"

func TestArbitrationRemoteMicOverride(t *testing.T) {
	a := NewAudioArbitrator(0.15)
	decision := a.Arbitrate(Context{RemoteMicActive: true, TVScreenOn: false})
	if decision.ListeningDevice != DeviceTV || decision.Reason != ReasonRemotePushToTalk {
		t.Fatalf("expected tv with %s, got %+v", ReasonRemotePushToTalk, decision)
	}
}

func TestArbitrationTVOn(t *testing.T) {
	a := NewAudioArbitrator(0.15)
	decision := a.Arbitrate(Context{TVScreenOn: true, UserOnSofa: false})
	if decision.ListeningDevice != DeviceTV || decision.SpeakingDevice != DeviceTV {
		t.Fatalf("expected tv listening and speaking, got %+v", decision)
	}
}

func TestArbitrationTVOffUserOnSofa(t *testing.T) {
	a := NewAudioArbitrator(0.15)
	decision := a.Arbitrate(Context{TVScreenOn: false, UserOnSofa: true})
	if decision.ListeningDevice != DeviceCamera || decision.SpeakingDevice != DeviceCamera {
		t.Fatalf("expected camera when tv is off and user on sofa, got %+v", decision)
	}
	if decision.Reason != ReasonTVOff {
		t.Fatalf("expected reason %s, got %s", ReasonTVOff, decision.Reason)
	}
}

func TestArbitrationEnergyComparison(t *testing.T) {
	a := NewAudioArbitrator(0.15)
	decision := a.Arbitrate(Context{
		TVScreenOn:        false,
		TVAudioEnergy:     0.8,
		CameraAudioEnergy: 0.3,
	})
	if decision.ListeningDevice != DeviceTV || decision.Reason != ReasonEnergyComparison {
		t.Fatalf("expected TV listening on higher energy, got %+v", decision)
	}

	decision2 := a.Arbitrate(Context{
		TVScreenOn:        false,
		TVAudioEnergy:     0.2,
		CameraAudioEnergy: 0.7,
	})
	if decision2.ListeningDevice != DeviceCamera || decision2.Reason != ReasonEnergyComparison {
		t.Fatalf("expected Camera listening on higher energy, got %+v", decision2)
	}
}
