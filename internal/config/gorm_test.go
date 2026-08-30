package config

import "testing"

func TestBuildDSN(t *testing.T) {
	cfg := &Config{}
	cfg.Database.Host = "db.supabase.co"
	cfg.Database.User = "postgres"
	cfg.Database.Password = "secret"
	cfg.Database.Name = "postgres"
	cfg.Database.Port = 5432
	cfg.Database.SSLMode = "require"

	got := buildDSN(cfg)
	want := "host=db.supabase.co user=postgres password=secret dbname=postgres port=5432 sslmode=require"

	if got != want {
		t.Errorf("buildDSN() = %q, want %q", got, want)
	}
}
