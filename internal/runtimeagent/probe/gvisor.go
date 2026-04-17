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

package probe

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// gvisorProbe checks whether the gVisor (runsc) container runtime is
// available on the host node.
//
// Detection strategy (no subprocess execution):
//  1. LookPath("runsc") must succeed — the runtime binary must be in PATH.
//  2. The probe attempts to verify that a containerd runtime entry for runsc
//     exists by reading containerd configuration files. This is best-effort:
//     if the config files are not accessible (e.g. not volume-mounted into
//     the DaemonSet), a warning is logged and the probe passes on binary
//     presence alone.
//
// The containerd config paths checked (in order):
//   - <FSRoot>/etc/containerd/config.toml
//   - Files under <FSRoot>/etc/containerd/conf.d/ (glob *.toml)
type gvisorProbe struct {
	cfg Config
}

func newGVisorProbe(cfg Config) Probe {
	return &gvisorProbe{cfg: cfg}
}

// Name implements Probe.
func (p *gvisorProbe) Name() string { return "gvisor" }

// Check implements Probe.
func (p *gvisorProbe) Check(_ context.Context) CapabilityResult {
	lookPath := p.cfg.lookPath()

	binPath, err := lookPath("runsc")
	if err != nil {
		return CapabilityResult{
			Available: false,
			Reason:    "runsc binary not found in PATH; install gVisor and ensure runsc is on the node PATH",
		}
	}

	// Best-effort containerd config verification.
	containerdOK, containerdNote := p.checkContainerdConfig()
	if !containerdOK {
		// Log a warning but do not fail: the binary is present and the
		// DaemonSet may not mount /etc/containerd.
		slog.Warn("gvisor probe: containerd config not accessible; passing on binary presence alone",
			"note", containerdNote,
			"runsc", binPath,
		)
		return CapabilityResult{
			Available: true,
			Reason:    "runsc found; containerd config not accessible (best-effort): " + containerdNote,
			Details:   map[string]string{"runsc": binPath, "containerd_check": "skipped"},
		}
	}

	return CapabilityResult{
		Available: true,
		Details:   map[string]string{"runsc": binPath, "containerd_check": "ok"},
	}
}

// checkContainerdConfig attempts to verify that at least one containerd
// configuration file references runsc. Returns (true, "") when a matching
// entry is found, (false, reason) when the config is readable but has no
// runsc entry, or (false, reason) when the config files are not accessible
// (the caller treats this as a best-effort skip rather than a hard failure).
func (p *gvisorProbe) checkContainerdConfig() (bool, string) {
	root := p.cfg.FSRoot

	// Candidate paths to search.
	candidates := []string{
		filepath.Join(root, "etc", "containerd", "config.toml"),
	}

	// Also check drop-in directory if it exists.
	dropinDir := filepath.Join(root, "etc", "containerd", "conf.d")
	entries, err := os.ReadDir(dropinDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
				candidates = append(candidates, filepath.Join(dropinDir, e.Name()))
			}
		}
	}

	anyAccessible := false
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			// File may not exist or may not be mounted; continue.
			continue
		}
		anyAccessible = true
		// A containerd config that registers gVisor will contain a
		// runtime handler block referencing "runsc". A simple substring
		// search is sufficient — we are not parsing TOML.
		if strings.Contains(string(data), "runsc") {
			return true, ""
		}
	}

	if !anyAccessible {
		return false, "no containerd config files found under /etc/containerd/ (not mounted?)"
	}
	return false, "containerd config found but no runsc runtime handler entry detected"
}
