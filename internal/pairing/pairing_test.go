package pairing

import (
	"testing"
	"time"
)

func TestApprovedPairingIsCollectedOnce(t *testing.T) {
	store := NewStore(time.Minute)
	entry, err := store.Start()
	if err != nil {
		t.Fatal(err)
	}

	if approved, found := store.Poll(entry.ID); approved || !found {
		t.Fatalf("want pending, got approved=%v found=%v", approved, found)
	}
	if !store.Approve(" " + entry.Code[:3] + "-" + entry.Code[3:] + " ") {
		t.Fatal("approve with spaced code failed")
	}
	if approved, found := store.Poll(entry.ID); !approved || !found {
		t.Fatalf("want approved, got approved=%v found=%v", approved, found)
	}
	if _, found := store.Poll(entry.ID); found {
		t.Fatal("approved pairing must be collected only once")
	}
}

func TestExpiredPairingCannotBeApproved(t *testing.T) {
	store := NewStore(time.Minute)
	now := time.Now()
	store.now = func() time.Time { return now }
	entry, err := store.Start()
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Minute)
	if store.Approve(entry.Code) {
		t.Fatal("expired code was approved")
	}
	if _, found := store.Poll(entry.ID); found {
		t.Fatal("expired pairing still found")
	}
}

func TestUnknownCodeIsRejected(t *testing.T) {
	store := NewStore(time.Minute)
	if store.Approve("ZZZZZZ") {
		t.Fatal("unknown code approved")
	}
}
