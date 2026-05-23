// Copyright 2021 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

// Package tracing is the o11y-fork noop replacement for the upstream
// alertmanager tracing package.
//
// Upstream imports otlptrace + otlptracegrpc + otlptracehttp, which pull
// google.golang.org/grpc into every binary that links alertmanager. The
// o11y stack uses luxfi/trace (ZAP-native span exporter) instead, so the
// upstream OTLP tracing is dead weight that braids grpc into the dep
// graph. This file keeps Manager.NewManager/Run/ApplyConfig/Stop as a
// no-op — same exported surface, zero grpc, zero otlp.
package tracing

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/alertmanager/config"
)

// Manager is the public type the rest of alertmanager constructs. With
// tracing exporters disabled, every method is a no-op.
type Manager struct {
	logger *slog.Logger
}

// NewManager returns a tracing Manager that does nothing.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{logger: logger}
}

// Run is a no-op. The original spins a reload loop; we have nothing to drive.
func (m *Manager) Run() {}

// ApplyConfig is a no-op. Original validates + rebuilds the OTLP exporter
// from cfg.Tracing — we don't ship one.
func (m *Manager) ApplyConfig(_ *config.Config) error { return nil }

// Stop is a no-op.
func (m *Manager) Stop() {}

// Middleware is a passthrough wrapper. Upstream wraps the http.Handler
// with otelhttp.NewHandler to emit OTLP spans per request — we don't
// ship that exporter chain, so just return the handler unchanged.
func Middleware(h http.Handler) http.Handler { return h }
