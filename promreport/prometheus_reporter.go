package promreport

import (
	"context"

	"github.com/safing/portmaster/plugin/shared/proto"
	"github.com/safing/portmaster/plugin/shared/reporter"
)

// PrometheusReporter exposes connection metrics from Portmaster
// via Prometheus.
type PrometheusReporter struct {
}

func (plg *PrometheusReporter) ReportConnection(ctx context.Context, conn *proto.Connection) error {
	return nil
}

// Check interfaces
var _ reporter.Reporter = new(PrometheusReporter)
