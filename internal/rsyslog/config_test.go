package rsyslog

import "testing"

func TestPrepareDoesNotError(t *testing.T) {
	if err := Prepare(); err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
}
