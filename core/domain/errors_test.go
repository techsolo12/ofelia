// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestContainerNotFoundError(t *testing.T) {
	t.Parallel()

	err := &ContainerNotFoundError{ID: "abc123"}

	t.Run("Error", func(t *testing.T) {
		got := err.Error()
		want := "container not found: abc123"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Is_ErrNotFound", func(t *testing.T) {
		if !errors.Is(err, ErrNotFound) {
			t.Error("expected errors.Is(err, ErrNotFound) to be true")
		}
	})

	t.Run("Is_OtherError", func(t *testing.T) {
		if errors.Is(err, ErrConflict) {
			t.Error("expected errors.Is(err, ErrConflict) to be false")
		}
	})
}

func TestImageNotFoundError(t *testing.T) {
	t.Parallel()

	err := &ImageNotFoundError{Image: "nginx:latest"}

	t.Run("Error", func(t *testing.T) {
		got := err.Error()
		want := "image not found: nginx:latest"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Is_ErrNotFound", func(t *testing.T) {
		if !errors.Is(err, ErrNotFound) {
			t.Error("expected errors.Is(err, ErrNotFound) to be true")
		}
	})

	t.Run("Is_OtherError", func(t *testing.T) {
		if errors.Is(err, ErrTimeout) {
			t.Error("expected errors.Is(err, ErrTimeout) to be false")
		}
	})
}

func TestNetworkNotFoundError(t *testing.T) {
	t.Parallel()

	err := &NetworkNotFoundError{Network: "my-network"}

	t.Run("Error", func(t *testing.T) {
		got := err.Error()
		want := "network not found: my-network"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Is_ErrNotFound", func(t *testing.T) {
		if !errors.Is(err, ErrNotFound) {
			t.Error("expected errors.Is(err, ErrNotFound) to be true")
		}
	})

	t.Run("Is_OtherError", func(t *testing.T) {
		if errors.Is(err, ErrCanceled) {
			t.Error("expected errors.Is(err, ErrCanceled) to be false")
		}
	})
}

func TestServiceNotFoundError(t *testing.T) {
	t.Parallel()

	err := &ServiceNotFoundError{ID: "svc-42"}

	t.Run("Error", func(t *testing.T) {
		got := err.Error()
		want := "service not found: svc-42"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Is_ErrNotFound", func(t *testing.T) {
		if !errors.Is(err, ErrNotFound) {
			t.Error("expected errors.Is(err, ErrNotFound) to be true")
		}
	})

	t.Run("Is_OtherError", func(t *testing.T) {
		if errors.Is(err, ErrForbidden) {
			t.Error("expected errors.Is(err, ErrForbidden) to be false")
		}
	})
}

func TestExecNotFoundError(t *testing.T) {
	t.Parallel()

	err := &ExecNotFoundError{ID: "exec-99"}

	t.Run("Error", func(t *testing.T) {
		got := err.Error()
		want := "exec not found: exec-99"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("Is_ErrNotFound", func(t *testing.T) {
		if !errors.Is(err, ErrNotFound) {
			t.Error("expected errors.Is(err, ErrNotFound) to be true")
		}
	})

	t.Run("Is_OtherError", func(t *testing.T) {
		if errors.Is(err, ErrUnauthorized) {
			t.Error("expected errors.Is(err, ErrUnauthorized) to be false")
		}
	})
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"sentinel", ErrNotFound, true},
		{"container", &ContainerNotFoundError{ID: "c1"}, true},
		{"image", &ImageNotFoundError{Image: "img"}, true},
		{"network", &NetworkNotFoundError{Network: "net"}, true},
		{"service", &ServiceNotFoundError{ID: "svc"}, true},
		{"exec", &ExecNotFoundError{ID: "ex"}, true},
		{"wrapped", fmt.Errorf("wrap: %w", ErrNotFound), true},
		{"conflict", ErrConflict, false},
		{"nil", nil, false},
		{"other", errors.New("something else"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"sentinel", ErrConflict, true},
		{"wrapped", fmt.Errorf("wrap: %w", ErrConflict), true},
		{"not_found", ErrNotFound, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConflict(tt.err); got != tt.want {
				t.Errorf("IsConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"sentinel", ErrTimeout, true},
		{"wrapped", fmt.Errorf("wrap: %w", ErrTimeout), true},
		{"not_found", ErrNotFound, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTimeout(tt.err); got != tt.want {
				t.Errorf("IsTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCanceled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"sentinel", ErrCanceled, true},
		{"wrapped", fmt.Errorf("wrap: %w", ErrCanceled), true},
		{"timeout", ErrTimeout, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCanceled(tt.err); got != tt.want {
				t.Errorf("IsCanceled() = %v, want %v", got, tt.want)
			}
		})
	}
}
