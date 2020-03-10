package reporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatch"

	metrics "github.com/launchdarkly/go-metrics"
	"github.com/launchdarkly/go-metrics-cloudwatch/config"
)

type MockPutMetricsClient struct {
	metricsPut int
	requests   int
}

func (m *MockPutMetricsClient) PutMetricData(in *cloudwatch.PutMetricDataInput) (*cloudwatch.PutMetricDataOutput, error) {
	m.metricsPut += len(in.MetricData)
	m.requests += 1
	return &cloudwatch.PutMetricDataOutput{}, nil
}

func TestCloudwatchReporter(t *testing.T) {
	mock := &MockPutMetricsClient{}
	registry := metrics.NewRegistry()
	cfg := &config.Config{
		Client:       mock,
		Filter:       &config.NoFilter{},
		Registry:     registry,
		DurationUnit: time.Millisecond,
	}

	for i := 0; i < 30; i++ {
		count := metrics.GetOrRegisterCounter(fmt.Sprintf("count-%d", i), registry)
		count.Inc(1)
	}

	EmitMetrics(cfg)

	if mock.metricsPut < 30 || mock.requests < 2 {
		t.Fatal("No Metrics Put")
	}
}

func TestCounters(t *testing.T) {
	mock := &MockPutMetricsClient{}
	registry := metrics.NewRegistry()
	cfg := &config.Config{
		Client:       mock,
		Filter:       &config.NoFilter{},
		Registry:     registry,
		DurationUnit: time.Millisecond,
	}
	counter := metrics.GetOrRegisterCounter(fmt.Sprintf("counter"), registry)
	counter.Inc(1)
	EmitMetrics(cfg)
	if counter.Count() != 0 {
		t.Fatalf("expected counter to be cleared but got %d", counter.Count())
	}

	if mock.metricsPut < 1 {
		t.Fatal("No Metrics Put")
	}
}


func TestCountersWithPreviousValueSnapshots(t *testing.T) {
	mock := &MockPutMetricsClient{}
	registry := metrics.NewRegistry()
	cfg := &config.Config{
		Client:       mock,
		Filter:       &config.NoFilter{},
		Registry:     registry,
		DurationUnit: time.Millisecond,

		PreviousCounterValues: make(map[string]int64),
	}
	counter := metrics.GetOrRegisterCounter(fmt.Sprintf("counter"), registry)
	counter.Inc(2)
	EmitMetrics(cfg)
	if expected, actual := int64(2), counter.Count(); expected != actual {
		t.Fatalf("expected counter to be %d but got %d", expected, actual)
	}

	counter.Inc(1)
	EmitMetrics(cfg)
	if expected, actual := int64(3), counter.Count(); expected != actual {
		t.Fatalf("expected counter to be %d but got %d", expected, actual)
	}
	if expected, actual := 2, mock.metricsPut; expected != mock.metricsPut {
		t.Fatalf("Expected to put %d but put %d", expected, actual)
	}
}


func TestGaugeCounters(t *testing.T) {
	mock := &MockPutMetricsClient{}
	registry := metrics.NewRegistry()
	cfg := &config.Config{
		Client:       mock,
		Filter:       &config.NoFilter{},
		Registry:     registry,
		DurationUnit: time.Millisecond,
	}
	gaugeCounter := metrics.GetOrRegisterGaugeCounter(fmt.Sprintf("gauge-counter"), registry)
	gaugeCounter.Dec(1)
	EmitMetrics(cfg)

	if mock.metricsPut < 1 {
		t.Fatal("No Metrics Put")
	}
}

func TestHistograms(t *testing.T) {
	mock := &MockPutMetricsClient{}
	registry := metrics.NewRegistry()
	filter := &config.NoFilter{}
	cfg := &config.Config{
		Client:       mock,
		Filter:       filter,
		Registry:     registry,
		DurationUnit: time.Millisecond,
	}

	hist := metrics.GetOrRegisterHistogram(fmt.Sprintf("histo"), registry, metrics.NewUniformSample(1024))
	hist.Update(1000)
	hist.Update(500)
	EmitMetrics(cfg)

	if mock.metricsPut < len(filter.Percentiles("")) {
		t.Fatal("No Metrics Put")
	}
}

func TestTimers(t *testing.T) {
	mock := &MockPutMetricsClient{}
	registry := metrics.NewRegistry()
	cfg := &config.Config{
		Client:       mock,
		Filter:       &config.NoFilter{},
		Registry:     registry,
		DurationUnit: time.Millisecond,
	}
	timer := metrics.GetOrRegisterTimer(fmt.Sprintf("timer"), registry)
	timer.Update(10 * time.Second)
	EmitMetrics(cfg)

	if mock.metricsPut < 7 {
		t.Fatal("No Metrics Put")
	}
}

func TestFilters(t *testing.T) {
	mock := &MockPutMetricsClient{}
	registry := metrics.NewRegistry()
	cfg := &config.Config{
		Client:       mock,
		Filter:       &config.AllFilter{},
		Registry:     registry,
		DurationUnit: time.Millisecond,
	}

	timer := metrics.GetOrRegisterTimer(fmt.Sprintf("timer"), registry)
	timer.Update(10 * time.Second)
	EmitMetrics(cfg)

	if mock.metricsPut > 0 {
		t.Fatal("Metrics Put")
	}
}
