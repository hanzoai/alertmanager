// Copyright 2019 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"github.com/prometheus/alertmanager/internal/promshim"
)

// Alerts stores metrics for alerts.
type Alerts struct {
	firing   promshim.Counter
	resolved promshim.Counter
	invalid  promshim.Counter
}

// NewAlerts returns an *Alerts struct for the given API version.
// Since v1 was deprecated in 0.27, v2 is now hardcoded.
func NewAlerts(r promshim.Registerer) *Alerts {
	if r == nil {
		return nil
	}
	numReceivedAlerts := promshim.With(r).NewCounterVec(promshim.CounterOpts{
		Name:        "alertmanager_alerts_received_total",
		Help:        "The total number of received alerts.",
		ConstLabels: promshim.Labels{"version": "v2"},
	}, []string{"status"})
	numInvalidAlerts := promshim.With(r).NewCounter(promshim.CounterOpts{
		Name:        "alertmanager_alerts_invalid_total",
		Help:        "The total number of received alerts that were invalid.",
		ConstLabels: promshim.Labels{"version": "v2"},
	})
	return &Alerts{
		firing:   numReceivedAlerts.WithLabelValues("firing"),
		resolved: numReceivedAlerts.WithLabelValues("resolved"),
		invalid:  numInvalidAlerts,
	}
}

// Firing returns a counter of firing alerts.
func (a *Alerts) Firing() promshim.Counter { return a.firing }

// Resolved returns a counter of resolved alerts.
func (a *Alerts) Resolved() promshim.Counter { return a.resolved }

// Invalid returns a counter of invalid alerts.
func (a *Alerts) Invalid() promshim.Counter { return a.invalid }
