// Copyright 2024 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

// Noop replacement — see tracing.go for context. Upstream wraps
// http.RoundTripper with otelhttp + injects span context; we return
// the transport untouched so no otel/otlp/grpc machinery is pulled in.

package tracing

import "net/http"

// Transport returns rt unchanged (or http.DefaultTransport if nil).
// Upstream wrapped rt with otelhttp.NewTransport(...) which pulls the
// OTel HTTP instrumentation tree — the noop drops that.
func Transport(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		return http.DefaultTransport
	}
	return rt
}
