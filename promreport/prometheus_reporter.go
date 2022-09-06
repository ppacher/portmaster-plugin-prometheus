package promreport

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/safing/portmaster/plugin/shared/proto"
	"github.com/safing/portmaster/plugin/shared/reporter"
)

type (
	Config struct {
		// Namespace for the registered metrics.
		Namespace string
		// Subsystem for the registered metrics.
		Subsystem string
		// Registerer is the prometheus registerer that should be used
		// for all metrics exposed by the PromtheusReporter. If left empty
		// prometheus.DefaultRegisterer will be used.
		Registerer prometheus.Registerer
	}

	// PrometheusReporter exposes connection metrics from Portmaster
	// via Prometheus.
	PrometheusReporter struct {
		registerer        prometheus.Registerer
		connectionCounter *prometheus.CounterVec
		domainCounter     *prometheus.CounterVec
	}
)

func NewPrometheusReporter(cfg *Config) (*PrometheusReporter, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	reporter := &PrometheusReporter{
		registerer: cfg.Registerer,
	}

	if reporter.registerer == nil {
		reporter.registerer = prometheus.DefaultRegisterer
	}

	reporter.connectionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "portmaster_connections_total",
			Help:      "The total number of processed connections",
		},
		[]string{
			"type",
			"verdict",
		},
	)

	reporter.domainCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "prometheus_domains_total",
			Help:      "The total number of processed connections by domain",
		},
		[]string{
			"domain",
			"verdict",
		},
	)

	if err := reporter.registerer.Register(reporter.connectionCounter); err != nil {
		return nil, err
	}

	if err := reporter.registerer.Register(reporter.domainCounter); err != nil {
		return nil, err
	}

	return reporter, nil
}

// Registerer returns the prometheus registerer used by the reporter.
func (plg *PrometheusReporter) Registerer() prometheus.Registerer {
	return plg.registerer
}

func (plg *PrometheusReporter) ReportConnection(ctx context.Context, conn *proto.Connection) error {
	verdict := getConnVerdict(conn)
	typeString := getConnType(conn)

	plg.connectionCounter.WithLabelValues(typeString, verdict).Inc()

	if conn.GetType() == proto.ConnectionType_CONNECTION_TYPE_IP {

		if domain := conn.GetEntity().GetDomain(); domain != "" {
			plg.domainCounter.WithLabelValues(domain, verdict).Inc()
		}
	}

	return nil
}

func getConnType(conn *proto.Connection) string {
	return strings.ToLower(
		strings.Replace(proto.ConnectionType_name[int32(conn.GetType())], "CONNECTION_TYPE_", "", 1),
	)
}

func getConnVerdict(conn *proto.Connection) string {
	return strings.ToLower(
		strings.Replace(proto.Verdict_name[int32(conn.GetVerdict())], "VERDICT_", "", 1),
	)
}

// Check interfaces
var _ reporter.Reporter = new(PrometheusReporter)
