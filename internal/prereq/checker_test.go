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

package prereq

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// testRuntimeClass is the RuntimeClass name used by every test case. It has
// no semantic meaning beyond giving the fake client an object to match (or
// not match) against.
const (
	testRuntimeClass = "kata-fc"
	testNodeLabel    = "katacontainers.io/kata-runtime"
)

// newScheme builds a runtime.Scheme pre-populated with the two API groups
// the prereq checker reads. The fake client needs these registered so it
// can dispatch Get/List calls on typed objects.
func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("register corev1: %v", err)
	}
	if err := nodev1.AddToScheme(s); err != nil {
		t.Fatalf("register nodev1: %v", err)
	}
	return s
}

// runtimeClassObj is a convenience constructor for a minimal RuntimeClass
// carrying the required Handler field.
func runtimeClassObj(name string) *nodev1.RuntimeClass {
	return &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Handler:    "kata-fc",
	}
}

// nodeObj is a convenience constructor for a Node with the given labels.
// A nil labels map means "no labels at all" so zero-match tests remain
// unambiguous.
func nodeObj(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

// TestCheck exercises every combination of RuntimeClass presence and
// Kata-capable Node presence. The fake controller-runtime client is
// sufficient because Check performs only Get + List calls, both of which
// the fake implements faithfully.
func TestCheck(t *testing.T) {
	tests := []struct {
		name                  string
		seed                  []client.Object
		wantRuntimeClass      bool
		wantKataCapableNodes  bool
		wantWarningsCount     int
		wantWarningSubstrings []string // substrings that must appear, in order
	}{
		{
			name: "a) RuntimeClass present and matching Nodes",
			seed: []client.Object{
				runtimeClassObj(testRuntimeClass),
				nodeObj("node-kata", map[string]string{testNodeLabel: "true"}),
				nodeObj("node-plain", nil),
			},
			wantRuntimeClass:     true,
			wantKataCapableNodes: true,
			wantWarningsCount:    0,
		},
		{
			name: "b) RuntimeClass present, zero matching Nodes",
			seed: []client.Object{
				runtimeClassObj(testRuntimeClass),
				nodeObj("node-plain", map[string]string{"kubernetes.io/hostname": "node-plain"}),
			},
			wantRuntimeClass:     true,
			wantKataCapableNodes: false,
			wantWarningsCount:    1,
			wantWarningSubstrings: []string{
				testNodeLabel,
			},
		},
		{
			name: "c) RuntimeClass absent, matching Nodes",
			seed: []client.Object{
				nodeObj("node-kata", map[string]string{testNodeLabel: "true"}),
			},
			wantRuntimeClass:     false,
			wantKataCapableNodes: true,
			wantWarningsCount:    1,
			wantWarningSubstrings: []string{
				testRuntimeClass,
			},
		},
		{
			name:                 "d) both absent",
			seed:                 []client.Object{},
			wantRuntimeClass:     false,
			wantKataCapableNodes: false,
			wantWarningsCount:    2,
			wantWarningSubstrings: []string{
				testRuntimeClass,
				testNodeLabel,
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := fake.NewClientBuilder().
				WithScheme(newScheme(t)).
				WithObjects(tc.seed...).
				Build()

			got, err := Check(context.Background(), c, testRuntimeClass, testNodeLabel)
			if err != nil {
				t.Fatalf("Check returned unexpected error: %v", err)
			}

			if got.RuntimeClassPresent != tc.wantRuntimeClass {
				t.Errorf("RuntimeClassPresent = %v, want %v",
					got.RuntimeClassPresent, tc.wantRuntimeClass)
			}
			if got.KataCapableNodes != tc.wantKataCapableNodes {
				t.Errorf("KataCapableNodes = %v, want %v",
					got.KataCapableNodes, tc.wantKataCapableNodes)
			}
			if len(got.Warnings) != tc.wantWarningsCount {
				t.Errorf("len(Warnings) = %d, want %d; warnings=%q",
					len(got.Warnings), tc.wantWarningsCount, got.Warnings)
			}

			// When the expected order is specified, verify each warning
			// contains the expected substring at its position. This keeps
			// the assertion tolerant of copywriting changes while still
			// pinning the stable (RuntimeClass, Nodes) ordering.
			if tc.wantWarningSubstrings != nil {
				if diff := cmp.Diff(
					len(tc.wantWarningSubstrings),
					len(got.Warnings),
				); diff != "" {
					t.Fatalf("warnings count mismatch (-want +got):\n%s", diff)
				}
				for i, sub := range tc.wantWarningSubstrings {
					if !strings.Contains(got.Warnings[i], sub) {
						t.Errorf("Warnings[%d] = %q, want substring %q",
							i, got.Warnings[i], sub)
					}
				}
			}
		})
	}
}

// TestCheck_RuntimeClassGetError verifies that a non-NotFound error from the
// RuntimeClass Get call is wrapped and surfaced to the caller rather than
// silently swallowed. An internal-server-error-class response must stop the
// check instead of reporting a false "RuntimeClass absent" signal.
func TestCheck_RuntimeClassGetError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom: api server is down")
	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				_ context.Context,
				_ client.WithWatch,
				_ client.ObjectKey,
				_ client.Object,
				_ ...client.GetOption,
			) error {
				return sentinel
			},
		}).
		Build()

	got, err := Check(context.Background(), c, testRuntimeClass, testNodeLabel)
	if err == nil {
		t.Fatalf("expected error, got nil (result=%+v)", got)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
	if !strings.Contains(err.Error(), "get RuntimeClass") {
		t.Errorf("expected wrap message to mention RuntimeClass Get, got %q", err.Error())
	}
	// The Node check must not have run yet, so the flag should be its zero
	// value and no Warnings should have been appended.
	if got.KataCapableNodes {
		t.Errorf("KataCapableNodes should be false when Get errored, got %v", got.KataCapableNodes)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("Warnings should be empty when Get errored, got %q", got.Warnings)
	}
}

// TestCheck_NodeListError verifies that an error from the Node List call is
// wrapped and surfaced to the caller. The RuntimeClass result accumulated
// before the failure must still be present on the returned CheckResult so
// callers can distinguish a partial success from a total failure.
func TestCheck_NodeListError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom: list failed")
	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithObjects(runtimeClassObj(testRuntimeClass)).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(
				_ context.Context,
				_ client.WithWatch,
				_ client.ObjectList,
				_ ...client.ListOption,
			) error {
				return sentinel
			},
		}).
		Build()

	got, err := Check(context.Background(), c, testRuntimeClass, testNodeLabel)
	if err == nil {
		t.Fatalf("expected error, got nil (result=%+v)", got)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
	if !strings.Contains(err.Error(), "list Nodes") {
		t.Errorf("expected wrap message to mention Node List, got %q", err.Error())
	}
	// RuntimeClass was found before List failed; that partial progress must
	// survive onto the returned CheckResult.
	if !got.RuntimeClassPresent {
		t.Errorf("RuntimeClassPresent should be true even when List errored, got %v", got.RuntimeClassPresent)
	}
}
