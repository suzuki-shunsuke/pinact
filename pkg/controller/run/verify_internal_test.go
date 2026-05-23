package run

import (
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
)

func TestController_verify(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name         string
		action       *Action
		expectedSHA  string
		expectedLine string
		shouldError  bool
		fix          bool
	}{
		{
			name: "matching SHA - no correction needed",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				RepoOwner:               "actions",
				RepoName:                "checkout",
				Version:                 "83b7061638ee4956cf7545a6f7efe594e5ad0247",
				VersionComment:          "v3.5.1",
				VersionCommentSeparator: " # ",
			},
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "",
			shouldError:  false,
			fix:          true,
		},
		{
			name: "mismatched SHA with Fix - comment corrected",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				RepoOwner:               "actions",
				RepoName:                "checkout",
				Version:                 "ee0669bd1cc54295c223e0bb666b733df41de1c5", // v2.7.0 SHA
				VersionComment:          "v3.5.1",
				VersionCommentSeparator: " # ",
			},
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "  - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0",
			shouldError:  false,
			fix:          true,
		},
		{
			name: "mismatched SHA without Fix - error",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				RepoOwner:               "actions",
				RepoName:                "checkout",
				Version:                 "ee0669bd1cc54295c223e0bb666b733df41de1c5",
				VersionComment:          "v3.5.1",
				VersionCommentSeparator: " # ",
			},
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "",
			shouldError:  true,
			fix:          false,
		},
		{
			name: "mismatched SHA with Fix but no matching tag - error",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				RepoOwner:               "actions",
				RepoName:                "checkout",
				Version:                 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				VersionComment:          "v3.5.1",
				VersionCommentSeparator: " # ",
			},
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "",
			shouldError:  true,
			fix:          true,
		},
	}

	logger := slog.New(slog.DiscardHandler)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()

			mockService := &github.RepositoriesServiceImpl{
				Commits: map[string]*github.GetCommitSHA1Result{
					tt.action.RepoOwner + "/" + tt.action.RepoName + "/" + tt.action.VersionComment: {
						SHA: tt.expectedSHA,
					},
				},
				Tags: map[string]*github.ListTagsResult{
					tt.action.RepoOwner + "/" + tt.action.RepoName + "/0": {
						Tags: []*github.RepositoryTag{
							{
								Name: new("v2.7.0"),
								Commit: &github.Commit{
									SHA: new("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
								},
							},
							{
								Name: new("v3.5.1"),
								Commit: &github.Commit{
									SHA: new("83b7061638ee4956cf7545a6f7efe594e5ad0247"),
								},
							},
						},
						Response: &github.Response{},
					},
				},
			}

			ctrl := New(mockService, nil, fs, &config.Config{
				Separator: " # ",
			}, &ParamRun{
				Fix:    tt.fix,
				Stderr: &bytes.Buffer{},
			})

			line, err := ctrl.verify(context.Background(), logger, tt.action)

			if tt.shouldError && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if line != tt.expectedLine {
				t.Fatalf("expected line %q but got %q", tt.expectedLine, line)
			}
		})
	}
}

func TestController_verifyIfNeeded(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name         string
		action       *Action
		isVerify     bool
		fix          bool
		expectedSHA  string
		expectedLine string
		shouldError  bool
	}{
		{
			name: "verify disabled - no verification",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				RepoOwner:               "actions",
				RepoName:                "checkout",
				Version:                 "wrongsha123",
				VersionComment:          "v3.5.1",
				VersionCommentSeparator: " # ",
			},
			isVerify:     false,
			expectedLine: "",
			shouldError:  false,
		},
		{
			name: "verify with -fix=false - mismatch errors",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				RepoOwner:               "actions",
				RepoName:                "checkout",
				Version:                 "ee0669bd1cc54295c223e0bb666b733df41de1c5",
				VersionComment:          "v3.5.1",
				VersionCommentSeparator: " # ",
			},
			isVerify:     true,
			fix:          false,
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "",
			shouldError:  true,
		},
		{
			name: "verify with Fix - mismatch corrected",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				RepoOwner:               "actions",
				RepoName:                "checkout",
				Version:                 "ee0669bd1cc54295c223e0bb666b733df41de1c5",
				VersionComment:          "v3.5.1",
				VersionCommentSeparator: " # ",
			},
			isVerify:     true,
			fix:          true,
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "  - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0",
			shouldError:  false,
		},
		{
			name: "verify with match - no change",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				RepoOwner:               "actions",
				RepoName:                "checkout",
				Version:                 "83b7061638ee4956cf7545a6f7efe594e5ad0247",
				VersionComment:          "v3.5.1",
				VersionCommentSeparator: " # ",
			},
			isVerify:     true,
			fix:          true,
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "",
			shouldError:  false,
		},
	}

	logger := slog.New(slog.DiscardHandler)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()

			var mockService *github.RepositoriesServiceImpl
			if tt.isVerify {
				mockService = &github.RepositoriesServiceImpl{
					Commits: map[string]*github.GetCommitSHA1Result{
						tt.action.RepoOwner + "/" + tt.action.RepoName + "/" + tt.action.VersionComment: {
							SHA: tt.expectedSHA,
						},
					},
					Tags: map[string]*github.ListTagsResult{
						tt.action.RepoOwner + "/" + tt.action.RepoName + "/0": {
							Tags: []*github.RepositoryTag{
								{
									Name: new("v2.7.0"),
									Commit: &github.Commit{
										SHA: new("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
									},
								},
								{
									Name: new("v3.5.1"),
									Commit: &github.Commit{
										SHA: new("83b7061638ee4956cf7545a6f7efe594e5ad0247"),
									},
								},
							},
							Response: &github.Response{},
						},
					},
				}
			}

			ctrl := New(mockService, nil, fs, &config.Config{
				Separator: " # ",
			}, &ParamRun{
				IsVerify: tt.isVerify,
				Fix:      tt.fix,
				Stderr:   &bytes.Buffer{},
			})

			line, err := ctrl.verifyIfNeeded(context.Background(), logger, tt.action)

			if tt.shouldError && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if line != tt.expectedLine {
				t.Fatalf("expected line %q but got %q", tt.expectedLine, line)
			}
		})
	}
}

// newVerifyMockService creates a mock RepositoriesServiceImpl with tag/SHA mappings
// for verify integration tests.
func newVerifyMockService() *github.RepositoriesServiceImpl {
	return &github.RepositoriesServiceImpl{
		Commits: map[string]*github.GetCommitSHA1Result{
			"actions/checkout/v3.5.1": {
				SHA: "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			},
			"actions/checkout/v2.7.0": {
				SHA: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
			},
		},
		Tags: map[string]*github.ListTagsResult{
			"actions/checkout/0": {
				Tags: []*github.RepositoryTag{
					{
						Name: new("v2.7.0"),
						Commit: &github.Commit{
							SHA: new("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
						},
					},
					{
						Name: new("v3.5.1"),
						Commit: &github.Commit{
							SHA: new("83b7061638ee4956cf7545a6f7efe594e5ad0247"),
						},
					},
				},
				Response: &github.Response{},
			},
		},
		Releases: map[string]*github.ListReleasesResult{
			"actions/checkout/0": {
				Releases: []*github.RepositoryRelease{},
				Response: &github.Response{},
			},
		},
	}
}

func TestController_processLines_verifyOnly(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Fix:      false,
		Stderr:   &bytes.Buffer{},
	})

	lines := []string{
		"    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1",
	}
	logger := slog.New(slog.DiscardHandler)
	changed, exitCode := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if changed {
		t.Error("--verify without Fix should not change lines")
	}
	if exitCode == ExitCodeOK {
		t.Error("--verify without Fix should report a non-zero exit code on mismatch")
	}
	if lines[0] != "    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1" {
		t.Errorf("line should be unchanged, got %q", lines[0])
	}
}

func TestController_processLines_verifyWithFix(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Fix:      true,
		Stderr:   &bytes.Buffer{},
	})

	lines := []string{
		"    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1",
	}
	logger := slog.New(slog.DiscardHandler)
	changed, exitCode := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if !changed {
		t.Error("--verify --fix should detect changes")
	}
	if exitCode != ExitCodeOK {
		t.Errorf("--verify --fix should exit OK, got %d", exitCode)
	}
	want := "    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0"
	if lines[0] != want {
		t.Errorf("expected %q, got %q", want, lines[0])
	}
}

func TestController_processLines_verifyMatchingNoChange(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Fix:      true,
		Stderr:   &bytes.Buffer{},
	})

	lines := []string{
		"    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1",
	}
	logger := slog.New(slog.DiscardHandler)
	changed, exitCode := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if changed {
		t.Error("matching SHA/comment should not produce changes")
	}
	if exitCode != ExitCodeOK {
		t.Errorf("matching SHA/comment should not fail, got exitCode %d", exitCode)
	}
}

func TestController_processLines_verifyMultipleLines(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Fix:      true,
		Stderr:   &bytes.Buffer{},
	})

	lines := []string{
		"name: test",
		"    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1",
		"    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1",
		"    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v2.7.0",
	}
	logger := slog.New(slog.DiscardHandler)
	changed, exitCode := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if !changed {
		t.Error("should detect changes for mismatched lines")
	}
	if exitCode != ExitCodeOK {
		t.Errorf("--fix should not set a non-zero exit code, got %d", exitCode)
	}

	if lines[0] != "name: test" {
		t.Errorf("non-action line should be unchanged, got %q", lines[0])
	}
	if lines[1] != "    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1" {
		t.Errorf("matching line should be unchanged, got %q", lines[1])
	}
	want2 := "    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0"
	if lines[2] != want2 {
		t.Errorf("expected %q, got %q", want2, lines[2])
	}
	want3 := "    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1"
	if lines[3] != want3 {
		t.Errorf("expected %q, got %q", want3, lines[3])
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return lines
}

func TestController_processLines_verifyFixTestdata(t *testing.T) {
	t.Parallel()
	input := readLines(t, "../../../testdata/verify.yaml")
	expected := readLines(t, "../../../testdata/verify_after.yaml")

	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Fix:      true,
		Stderr:   &bytes.Buffer{},
	})

	logger := slog.New(slog.DiscardHandler)
	changed, exitCode := ctrl.processLines(context.Background(), logger, "verify.yaml", input)

	if !changed {
		t.Error("expected changes")
	}
	if exitCode != ExitCodeOK {
		t.Errorf("--fix should not set a non-zero exit code, got %d", exitCode)
	}

	got := strings.Join(input, "\n")
	want := strings.Join(expected, "\n")
	if got != want {
		t.Errorf("output mismatch:\ngot:\n%s\n\nwant:\n%s", got, want)
	}
}
