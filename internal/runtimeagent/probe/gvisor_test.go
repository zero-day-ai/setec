/*
Copyright 2026 The Setec Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//go:build linux

package probe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeLookPath returns a LookPath function whose behaviour is controlled by
// the found flag. When found=true it returns a synthetic binary path;
// otherwise it returns an error that mimics exec.ErrNotFound.
func fakeLookPath(found bool) func(string) (string, error) {
	return func(file string) (string, error) {
		if found {
			return "/usr/local/bin/" + file, nil
		}
		return "", errors.New(file + ": executable file not found in $PATH")
	}
}

// TestGVisorProbe exercises the gvisor probe against injected LookPath
// functions and fake containerd config files.
func TestGVisorProbe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		lookPathFound    bool
		containerdConfig string // content of config.toml; empty = don't create it
		wantAvailable    bool
		wantReason       string // substring required in Reason when set
		wantDetailKey    string // key expected in Details when available
	}{
		{
			name:          "runsc not in PATH",
			lookPathFound: false,
			wantAvailable: false,
			wantReason:    "runsc binary not found in PATH",
		},
		{
			name:             "runsc found, containerd config references runsc",
			lookPathFound:    true,
			containerdConfig: `[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runsc]\nruntime_type = "io.containerd.runsc.v1"`,
			wantAvailable:    true,
			wantDetailKey:    "runsc",
		},
		{
			name:             "runsc found, containerd config present but no runsc entry",
			lookPathFound:    true,
			containerdConfig: `[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]`,
			wantAvailable:    true,
			// The probe falls back to best-effort and logs a warning.
			wantReason:    "containerd config not accessible",
			wantDetailKey: "containerd_check",
		},
		{
			name:          "runsc found, no containerd config mounted",
			lookPathFound: true,
			wantAvailable: true,
			// Best-effort: passes on binary presence, surfaces skipped note.
			wantReason:    "containerd config not accessible",
			wantDetailKey: "containerd_check",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if tc.containerdConfig != "" {
				dir := filepath.Join(root, "etc", "containerd")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(
					filepath.Join(dir, "config.toml"),
					[]byte(tc.containerdConfig),
					0o644,
				); err != nil {
					t.Fatalf("write config: %v", err)
				}
			}

			p := newGVisorProbe(Config{
				FSRoot:   root,
				LookPath: fakeLookPath(tc.lookPathFound),
			})

			got := p.Check(context.Background())

			if got.Available != tc.wantAvailable {
				t.Errorf("Available = %v, want %v (Reason=%q)", got.Available, tc.wantAvailable, got.Reason)
			}
			if tc.wantReason != "" && !strings.Contains(got.Reason, tc.wantReason) {
				t.Errorf("Reason = %q, want substring %q", got.Reason, tc.wantReason)
			}
			if tc.wantAvailable && tc.wantDetailKey != "" {
				if _, ok := got.Details[tc.wantDetailKey]; !ok {
					t.Errorf("Details missing key %q; got %v", tc.wantDetailKey, got.Details)
				}
			}
			if p.Name() != "gvisor" {
				t.Errorf("Name() = %q, want %q", p.Name(), "gvisor")
			}
		})
	}
}
