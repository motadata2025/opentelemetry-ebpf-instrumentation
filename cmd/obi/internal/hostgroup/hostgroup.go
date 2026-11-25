// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package hostgroup reads the host group from the Motadata agent configuration and
// attaches it as a resource attribute to the traces exported by OBI.
package hostgroup // import "go.opentelemetry.io/obi/cmd/obi/internal/hostgroup"

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/obi/pkg/instrumenter"
	"go.opentelemetry.io/obi/pkg/pipe/global"
)

const (
	agentConfigRelPath = "config/agent.json"
	attributeName      = "md.host.group"
)

type agentConfig struct {
	Agent struct {
		// "trace.host.group" is a single key containing dots, not a nested path.
		TraceHostGroup *string `json:"trace.host.group"`
	} `json:"agent"`
}

// Load reads the host group from config/agent.json in the directory of the running executable.
func Load() string {
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("can't resolve executable path, using empty host group", "error", err)
		return ""
	}

	return Read(agentConfigPath(exe))
}

func agentConfigPath(exe string) string {
	return filepath.Join(filepath.Dir(exe), agentConfigRelPath)
}

// Read returns the agent."trace.host.group" value of the Motadata agent configuration at path.
// It returns an empty string when the file can't be read or parsed, or when the key is
// missing or not a string.
func Read(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("can't read agent configuration, using empty host group", "path", path, "error", err)
		return ""
	}

	var cfg agentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		slog.Warn("can't parse agent configuration, using empty host group", "path", path, "error", err)
		return ""
	}

	if cfg.Agent.TraceHostGroup == nil {
		slog.Warn("agent configuration has no agent.trace.host.group, using empty host group", "path", path)
		return ""
	}

	slog.Info("trace host group loaded", "path", path, attributeName, *cfg.Agent.TraceHostGroup)
	return *cfg.Agent.TraceHostGroup
}

// WithResourceAttribute adds the host group to the resource attributes of every exported trace.
// Extra resource attributes are applied last, so they override any value set by the application.
func WithResourceAttribute(hostGroup string) instrumenter.Option {
	return func(info *global.ContextInfo) {
		info.ExtraResourceAttributes = append(info.ExtraResourceAttributes, attribute.String(attributeName, hostGroup))
	}
}
