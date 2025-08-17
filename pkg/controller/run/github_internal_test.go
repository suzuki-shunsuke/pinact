package run

import (
	"testing"

	"github.com/hashicorp/go-version"
)

func Test_compare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		latestSemver      *version.Version
		latestVersion     string
		tag               string
		wantSemver        string
		wantLatestVersion string
		wantErr           bool
	}{
		{
			name:              "new semver is greater than current semver",
			latestSemver:      version.Must(version.NewVersion("1.0.0")),
			latestVersion:     "",
			tag:               "2.0.0",
			wantSemver:        "2.0.0",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "new semver is less than current semver",
			latestSemver:      version.Must(version.NewVersion("2.0.0")),
			latestVersion:     "",
			tag:               "1.0.0",
			wantSemver:        "2.0.0",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "new semver equals current semver",
			latestSemver:      version.Must(version.NewVersion("1.0.0")),
			latestVersion:     "",
			tag:               "1.0.0",
			wantSemver:        "1.0.0",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "first semver with nil latest",
			latestSemver:      nil,
			latestVersion:     "",
			tag:               "1.2.3",
			wantSemver:        "1.2.3",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "semver with v prefix",
			latestSemver:      nil,
			latestVersion:     "",
			tag:               "v1.2.3",
			wantSemver:        "v1.2.3",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "invalid semver with greater string comparison",
			latestSemver:      nil,
			latestVersion:     "main",
			tag:               "release",
			wantSemver:        "",
			wantLatestVersion: "release",
			wantErr:           true,
		},
		{
			name:              "invalid semver with lesser string comparison",
			latestSemver:      nil,
			latestVersion:     "release",
			tag:               "main",
			wantSemver:        "",
			wantLatestVersion: "release",
			wantErr:           true,
		},
		{
			name:              "invalid semver as first tag",
			latestSemver:      nil,
			latestVersion:     "",
			tag:               "not-a-version",
			wantSemver:        "",
			wantLatestVersion: "not-a-version",
			wantErr:           true,
		},
		{
			name:              "invalid tag with existing semver",
			latestSemver:      version.Must(version.NewVersion("1.0.0")),
			latestVersion:     "",
			tag:               "invalid",
			wantSemver:        "1.0.0",
			wantLatestVersion: "invalid",
			wantErr:           true,
		},
		{
			name:              "compare with prerelease versions",
			latestSemver:      version.Must(version.NewVersion("1.0.0-alpha")),
			latestVersion:     "",
			tag:               "1.0.0",
			wantSemver:        "1.0.0",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "compare with build metadata",
			latestSemver:      version.Must(version.NewVersion("1.0.0+build.1")),
			latestVersion:     "",
			tag:               "1.0.0+build.2",
			wantSemver:        "1.0.0+build.1",
			wantLatestVersion: "",
			wantErr:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotSemver, gotLatestVersion, err := compare(tt.latestSemver, tt.latestVersion, tt.tag)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("compare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// Check semver result
			if tt.wantSemver == "" {
				if gotSemver != nil {
					t.Errorf("compare() gotSemver = %v, want nil", gotSemver)
				}
			} else {
				if gotSemver == nil {
					t.Errorf("compare() gotSemver = nil, want %v", tt.wantSemver)
				} else if gotSemver.Original() != tt.wantSemver {
					t.Errorf("compare() gotSemver = %v, want %v", gotSemver.Original(), tt.wantSemver)
				}
			}
			
			// Check latest version string result
			if gotLatestVersion != tt.wantLatestVersion {
				t.Errorf("compare() gotLatestVersion = %v, want %v", gotLatestVersion, tt.wantLatestVersion)
			}
		})
	}
}