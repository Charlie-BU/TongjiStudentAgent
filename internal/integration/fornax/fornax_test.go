package fornax

import "testing"

func TestEnabled(t *testing.T) {
	t.Setenv("FORNAX_ENABLED", "true")
	if !Enabled() {
		t.Fatal("Enabled() = false, want true")
	}

	t.Setenv("FORNAX_ENABLED", "false")
	if Enabled() {
		t.Fatal("Enabled() = true, want false")
	}
}
