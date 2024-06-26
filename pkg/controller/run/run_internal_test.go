package run

import (
	"testing"

	"github.com/spf13/afero"
)

func TestController_getConfigPath(t *testing.T) {
	t.Parallel()
	data := []struct {
		name  string
		paths []string
		exp   string
	}{
		{
			name:  "no config",
			paths: []string{},
			exp:   "",
		},
		{
			name:  "primary",
			paths: []string{".pinact.yaml"},
			exp:   ".pinact.yaml",
		},
		{
			name:  "another",
			paths: []string{".github/pinact.yaml"},
			exp:   ".github/pinact.yaml",
		},
		{
			name:  "both primary and others",
			paths: []string{".pinact.yaml", ".github/pinact.yaml"},
			exp:   ".pinact.yaml",
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			for _, path := range d.paths {
				if err := afero.WriteFile(fs, path, []byte(""), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := getConfigPath(fs)
			if err != nil {
				t.Fatal(err)
			}
			if got != d.exp {
				t.Fatalf(`wanted %s, got %s`, d.exp, got)
			}
		})
	}
}
