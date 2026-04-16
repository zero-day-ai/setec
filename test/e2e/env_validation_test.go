//go:build e2e

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

package e2e

import (
	"errors"
	"os"
	"testing"
)

// TestEnv_KVMPresent is the loud-fail environment guard for the Phase 3
// suite. Every Phase 3 scenario implicitly assumes /dev/kvm is present
// because Kata Containers with Firecracker requires hardware
// virtualisation. Without this guard, a CI host missing /dev/kvm would
// cause the Phase 3 scenarios to all hit t.Skip() and the whole suite
// would report PASS with zero meaningful coverage — a silent regression
// hiding underneath green CI.
//
// Run this with `go test -tags=e2e ./test/e2e -run TestEnv_KVMPresent`
// before the full Phase 3 suite to confirm the environment is viable.
func TestEnv_KVMPresent(t *testing.T) {
	if _, err := os.Stat("/dev/kvm"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatal("FATAL: /dev/kvm is missing; Phase 3 cannot run. " +
				"Install KVM modules (kvm_intel or kvm_amd) or run the suite " +
				"on a bare-metal host. Do NOT bypass this check.")
		}
		t.Fatalf("stat /dev/kvm: %v", err)
	}
}
