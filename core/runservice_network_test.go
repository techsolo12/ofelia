// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"testing"
)

// TestRunServiceJob_BuildService_NetworkWired verifies that setting Network on
// RunServiceJob results in the network appearing in the service spec passed to
// the Docker API via CreateService.
func TestRunServiceJob_BuildService_NetworkWired(t *testing.T) {
	t.Parallel()
	k := newTestRunServiceKit(t)
	k.job.Network = "overlay-cluster"

	svcID, err := k.job.buildService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svcID == "" {
		t.Fatal("expected non-empty service ID")
	}

	if len(k.services.CreateCalls) == 0 {
		t.Fatal("expected Create to be called")
	}
	spec := k.services.CreateCalls[0].Spec

	// The network must be reachable in the domain spec. buildService() sets
	// TaskTemplate.Networks, and the adapter converter must read it.
	allNets := append(spec.Networks, spec.TaskTemplate.Networks...)
	if len(allNets) == 0 {
		t.Fatal("expected at least one network attachment in the service spec")
	}

	found := false
	for _, n := range allNets {
		if n.Target == "overlay-cluster" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected network 'overlay-cluster' in spec, got Networks=%v TaskTemplate.Networks=%v",
			spec.Networks, spec.TaskTemplate.Networks)
	}
}
