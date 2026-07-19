package sandbox

import "testing"

func TestEnabledFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{name: "unset", want: false},
		{name: "disabled", value: "false", want: false},
		{name: "enabled", value: "true", want: true},
		{name: "numeric enabled", value: "1", want: true},
		{name: "invalid", value: "enabled", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("SANDBOX_ENABLED", test.value)

			enabled, err := EnabledFromEnv()
			if test.wantErr {
				if err == nil {
					t.Fatal("EnabledFromEnv() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("EnabledFromEnv() error = %v", err)
			}
			if enabled != test.want {
				t.Fatalf("EnabledFromEnv() = %t, want %t", enabled, test.want)
			}
		})
	}
}
