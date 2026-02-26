// Copyright (c) 2025-2026 Netresearch DTT GmbH
// SPDX-License-Identifier: MIT

package cli

// setJobSource assigns the given JobSource to all jobs in the provided Config.
func setJobSource(cfg *Config, src JobSource) {
	for _, j := range cfg.ExecJobs {
		j.JobSource = src
	}
	for _, j := range cfg.RunJobs {
		j.JobSource = src
	}
	for _, j := range cfg.LocalJobs {
		j.JobSource = src
	}
	for _, j := range cfg.ServiceJobs {
		j.JobSource = src
	}
	for _, j := range cfg.ComposeJobs {
		j.JobSource = src
	}
}
