// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/gobs/args"
)

// localJobEnvResolveTimeout bounds env-from-container resolution for local
// jobs so a wedged Docker daemon cannot stall the local-exec path
// indefinitely. Inspecting a Docker container for its environment is a
// fast read; 10s is a generous upper bound. See issue #638.
const localJobEnvResolveTimeout = 10 * time.Second

type LocalJob struct {
	BareJob     `mapstructure:",squash"`
	Dir         string   `hash:"true"`
	Environment []string `mapstructure:"environment" hash:"true"`
	EnvFile     []string `gcfg:"env-file" mapstructure:"env-file," hash:"true"`
	EnvFrom     []string `gcfg:"env-from" mapstructure:"env-from," hash:"true"`
}

func NewLocalJob() *LocalJob {
	return &LocalJob{}
}

func (j *LocalJob) Run(ctx *Context) error {
	cmd, err := j.buildCommand(ctx)
	if err != nil {
		return err
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("local run: %w", err)
	}
	return nil
}

func (j *LocalJob) buildCommand(ctx *Context) (*exec.Cmd, error) {
	// Parse command arguments and ensure non-empty
	// Note: Config file commands are validated during load by config.Validator2
	// API-created jobs are validated in web/server.go before reaching here
	cmdArgs := args.GetArgs(j.Command)
	if len(cmdArgs) == 0 {
		return nil, ErrEmptyCommand
	}

	bin, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		return nil, fmt.Errorf("look path %q: %w", cmdArgs[0], err)
	}

	// Resolve environment from env-file, env-from, and explicit environment.
	// Bound the resolver context with localJobEnvResolveTimeout so a wedged
	// Docker daemon (env-from) cannot stall local-job startup indefinitely.
	// Inherits cancellation from the scheduler's per-run bounded context
	// when present; otherwise (*Context).RunContext returns
	// context.Background() so legacy *Context{} literals keep working.
	// See issue #638.
	resolveCtx, cancel := context.WithTimeout(ctx.RunContext(), localJobEnvResolveTimeout)
	defer cancel()
	mergedEnv, err := ResolveJobEnvironment(resolveCtx, j.EnvFile, j.EnvFrom, j.Environment, nil, func(msg string) {
		ctx.Warn(msg)
	})
	if err != nil {
		return nil, err
	}

	return &exec.Cmd{
		Path:   bin,
		Args:   cmdArgs,
		Stdout: ctx.Execution.OutputStream,
		Stderr: ctx.Execution.ErrorStream,
		// add custom env variables to the existing ones
		// instead of overwriting them
		Env: append(os.Environ(), mergedEnv...),
		Dir: j.Dir,
	}, nil
}
