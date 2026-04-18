//go:build linux

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
	"os"
	"path/filepath"
)

// KVMAvailable reports whether the KVM device node exists at
// <root>/dev/kvm. Returns (true, "") when available. On failure the
// returned reason is a human-readable explanation suitable for use in
// CapabilityResult.Reason.
//
// No subprocess is executed: the check is a single os.Stat call.
func KVMAvailable(root string) (bool, string) {
	path := filepath.Join(root, "dev", "kvm")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, "KVM device /dev/kvm not found; node may not support hardware virtualisation"
		}
		return false, "KVM device /dev/kvm not accessible: " + err.Error()
	}
	return true, ""
}

// ModuleLoaded reports whether the kernel module directory exists at
// <root>/sys/module/<name>. The sysfs module directory is present for
// every loaded module — its absence means the module is not loaded or
// the kernel does not support sysfs (rare but worth handling gracefully).
//
// No subprocess is executed: the check is a single os.Stat call.
func ModuleLoaded(root, name string) bool {
	path := filepath.Join(root, "sys", "module", name)
	_, err := os.Stat(path)
	return err == nil
}
