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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VMM identifies a virtual machine monitor a SandboxClass may target. The
// enum matches the set of VMMs Kata Containers currently ships support for;
// the operator itself does not embed VMM-specific logic — the value is
// surfaced to administrators as an explicit capability declaration.
// +kubebuilder:validation:Enum=firecracker;qemu;cloud-hypervisor
type VMM string

const (
	// VMMFirecracker selects the Firecracker VMM (Kata runtime kata-fc).
	VMMFirecracker VMM = "firecracker"
	// VMMQEMU selects the QEMU VMM (Kata runtime kata-qemu).
	VMMQEMU VMM = "qemu"
	// VMMCloudHypervisor selects Cloud Hypervisor (Kata runtime kata-clh).
	VMMCloudHypervisor VMM = "cloud-hypervisor"
)

// SandboxClassSpec defines the constraints and defaults a cluster
// administrator publishes for tenant-facing Sandboxes. Tenants reference a
// SandboxClass by name in Sandbox.spec.sandboxClassName (added to
// SandboxSpec in a later task) and the operator enforces that the requested
// Sandbox fits within the class.
type SandboxClassSpec struct {
	// VMM selects the virtual machine monitor targeted by this class.
	// +required
	VMM VMM `json:"vmm"`

	// RuntimeClassName optionally overrides the operator-wide default
	// RuntimeClass name (e.g. "kata-fc", "kata-qemu"). When empty the
	// controller falls back to its --runtime-class-name flag.
	// +optional
	RuntimeClassName string `json:"runtimeClassName,omitempty"`

	// KernelImage is an optional OCI reference to a custom guest kernel
	// image the node agent pre-pulls and hands to Kata. Empty means the
	// Kata-packaged kernel for the selected VMM is used.
	// +optional
	KernelImage string `json:"kernelImage,omitempty"`

	// RootfsImage is an optional OCI reference to a custom guest rootfs
	// image. Empty means the Kata-packaged rootfs is used.
	// +optional
	RootfsImage string `json:"rootfsImage,omitempty"`

	// DefaultResources is the resource budget applied to Sandboxes that do
	// not specify their own. Optional; when nil the Sandbox must declare
	// its own resources explicitly.
	// +optional
	DefaultResources *Resources `json:"defaultResources,omitempty"`

	// MaxResources is the upper bound tenant Sandboxes may request. The
	// validating admission webhook rejects any Sandbox requesting more
	// than these values. Optional; when nil the class imposes no ceiling
	// beyond whatever ResourceQuota the tenant namespace enforces.
	// +optional
	MaxResources *Resources `json:"maxResources,omitempty"`

	// AllowedNetworkModes enumerates the Sandbox.network.mode values
	// tenants may request under this class. Empty list means all modes
	// are allowed (back-compat: Phase 1 behavior).
	// +optional
	AllowedNetworkModes []NetworkMode `json:"allowedNetworkModes,omitempty"`

	// NodeSelector is injected into every Sandbox Pod produced under this
	// class. It is additive to any Pod-level selectors the controller sets
	// for RuntimeClass affinity.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Default marks this SandboxClass as the cluster-wide default. Only
	// one SandboxClass may carry this flag set to true; multiple defaults
	// produce a startup warning and cause the resolver to reject
	// defaulting until the ambiguity is resolved.
	// +optional
	Default bool `json:"default,omitempty"`

	// PreWarmPoolSize declares how many paused microVMs the node-agent
	// maintains per eligible node for this class. Zero disables the
	// pool (Phase 1/2 behavior). When non-zero PreWarmImage MUST be
	// set — the webhook enforces the pairing.
	// +kubebuilder:validation:Minimum=0
	// +optional
	PreWarmPoolSize int32 `json:"preWarmPoolSize,omitempty"`

	// PreWarmImage is the OCI reference baked into pre-warmed pool
	// entries. Sandboxes requesting a different image fall through to
	// the cold-boot path. The format follows the usual OCI reference
	// grammar; validation beyond non-empty is a webhook concern so the
	// CRD schema remains minimal.
	// +optional
	PreWarmImage string `json:"preWarmImage,omitempty"`

	// PreWarmTTL bounds the age of pool entries. Entries older than
	// this are recycled (torn down and reprovisioned) to avoid stale
	// kernel state accumulating in paused VMs. When unset the
	// node-agent defaults to 24h at runtime.
	// +optional
	PreWarmTTL *metav1.Duration `json:"preWarmTTL,omitempty"`

	// MaxPauseDuration bounds how long a Sandbox may remain in
	// phase=Paused. Beyond this the reconciler transitions the
	// Sandbox to Failed with reason=PauseTimeoutExceeded. When unset
	// pauses are unbounded.
	// +optional
	MaxPauseDuration *metav1.Duration `json:"maxPauseDuration,omitempty"`
}

// SandboxClassStatus reflects the observed state of a SandboxClass. Phase 2
// does not compute any status fields — the struct exists so future phases
// can record counts, validation summaries, or image-prefetch state without
// breaking the CRD schema.
type SandboxClassStatus struct {
	// ObservedGeneration is the .metadata.generation the operator last
	// reconciled. Optional; left empty in Phase 2.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=sbxcls
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="VMM",type=string,JSONPath=`.spec.vmm`
// +kubebuilder:printcolumn:name="Default",type=boolean,JSONPath=`.spec.default`
// +kubebuilder:printcolumn:name="Max-VCPU",type=integer,JSONPath=`.spec.maxResources.vcpu`,priority=1
// +kubebuilder:printcolumn:name="Max-Memory",type=string,JSONPath=`.spec.maxResources.memory`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SandboxClass is a cluster-scoped, administrator-authored resource that
// publishes a named, pre-approved sandbox configuration. Tenant users
// reference a SandboxClass by name in their Sandbox manifests; the
// operator's validating webhook enforces that the Sandbox fits within the
// class's constraints.
type SandboxClass struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard Kubernetes object metadata. SandboxClass is
	// cluster-scoped so namespace is ignored.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the constraints and defaults of the class.
	// +required
	Spec SandboxClassSpec `json:"spec"`

	// status reflects the observed state of the class.
	// +optional
	Status SandboxClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SandboxClassList is a list of SandboxClass resources.
type SandboxClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SandboxClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SandboxClass{}, &SandboxClassList{})
}
