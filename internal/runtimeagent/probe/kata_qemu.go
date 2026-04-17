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

import "context"

// kataQEMUProbe checks whether the kata-qemu (Kata Containers + QEMU VMM)
// runtime is available on the host node.
//
// Unlike kata-fc, QEMU supports TCG software emulation in addition to
// hardware-accelerated KVM. The probe behaviour depends on Config.AllowTCG:
//
//   - AllowTCG=false (default): KVM device and a CPU module (kvm_intel or
//     kvm_amd) must both be present. Without KVM the result is Available=false.
//
//   - AllowTCG=true: if KVM is absent the probe still returns Available=true
//     with Details["mode"]="tcg" and a Reason noting the fallback. This is
//     useful for CI environments or developer workstations that intentionally
//     use software emulation, but should be disabled in production.
//
// No binary lookup is performed: kata-qemu availability is determined by
// hardware/kernel state alone.
type kataQEMUProbe struct {
	cfg Config
}

func newKataQEMUProbe(cfg Config) Probe {
	return &kataQEMUProbe{cfg: cfg}
}

// Name implements Probe.
func (p *kataQEMUProbe) Name() string { return "kata-qemu" }

// Check implements Probe.
func (p *kataQEMUProbe) Check(_ context.Context) CapabilityResult {
	kvmOK, kvmReason := KVMAvailable(p.cfg.FSRoot)

	if !kvmOK {
		if p.cfg.AllowTCG {
			// TCG fallback: QEMU can still run in software emulation mode.
			// Surface a reason so the operator knows KVM is absent.
			return CapabilityResult{
				Available: true,
				Reason:    "KVM absent, TCG fallback: " + kvmReason,
				Details:   map[string]string{"mode": "tcg"},
			}
		}
		return CapabilityResult{
			Available: false,
			Reason:    "kata-qemu requires KVM (set AllowTCG to enable software emulation): " + kvmReason,
		}
	}

	intelLoaded := ModuleLoaded(p.cfg.FSRoot, "kvm_intel")
	amdLoaded := ModuleLoaded(p.cfg.FSRoot, "kvm_amd")
	if !intelLoaded && !amdLoaded {
		if p.cfg.AllowTCG {
			return CapabilityResult{
				Available: true,
				Reason:    "KVM absent, TCG fallback: neither kvm_intel nor kvm_amd loaded",
				Details:   map[string]string{"mode": "tcg"},
			}
		}
		return CapabilityResult{
			Available: false,
			Reason: "kata-qemu requires a KVM CPU module: " +
				"neither kvm_intel nor kvm_amd is loaded in /sys/module/",
		}
	}

	mod := "kvm_intel"
	if amdLoaded && !intelLoaded {
		mod = "kvm_amd"
	}
	return CapabilityResult{
		Available: true,
		Details:   map[string]string{"mode": "kvm", "kvm_module": mod},
	}
}
