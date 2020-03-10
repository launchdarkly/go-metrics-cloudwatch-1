package config

import (
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatch"

	"github.com/launchdarkly/go-metrics"
)

const (
	Perc50  = float64(0.50)
	Perc75  = float64(0.75)
	Perc95  = float64(0.95)
	Perc99  = float64(0.99)
	Perc999 = float64(0.999)
	Perc100 = float64(1)
)

type PutMetricsClient interface {
	PutMetricData(*cloudwatch.PutMetricDataInput) (*cloudwatch.PutMetricDataOutput, error)
}

type Config struct {
	Filter            Filter
	Client            PutMetricsClient
	ReportingInterval time.Duration
	Registry          metrics.Registry
	Namespace         string
	StaticDimensions  map[string]string
	DurationUnit      time.Duration

	PreviousCounterValues map[string]int64 // when provided, we store previous counters here and use them to compute diffs instead of clearing counters
}

type Filter interface {
	ShouldReport(metric string, value float64) bool
	Percentiles(metric string) []float64
}

type NoFilter struct{}

func (n *NoFilter) ShouldReport(metric string, value float64) bool {
	return true
}

func (n *NoFilter) Percentiles(metric string) []float64 {
	return []float64{Perc50, Perc75, Perc95, Perc99, Perc999, Perc100}
}

type AllFilter struct{}

func (n *AllFilter) ShouldReport(metric string, value float64) bool {
	return false
}

func (n *AllFilter) Percentiles(metric string) []float64 {
	return []float64{}
}

/*
type DynamoDBFilter struct {
	globalEnabledMetrics []string
	perInstanceEnabledMetrics map[string]string
}

func (d *DynamodbConfig) PollConfig() {
	poll once every few minutes, read enabled metrics
}
*/
