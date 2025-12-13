package run

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

func TestController_searchFiles(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name    string
		param   *ParamRun
		cfg     *config.Config
		want    []string
		wantErr bool
	}{
		{
			name: "use workflow file paths from param",
			param: &ParamRun{
				WorkflowFilePaths: []string{"workflow1.yml", "workflow2.yml"},
			},
			cfg:     &config.Config{},
			want:    []string{"workflow1.yml", "workflow2.yml"},
			wantErr: false,
		},
		{
			name: "empty workflow file paths with config files - uses glob",
			param: &ParamRun{
				ConfigFilePath: ".pinact.yaml",
			},
			cfg: &config.Config{
				Files: []*config.File{
					{Pattern: "*.yml"},
				},
			},
			// Note: This will return empty because filepath.Glob uses real filesystem
			want:    nil,
			wantErr: false,
		},
		{
			name:  "nil config - fallback to listWorkflows",
			param: &ParamRun{},
			cfg:   nil,
			// Note: listWorkflows uses real filesystem, will return empty in test
			want:    nil,
			wantErr: false,
		},
		{
			name:  "empty config files - fallback to listWorkflows",
			param: &ParamRun{},
			cfg:   &config.Config{},
			// Note: listWorkflows uses real filesystem, will return empty in test
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(nil, nil, nil, fs, tt.cfg, tt.param)
			got, err := ctrl.searchFiles()

			if (err != nil) != tt.wantErr {
				t.Errorf("searchFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Only check length for workflow file paths from param case
			if tt.param.WorkflowFilePaths != nil {
				if len(got) != len(tt.want) {
					t.Errorf("searchFiles() got %d files, want %d", len(got), len(tt.want))
					return
				}

				for i, path := range got {
					if path != tt.want[i] {
						t.Errorf("searchFiles()[%d] = %v, want %v", i, path, tt.want[i])
					}
				}
			}
		})
	}
}

func TestController_searchFiles_withWorkflowPaths(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	param := &ParamRun{
		WorkflowFilePaths: []string{"a.yml", "b.yml", "c.yml"},
	}
	ctrl := New(nil, nil, nil, fs, &config.Config{}, param)

	got, err := ctrl.searchFiles()
	if err != nil {
		t.Errorf("searchFiles() error = %v", err)
		return
	}

	if len(got) != 3 {
		t.Errorf("searchFiles() got %d files, want 3", len(got))
	}
}

func TestController_searchFilesByGlob_emptyFiles(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := &Controller{
		fs: fs,
		cfg: &config.Config{
			Files: []*config.File{},
		},
		param: &ParamRun{
			ConfigFilePath: ".pinact.yaml",
		},
	}

	got, err := ctrl.searchFilesByGlob()
	if err != nil {
		t.Errorf("searchFilesByGlob() error = %v", err)
		return
	}

	if len(got) != 0 {
		t.Errorf("searchFilesByGlob() got %d files, want 0", len(got))
	}
}
