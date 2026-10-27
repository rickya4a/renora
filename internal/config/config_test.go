package config

import "testing"

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/renora")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if !*cfg.GraphQL.Introspection || !*cfg.GraphQL.Playground {
		t.Error("introspection and playground should default on outside production")
	}
}

func TestLoadProductionDefaults(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_ENV", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if *cfg.GraphQL.Introspection || *cfg.GraphQL.Playground {
		t.Error("introspection and playground should default off in production")
	}
}

func TestLoadRejectsUnsafeProduction(t *testing.T) {
	tests := map[string]map[string]string{
		"playground on": {"GRAPHQL_PLAYGROUND": "true"},
		"wildcard cors": {"CORS_ALLOWED_ORIGINS": "https://app.renora.id,*"},
	}
	for name, vars := range tests {
		t.Run(name, func(t *testing.T) {
			setRequired(t)
			t.Setenv("APP_ENV", "production")
			for k, v := range vars {
				t.Setenv(k, v)
			}
			if _, err := Load(); err == nil {
				t.Fatal("Load: want error, got nil")
			}
		})
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load: want error for empty DATABASE_URL")
	}
}
