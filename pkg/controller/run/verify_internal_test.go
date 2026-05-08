package run

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
)

func TestController_verify(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name         string
		action       *Action
		expectedSHA  string
		expectedLine string
		shouldError  bool
		check        bool
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
		},
		{
			name: "mismatched SHA with --fix - comment corrected",
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
			name: "mismatched SHA with --check - comment corrected",
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
			expectedLine: "  - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0",
			shouldError:  false,
			check:        true,
		},
		{
			name: "mismatched SHA without --check or --fix - error",
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
		},
		{
			name: "mismatched SHA with --fix but no matching tag - error",
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

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

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
								Name: strP("v2.7.0"),
								Commit: &github.Commit{
									SHA: strP("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
								},
							},
							{
								Name: strP("v3.5.1"),
								Commit: &github.Commit{
									SHA: strP("83b7061638ee4956cf7545a6f7efe594e5ad0247"),
								},
							},
						},
						Response: &github.Response{},
					},
				},
			}

			ctrl := New(mockService, nil, nil, fs, &config.Config{
				Separator: " # ",
			}, &ParamRun{
				Check: tt.check,
				Fix:   tt.fix,
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
			name: "verify only - mismatch errors",
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
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "",
			shouldError:  true,
		},
		{
			name: "verify with --fix - mismatch corrected",
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
			expectedSHA:  "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			expectedLine: "",
			shouldError:  false,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

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
									Name: strP("v2.7.0"),
									Commit: &github.Commit{
										SHA: strP("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
									},
								},
								{
									Name: strP("v3.5.1"),
									Commit: &github.Commit{
										SHA: strP("83b7061638ee4956cf7545a6f7efe594e5ad0247"),
									},
								},
							},
							Response: &github.Response{},
						},
					},
				}
			}

			ctrl := New(mockService, nil, nil, fs, &config.Config{
				Separator: " # ",
			}, &ParamRun{
				IsVerify: tt.isVerify,
				Fix:      tt.fix,
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
			// v3.5.1 resolves to this SHA
			"actions/checkout/v3.5.1": {
				SHA: "83b7061638ee4956cf7545a6f7efe594e5ad0247",
			},
			// v2.7.0 resolves to this SHA
			"actions/checkout/v2.7.0": {
				SHA: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
			},
		},
		Tags: map[string]*github.ListTagsResult{
			"actions/checkout/0": {
				Tags: []*github.RepositoryTag{
					{
						Name: strP("v2.7.0"),
						Commit: &github.Commit{
							SHA: strP("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
						},
					},
					{
						Name: strP("v3.5.1"),
						Commit: &github.Commit{
							SHA: strP("83b7061638ee4956cf7545a6f7efe594e5ad0247"),
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
	ctrl := New(newVerifyMockService(), nil, nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Check:    false,
		Fix:      false,
		Stderr:   &bytes.Buffer{},
	})

	lines := []string{
		"    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	changed, failed := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if changed {
		t.Error("--verify alone should not change lines")
	}
	if !failed {
		t.Error("--verify alone should report failure on mismatch")
	}
	// Line should remain unchanged
	if lines[0] != "    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1" {
		t.Errorf("line should be unchanged, got %q", lines[0])
	}
}

func TestController_processLines_verifyWithCheck(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Check:    true,
		Fix:      false,
		Stderr:   &bytes.Buffer{},
	})

	lines := []string{
		"    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	changed, failed := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if !changed {
		t.Error("--verify --check should detect changes")
	}
	if !failed {
		t.Error("--verify --check should set failed (exit non-zero)")
	}
	// SHA should be kept, comment should be corrected
	want := "    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0"
	if lines[0] != want {
		t.Errorf("expected %q, got %q", want, lines[0])
	}
}

func TestController_processLines_verifyWithFix(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Check:    false,
		Fix:      true,
		Stderr:   &bytes.Buffer{},
	})

	lines := []string{
		"    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	changed, failed := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if !changed {
		t.Error("--verify --fix should detect changes")
	}
	if failed {
		t.Error("--verify --fix should not set failed")
	}
	// SHA should be kept, comment should be corrected
	want := "    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0"
	if lines[0] != want {
		t.Errorf("expected %q, got %q", want, lines[0])
	}
}

func TestController_processLines_verifyMatchingNoChange(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Check:    true,
		Fix:      true,
		Stderr:   &bytes.Buffer{},
	})

	// SHA matches v3.5.1 - no correction needed
	lines := []string{
		"    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	changed, failed := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if changed {
		t.Error("matching SHA/comment should not produce changes")
	}
	if failed {
		t.Error("matching SHA/comment should not fail")
	}
}

func TestController_processLines_verifyMultipleLines(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := New(newVerifyMockService(), nil, nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Check:    false,
		Fix:      true,
		Stderr:   &bytes.Buffer{},
	})

	lines := []string{
		"name: test",
		"    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1",
		"    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.1",
		"    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v2.7.0",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	changed, failed := ctrl.processLines(context.Background(), logger, "test.yml", lines)

	if !changed {
		t.Error("should detect changes for mismatched lines")
	}
	if failed {
		t.Error("--fix should not set failed")
	}

	// Line 0: non-action line, unchanged
	if lines[0] != "name: test" {
		t.Errorf("non-action line should be unchanged, got %q", lines[0])
	}
	// Line 1: correct match, unchanged
	if lines[1] != "    - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # v3.5.1" {
		t.Errorf("matching line should be unchanged, got %q", lines[1])
	}
	// Line 2: SHA is v2.7.0 but comment says v3.5.1 → comment corrected to v2.7.0
	want2 := "    - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0"
	if lines[2] != want2 {
		t.Errorf("expected %q, got %q", want2, lines[2])
	}
	// Line 3: SHA is v3.5.1 but comment says v2.7.0 → comment corrected to v3.5.1
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
	ctrl := New(newVerifyMockService(), nil, nil, fs, &config.Config{
		Separator: " # ",
	}, &ParamRun{
		IsVerify: true,
		Fix:      true,
		Stderr:   &bytes.Buffer{},
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	changed, failed := ctrl.processLines(context.Background(), logger, "verify.yaml", input)

	if !changed {
		t.Error("expected changes")
	}
	if failed {
		t.Error("--fix should not set failed")
	}

	got := strings.Join(input, "\n")
	want := strings.Join(expected, "\n")
	if got != want {
		t.Errorf("output mismatch:\ngot:\n%s\n\nwant:\n%s", got, want)
	}
}
