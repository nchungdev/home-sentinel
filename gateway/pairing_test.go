package main

import (
	"testing"
	"time"
)

func newTestPairingStore(now *time.Time) *PairingStore {
	return NewPairingStore(func() time.Time { return *now })
}

func TestNewPairingHasSixDigitCode(t *testing.T) {
	now := time.Now()
	request, _ := newTestPairingStore(&now).Create("Living room TV")

	if len(request.Code) != pairingCodeDigits || request.Status != pairingPending {
		t.Fatalf("unexpected request %+v", request)
	}
	for _, digit := range request.Code {
		if digit < '0' || digit > '9' {
			t.Fatalf("code must be numeric, got %q", request.Code)
		}
	}
}

func TestBlankNameFallsBackToTV(t *testing.T) {
	now := time.Now()
	request, _ := newTestPairingStore(&now).Create("   ")

	if request.Name != defaultDeviceName {
		t.Fatalf("expected default name, got %q", request.Name)
	}
}

func TestApprovedTokenIsDeliveredOnce(t *testing.T) {
	now := time.Now()
	store := newTestPairingStore(&now)
	request, _ := store.Create("TV")
	nonce, _ := store.BeginApproval(request.ID)
	if _, approveErr := store.CompleteApproval(nonce, "token"); approveErr != nil {
		t.Fatalf("approve: %v", approveErr)
	}

	first, firstErr := store.Collect(request.ID)
	_, secondErr := store.Collect(request.ID)

	if firstErr != nil || first.AccessToken != "token" || secondErr == nil {
		t.Fatalf("token must be delivered exactly once: %+v %v", first, secondErr)
	}
}

func TestApprovalNonceWorksOnce(t *testing.T) {
	now := time.Now()
	store := newTestPairingStore(&now)
	request, _ := store.Create("TV")
	nonce, _ := store.BeginApproval(request.ID)
	_, _ = store.CompleteApproval(nonce, "token")

	if _, replayErr := store.CompleteApproval(nonce, "other"); replayErr == nil {
		t.Fatalf("a nonce must not be reusable")
	}
}

func TestRejectedRequestHasNoToken(t *testing.T) {
	now := time.Now()
	store := newTestPairingStore(&now)
	request, _ := store.Create("TV")
	_ = store.Reject(request.ID)

	collected, _ := store.Collect(request.ID)

	if collected.Status != pairingRejected || collected.AccessToken != "" {
		t.Fatalf("unexpected %+v", collected)
	}
}

func TestRequestsExpire(t *testing.T) {
	now := time.Now()
	store := newTestPairingStore(&now)
	request, _ := store.Create("TV")
	now = now.Add(pairingTTL + time.Second)

	if _, collectErr := store.Collect(request.ID); collectErr == nil {
		t.Fatalf("expired request must be gone")
	}
}

func TestPendingRequestsAreCapped(t *testing.T) {
	now := time.Now()
	store := newTestPairingStore(&now)
	for count := 0; count < maxPendingPairings; count++ {
		if _, createErr := store.Create("TV"); createErr != nil {
			t.Fatalf("create %d: %v", count, createErr)
		}
	}

	if _, createErr := store.Create("one too many"); createErr == nil {
		t.Fatalf("expected the cap to refuse more requests")
	}
}
