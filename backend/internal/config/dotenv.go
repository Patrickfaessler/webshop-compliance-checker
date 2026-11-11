// Package config loads settings from the environment, optionally filled from deploy/.env (ADR-5).
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv reads KEY=VALUE lines from the first existing file in paths and sets each variable that is
// not already set in the environment (real environment variables win). Blank lines and # comments are
// ignored; values may be wrapped in single or double quotes. It returns the file it used, or "".
func LoadDotEnv(paths ...string) (string, error) {
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key, val = strings.TrimSpace(key), strings.TrimSpace(val)
			if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
			if _, set := os.LookupEnv(key); !set && key != "" {
				_ = os.Setenv(key, val)
			}
		}
		abs, _ := filepath.Abs(p)
		return abs, sc.Err()
	}
	return "", nil
}

// DefaultDotEnvPaths are tried from the repository root or from backend/.
var DefaultDotEnvPaths = []string{"deploy/.env", "../deploy/.env"}
