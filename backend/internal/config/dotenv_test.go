package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Spec: ADR-5 (secrets only via deploy/.env or the environment)
func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "# comment\n\nCFGTEST_A=one\nexport CFGTEST_B = \"two words\"\nCFGTEST_C='x=y'\nCFGTEST_SET=from-file\nnot a pair\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CFGTEST_SET", "from-env")
	for _, k := range []string{"CFGTEST_A", "CFGTEST_B", "CFGTEST_C"} {
		t.Setenv(k, "") // registers cleanup; then unset so the file can fill it
		os.Unsetenv(k)
	}

	used, err := LoadDotEnv(filepath.Join(dir, "missing.env"), p)
	if err != nil || used == "" {
		t.Fatalf("used=%q err=%v", used, err)
	}
	for k, want := range map[string]string{"CFGTEST_A": "one", "CFGTEST_B": "two words", "CFGTEST_C": "x=y", "CFGTEST_SET": "from-env"} {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestLoadDotEnvNoFile(t *testing.T) {
	used, err := LoadDotEnv(filepath.Join(t.TempDir(), "nope"))
	if used != "" || err != nil {
		t.Fatalf("used=%q err=%v", used, err)
	}
}
