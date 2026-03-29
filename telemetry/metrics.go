package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ToolCallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_gateway_tool_calls_total",
		Help: "Total number of tool calls",
	}, []string{"spec", "operation", "status"})

	ProxyDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mcp_gateway_proxy_duration_seconds",
		Help:    "Proxy request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"spec", "operation"})

	ActiveSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_gateway_active_sessions",
		Help: "Number of active SSE sessions",
	})

	RegistryToolsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_gateway_registry_tools_total",
		Help: "Total number of tools in the registry",
	})
)

func Register() {
	// Metrics are auto-registered via promauto
}
