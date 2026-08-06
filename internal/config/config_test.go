package config

import "testing"

func TestLoadSettingsRequiresConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	if _, err := LoadSettings(); err == nil {
		t.Fatal("expected missing settings to fail")
	}
	t.Setenv("DATABASE_URL", "postgresql://example")
	t.Setenv("JWT_SECRET", "secret")
	if _, err := LoadSettings(); err != nil {
		t.Fatal(err)
	}
}
