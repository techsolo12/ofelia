// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package docker

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ofelia/core/domain"
)

// roundTripServiceSpec converts domain->swarm->domain and returns the result.
func roundTripServiceSpec(spec *domain.ServiceSpec) *domain.Service {
	swarmSpec := convertToSwarmSpec(spec)
	svc := &swarm.Service{
		ID:   "roundtrip-test",
		Spec: swarmSpec,
	}

	return convertFromSwarmService(svc)
}

func TestServiceSpec_RoundTrip_ContainerSpec(t *testing.T) {
	t.Parallel()

	original := &domain.ServiceSpec{
		Name:   "container-rt",
		Labels: map[string]string{"app": "test"},
		TaskTemplate: domain.TaskSpec{
			ContainerSpec: domain.ContainerSpec{
				Image:     "alpine:3.20",
				Labels:    map[string]string{"tier": "backend"},
				Command:   []string{"/bin/sh", "-c"},
				Args:      []string{"echo hello"},
				Hostname:  "worker-01",
				Env:       []string{"FOO=bar", "BAZ=qux"},
				Dir:       "/opt/app",
				User:      "appuser",
				TTY:       true,
				OpenStdin: true,
				Mounts: []domain.ServiceMount{
					{Type: domain.MountTypeBind, Source: "/host/data", Target: "/data", ReadOnly: true},
					{Type: domain.MountTypeVolume, Source: "vol1", Target: "/vol1", ReadOnly: false},
				},
			},
		},
	}

	result := roundTripServiceSpec(original)
	require.NotNil(t, result)

	cs := result.Spec.TaskTemplate.ContainerSpec
	assert.Equal(t, "alpine:3.20", cs.Image, "Image should survive round-trip")
	assert.Equal(t, map[string]string{"tier": "backend"}, cs.Labels, "ContainerSpec Labels should survive round-trip")
	assert.Equal(t, []string{"/bin/sh", "-c"}, cs.Command, "Command should survive round-trip")
	assert.Equal(t, []string{"echo hello"}, cs.Args, "Args should survive round-trip")
	assert.Equal(t, "worker-01", cs.Hostname, "Hostname should survive round-trip")
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, cs.Env, "Env should survive round-trip")
	assert.Equal(t, "/opt/app", cs.Dir, "Dir should survive round-trip")
	assert.Equal(t, "appuser", cs.User, "User should survive round-trip")
	assert.True(t, cs.TTY, "TTY should survive round-trip")
	assert.True(t, cs.OpenStdin, "OpenStdin should survive round-trip")

	require.Len(t, cs.Mounts, 2, "Mounts should survive round-trip")
	assert.Equal(t, domain.MountTypeBind, cs.Mounts[0].Type)
	assert.Equal(t, "/host/data", cs.Mounts[0].Source)
	assert.Equal(t, "/data", cs.Mounts[0].Target)
	assert.True(t, cs.Mounts[0].ReadOnly)
	assert.Equal(t, domain.MountTypeVolume, cs.Mounts[1].Type)
	assert.Equal(t, "vol1", cs.Mounts[1].Source)
	assert.Equal(t, "/vol1", cs.Mounts[1].Target)
	assert.False(t, cs.Mounts[1].ReadOnly)

	// Service-level fields
	assert.Equal(t, "container-rt", result.Spec.Name, "Service name should survive round-trip")
	assert.Equal(t, map[string]string{"app": "test"}, result.Spec.Labels, "Service labels should survive round-trip")
}

func TestServiceSpec_RoundTrip_TaskTemplateNetworks(t *testing.T) {
	t.Parallel()

	// This mimics how buildService() populates networks: on TaskTemplate.Networks.
	// convertToSwarmSpec reads from both spec.Networks and spec.TaskTemplate.Networks.
	original := &domain.ServiceSpec{
		Name: "task-net-rt",
		TaskTemplate: domain.TaskSpec{
			ContainerSpec: domain.ContainerSpec{
				Image: "nginx:latest",
			},
			Networks: []domain.NetworkAttachment{
				{Target: "frontend-net", Aliases: []string{"web", "proxy"}},
				{Target: "backend-net", Aliases: []string{"api"}},
			},
		},
	}

	// First, verify the toSwarm direction: TaskTemplate.Networks should appear
	// in the swarm spec's TaskTemplate.Networks.
	swarmSpec := convertToSwarmSpec(original)
	require.Len(t, swarmSpec.TaskTemplate.Networks, 2,
		"convertToSwarmSpec should include TaskTemplate.Networks")
	assert.Equal(t, "frontend-net", swarmSpec.TaskTemplate.Networks[0].Target)
	assert.Equal(t, []string{"web", "proxy"}, swarmSpec.TaskTemplate.Networks[0].Aliases)
	assert.Equal(t, "backend-net", swarmSpec.TaskTemplate.Networks[1].Target)
	assert.Equal(t, []string{"api"}, swarmSpec.TaskTemplate.Networks[1].Aliases)

	// Full round-trip
	result := roundTripServiceSpec(original)
	require.NotNil(t, result)

	// Check that networks survived the full round-trip (either in Networks or TaskTemplate.Networks)
	allNetworks := append(result.Spec.Networks, result.Spec.TaskTemplate.Networks...)
	require.Len(t, allNetworks, 2,
		"Networks set on TaskTemplate should survive the full round-trip")
}

func TestServiceSpec_RoundTrip_ServiceSpecNetworks(t *testing.T) {
	t.Parallel()

	// Networks set on ServiceSpec.Networks -- this is the location convertToSwarmSpec reads from.
	original := &domain.ServiceSpec{
		Name: "spec-net-rt",
		TaskTemplate: domain.TaskSpec{
			ContainerSpec: domain.ContainerSpec{
				Image: "nginx:latest",
			},
		},
		Networks: []domain.NetworkAttachment{
			{Target: "overlay-1", Aliases: []string{"svc-a"}},
			{Target: "overlay-2", Aliases: []string{"svc-b", "svc-c"}},
		},
	}

	// toSwarm direction should work: spec.Networks -> swarmSpec.TaskTemplate.Networks
	swarmSpec := convertToSwarmSpec(original)
	require.Len(t, swarmSpec.TaskTemplate.Networks, 2, "toSwarm should convert spec.Networks")
	assert.Equal(t, "overlay-1", swarmSpec.TaskTemplate.Networks[0].Target)
	assert.Equal(t, "overlay-2", swarmSpec.TaskTemplate.Networks[1].Target)

	// Full round-trip: fromSwarm should convert networks back
	result := roundTripServiceSpec(original)
	require.NotNil(t, result)

	allNetworks := append(result.Spec.Networks, result.Spec.TaskTemplate.Networks...)
	require.Len(t, allNetworks, 2, "Networks should survive round-trip")
	assert.Equal(t, "overlay-1", allNetworks[0].Target)
	assert.Equal(t, []string{"svc-a"}, allNetworks[0].Aliases)
	assert.Equal(t, "overlay-2", allNetworks[1].Target)
	assert.Equal(t, []string{"svc-b", "svc-c"}, allNetworks[1].Aliases)
}

func TestServiceSpec_RoundTrip_RestartPolicy(t *testing.T) {
	t.Parallel()

	original := &domain.ServiceSpec{
		Name: "restart-rt",
		TaskTemplate: domain.TaskSpec{
			ContainerSpec: domain.ContainerSpec{
				Image: "alpine:latest",
			},
			RestartPolicy: &domain.ServiceRestartPolicy{
				Condition:   domain.RestartConditionOnFailure,
				Delay:       durationPtr(10 * time.Second),
				MaxAttempts: uint64Ptr(5),
				Window:      durationPtr(3 * time.Minute),
			},
		},
	}

	// toSwarm direction should work (already tested in unit tests)
	swarmSpec := convertToSwarmSpec(original)
	require.NotNil(t, swarmSpec.TaskTemplate.RestartPolicy, "toSwarm should convert restart policy")
	assert.Equal(t, swarm.RestartPolicyCondition("on-failure"), swarmSpec.TaskTemplate.RestartPolicy.Condition)

	// Full round-trip
	result := roundTripServiceSpec(original)
	require.NotNil(t, result)

	rp := result.Spec.TaskTemplate.RestartPolicy
	require.NotNil(t, rp, "RestartPolicy should survive round-trip")
	assert.Equal(t, domain.RestartConditionOnFailure, rp.Condition)
	require.NotNil(t, rp.Delay)
	assert.Equal(t, 10*time.Second, *rp.Delay)
	require.NotNil(t, rp.MaxAttempts)
	assert.Equal(t, uint64(5), *rp.MaxAttempts)
	require.NotNil(t, rp.Window)
	assert.Equal(t, 3*time.Minute, *rp.Window)
}

func TestServiceSpec_RoundTrip_Resources(t *testing.T) {
	t.Parallel()

	original := &domain.ServiceSpec{
		Name: "resources-rt",
		TaskTemplate: domain.TaskSpec{
			ContainerSpec: domain.ContainerSpec{
				Image: "nginx:latest",
			},
			Resources: &domain.ResourceRequirements{
				Limits: &domain.Resources{
					NanoCPUs:    2000000000, // 2 CPUs
					MemoryBytes: 1073741824, // 1 GiB
				},
				Reservations: &domain.Resources{
					NanoCPUs:    500000000, // 0.5 CPU
					MemoryBytes: 268435456, // 256 MiB
				},
			},
		},
	}

	// toSwarm direction should work
	swarmSpec := convertToSwarmSpec(original)
	require.NotNil(t, swarmSpec.TaskTemplate.Resources)
	require.NotNil(t, swarmSpec.TaskTemplate.Resources.Limits)
	assert.Equal(t, int64(2000000000), swarmSpec.TaskTemplate.Resources.Limits.NanoCPUs)
	assert.Equal(t, int64(1073741824), swarmSpec.TaskTemplate.Resources.Limits.MemoryBytes)
	require.NotNil(t, swarmSpec.TaskTemplate.Resources.Reservations)
	assert.Equal(t, int64(500000000), swarmSpec.TaskTemplate.Resources.Reservations.NanoCPUs)
	assert.Equal(t, int64(268435456), swarmSpec.TaskTemplate.Resources.Reservations.MemoryBytes)

	// Full round-trip
	result := roundTripServiceSpec(original)
	require.NotNil(t, result)

	res := result.Spec.TaskTemplate.Resources
	require.NotNil(t, res, "Resources should survive round-trip")
	require.NotNil(t, res.Limits, "Resource Limits should survive round-trip")
	assert.Equal(t, int64(2000000000), res.Limits.NanoCPUs)
	assert.Equal(t, int64(1073741824), res.Limits.MemoryBytes)
	require.NotNil(t, res.Reservations, "Resource Reservations should survive round-trip")
	assert.Equal(t, int64(500000000), res.Reservations.NanoCPUs)
	assert.Equal(t, int64(268435456), res.Reservations.MemoryBytes)
}

func TestServiceSpec_RoundTrip_ModeReplicated(t *testing.T) {
	t.Parallel()

	original := &domain.ServiceSpec{
		Name: "replicated-rt",
		TaskTemplate: domain.TaskSpec{
			ContainerSpec: domain.ContainerSpec{
				Image: "nginx:latest",
			},
		},
		Mode: domain.ServiceMode{
			Replicated: &domain.ReplicatedService{
				Replicas: uint64Ptr(3),
			},
		},
	}

	// toSwarm direction should work
	swarmSpec := convertToSwarmSpec(original)
	require.NotNil(t, swarmSpec.Mode.Replicated)
	require.NotNil(t, swarmSpec.Mode.Replicated.Replicas)
	assert.Equal(t, uint64(3), *swarmSpec.Mode.Replicated.Replicas)

	// Full round-trip
	result := roundTripServiceSpec(original)
	require.NotNil(t, result)

	require.NotNil(t, result.Spec.Mode.Replicated, "Mode.Replicated should survive round-trip")
	require.NotNil(t, result.Spec.Mode.Replicated.Replicas)
	assert.Equal(t, uint64(3), *result.Spec.Mode.Replicated.Replicas)
	assert.Nil(t, result.Spec.Mode.Global, "Global should be nil for replicated mode")
}

func TestServiceSpec_RoundTrip_ModeGlobal(t *testing.T) {
	t.Parallel()

	original := &domain.ServiceSpec{
		Name: "global-rt",
		TaskTemplate: domain.TaskSpec{
			ContainerSpec: domain.ContainerSpec{
				Image: "nginx:latest",
			},
		},
		Mode: domain.ServiceMode{
			Global: &domain.GlobalService{},
		},
	}

	// toSwarm direction should work
	swarmSpec := convertToSwarmSpec(original)
	require.NotNil(t, swarmSpec.Mode.Global)
	assert.Nil(t, swarmSpec.Mode.Replicated)

	// Full round-trip
	result := roundTripServiceSpec(original)
	require.NotNil(t, result)

	require.NotNil(t, result.Spec.Mode.Global, "Mode.Global should survive round-trip")
	assert.Nil(t, result.Spec.Mode.Replicated, "Replicated should be nil for global mode")
}

func TestServiceSpec_RoundTrip_Mounts(t *testing.T) {
	t.Parallel()

	original := &domain.ServiceSpec{
		Name: "mounts-rt",
		TaskTemplate: domain.TaskSpec{
			ContainerSpec: domain.ContainerSpec{
				Image: "nginx:latest",
				Mounts: []domain.ServiceMount{
					{
						Type:     domain.MountTypeBind,
						Source:   "/etc/config",
						Target:   "/app/config",
						ReadOnly: true,
					},
					{
						Type:     domain.MountTypeVolume,
						Source:   "app-data",
						Target:   "/app/data",
						ReadOnly: false,
					},
				},
			},
		},
	}

	// toSwarm direction should work
	swarmSpec := convertToSwarmSpec(original)
	require.Len(t, swarmSpec.TaskTemplate.ContainerSpec.Mounts, 2)
	assert.Equal(t, "/etc/config", swarmSpec.TaskTemplate.ContainerSpec.Mounts[0].Source)
	assert.Equal(t, "/app/config", swarmSpec.TaskTemplate.ContainerSpec.Mounts[0].Target)
	assert.True(t, swarmSpec.TaskTemplate.ContainerSpec.Mounts[0].ReadOnly)
	assert.Equal(t, "app-data", swarmSpec.TaskTemplate.ContainerSpec.Mounts[1].Source)
	assert.Equal(t, "/app/data", swarmSpec.TaskTemplate.ContainerSpec.Mounts[1].Target)
	assert.False(t, swarmSpec.TaskTemplate.ContainerSpec.Mounts[1].ReadOnly)

	// Full round-trip
	result := roundTripServiceSpec(original)
	require.NotNil(t, result)

	mounts := result.Spec.TaskTemplate.ContainerSpec.Mounts
	require.Len(t, mounts, 2, "Mounts should survive round-trip")
	assert.Equal(t, domain.MountTypeBind, mounts[0].Type)
	assert.Equal(t, "/etc/config", mounts[0].Source)
	assert.Equal(t, "/app/config", mounts[0].Target)
	assert.True(t, mounts[0].ReadOnly)
	assert.Equal(t, domain.MountTypeVolume, mounts[1].Type)
	assert.Equal(t, "app-data", mounts[1].Source)
	assert.Equal(t, "/app/data", mounts[1].Target)
	assert.False(t, mounts[1].ReadOnly)
}
