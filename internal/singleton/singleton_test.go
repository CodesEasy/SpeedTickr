package singleton

import "testing"

// TestAcquireBlocksSecond verifies the first Acquire wins and a second reports that an
// instance is already running. flock conflicts across open file descriptions and the
// named mutex reports ERROR_ALREADY_EXISTS, so this holds within one process on every
// platform — no second binary needed to exercise it.
func TestAcquireBlocksSecond(t *testing.T) {
	release, ok, err := Acquire()
	if err != nil {
		t.Skipf("cannot acquire a lock in this environment: %v", err)
	}
	if !ok {
		t.Fatal("first Acquire should succeed (no other instance in a fresh test process)")
	}
	defer release()

	_, ok2, err2 := Acquire()
	if err2 != nil {
		t.Fatalf("second Acquire errored: %v", err2)
	}
	if ok2 {
		t.Error("second Acquire should report that an instance is already running")
	}
}
