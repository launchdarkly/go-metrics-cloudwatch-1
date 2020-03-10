package reporter

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"

	metrics "github.com/launchdarkly/go-metrics"
	"github.com/launchdarkly/go-metrics-cloudwatch/config"
)

var Silence = false

//blocks, run as go reporter.Cloudwatch(cfg)
func Cloudwatch(cfg *config.Config) {
	ticks := time.NewTicker(cfg.ReportingInterval)
	defer ticks.Stop()
	for {
		select {
		case <-ticks.C:
			EmitMetrics(cfg)
		}
	}
}

func EmitMetrics(cfg *config.Config) error {
	data := metricsData(cfg)
	var err error
	//20 is the max metrics per request
	for len(data) > 20 {
		put := data[0:20]
		err = putMetrics(cfg, put)
		data = data[20:]
	}

	if len(data) > 0 {
		err = putMetrics(cfg, data)
	}
	return err

}

func putMetrics(cfg *config.Config, data []*cloudwatch.MetricDatum) error {
	client := cfg.Client
	req := &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(cfg.Namespace),
		MetricData: data,
	}
	_, err := client.PutMetricData(req)
	if err != nil {
		return fmt.Errorf("component=cloudwatch-reporter fn=EmitMetrics at=error error=%s", err)
	}
	return nil
}

func metricsData(cfg *config.Config) []*cloudwatch.MetricDatum {
	counters, gauges, histos, meters, timers := 0, 0, 0, 0, 0
	countersOut, gaugesOut, histosOut, metersOut, timersOut := 0, 0, 0, 0, 0

	data := []*cloudwatch.MetricDatum{}
	timestamp := aws.Time(time.Now())

	aDatum := func(name string) *cloudwatch.MetricDatum {
		return &cloudwatch.MetricDatum{
			MetricName: aws.String(name),
			Timestamp:  timestamp,
			Dimensions: dimensions(cfg),
		}
	}
	//rough port from the graphite reporter
	cfg.Registry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case metrics.Counter:
			counters += 1
			// For the LD flavor of go-metrics we do an atomic snapshot and clear
			// Caveat: we cannot have multiple reports reading and clearing this metric
			var count float64
			if cfg.PreviousCounterValues != nil {
				current := metric.Count()
				count = float64(current - cfg.PreviousCounterValues[name])
				cfg.PreviousCounterValues[name] = current
			} else {
				count = float64(metric.Clear().Count())
			}
			if cfg.Filter.ShouldReport(name, count) {
				datum := aDatum(name)
				datum.Unit = aws.String(cloudwatch.StandardUnitCount)
				datum.Value = aws.Float64(count)
				data = append(data, datum)
				countersOut += 1
			}
		case metrics.GaugeCounter:
			// treat these like Counter
			counters += 1
			count := float64(metric.Count())
			if cfg.Filter.ShouldReport(name, count) {
				datum := aDatum(name)
				datum.Unit = aws.String(cloudwatch.StandardUnitCount)
				datum.Value = aws.Float64(count)
				data = append(data, datum)
				countersOut += 1
			}
			// We don't clear gauge counters on reporting because they cannot recover their value like normal gauges
			// They can only increment and decrement and so cannot be cleared after they are created.
		case metrics.Gauge:
			gauges += 1
			value := float64(metric.Value())
			if cfg.Filter.ShouldReport(name, value) {
				datum := aDatum(name)
				datum.Unit = aws.String(cloudwatch.StandardUnitCount)
				datum.Value = aws.Float64(float64(value))
				data = append(data, datum)
				gaugesOut += 1
			}
		case metrics.GaugeFloat64:
			gauges += 1
			value := float64(metric.Value())
			if cfg.Filter.ShouldReport(name, value) {
				datum := aDatum(name)
				datum.Unit = aws.String(cloudwatch.StandardUnitCount)
				datum.Value = aws.Float64(value)
				data = append(data, datum)
				gaugesOut += 1
			}
		case metrics.Histogram:
			histos += 1
			h := metric.Clear() // For the LD flavor of go-metrics we do an atomic snapshot and clear

			for n, v := range map[string]float64{
				fmt.Sprintf("%s.count", name):   float64(h.Count()),
				fmt.Sprintf("%s.min", name):     float64(h.Min()),
				fmt.Sprintf("%s.max", name):     float64(h.Max()),
				fmt.Sprintf("%s.mean", name):    h.Mean(),
				fmt.Sprintf("%s.std-dev", name): h.StdDev(),
			} {
				if cfg.Filter.ShouldReport(n, v) {
					datum := aDatum(n)
					datum.Value = aws.Float64(v)
					data = append(data, datum)
					histosOut += 1
				}
			}
			for _, p := range cfg.Filter.Percentiles(name) {
				pname := fmt.Sprintf("%s-perc%.3f", name, p)
				pvalue := h.Percentile(p)
				if cfg.Filter.ShouldReport(pname, pvalue) {
					datum := aDatum(pname)
					datum.Value = aws.Float64(pvalue)
					data = append(data, datum)
					histosOut += 1
				}
			}
		case metrics.Meter:
			meters += 1
			m := metric.Snapshot()
			for n, v := range map[string]float64{
				fmt.Sprintf("%s.count", name):          float64(m.Count()),
				fmt.Sprintf("%s.one-minute", name):     m.Rate1(),
				fmt.Sprintf("%s.five-minute", name):    m.Rate5(),
				fmt.Sprintf("%s.fifteen-minute", name): m.Rate15(),
				fmt.Sprintf("%s.mean", name):           m.RateMean(),
			} {
				if cfg.Filter.ShouldReport(n, v) {
					datum := aDatum(n)
					datum.Value = aws.Float64(v)
					data = append(data, datum)
					metersOut += 1
				}
			}
		case metrics.Timer:
			timers += 1
			t := metric.Clear() // For the LD flavor of go-metrics we do an atomic snapshot and clear
			if t.Count() == 0 {
				return
			}

			for n, v := range map[string]float64{
				fmt.Sprintf("%s.count", name):          float64(t.Count()),
				fmt.Sprintf("%s.rate-mean", name):      t.RateMean(),
				fmt.Sprintf("%s.one-minute", name):     t.Rate1(),
				fmt.Sprintf("%s.five-minute", name):    t.Rate5(),
				fmt.Sprintf("%s.fifteen-minute", name): t.Rate15(),
				fmt.Sprintf("%s.min", name):            float64(t.Min() / int64(cfg.DurationUnit)),
				fmt.Sprintf("%s.max", name):            float64(t.Max() / int64(cfg.DurationUnit)),
				fmt.Sprintf("%s.mean", name):           t.Mean() / float64(cfg.DurationUnit),
				fmt.Sprintf("%s.std-dev", name):        t.StdDev() / float64(cfg.DurationUnit),
			} {
				if cfg.Filter.ShouldReport(n, v) {
					datum := aDatum(n)
					datum.Value = aws.Float64(v)
					data = append(data, datum)
					timersOut += 1
				}
			}

			for _, p := range cfg.Filter.Percentiles(name) {
				pname := fmt.Sprintf("%s-perc%.3f", name, p)
				pvalue := t.Percentile(p)
				if cfg.Filter.ShouldReport(pname, pvalue) {
					datum := aDatum(pname)
					datum.Value = aws.Float64(pvalue / float64(cfg.DurationUnit))
					data = append(data, datum)
					timersOut += 1
				}
			}
		}
	})
	total := counters + gauges + histos + meters + timers
	totalOut := countersOut + gaugesOut + histosOut + metersOut + timersOut
	if !Silence {
		log.Printf("component=cloudwatch-reporter fn=metricsData at=sources total=%d counters=%d gauges=%d histos=%d meters=%d timers=%d", total, counters, gauges, histos, meters, timers)
		log.Printf("component=cloudwatch-reporter fn=metricsData at=targets total=%d counters=%d gauges=%d histos=%d meters=%d timers=%d", totalOut, countersOut, gaugesOut, histosOut, metersOut, timersOut)
	}

	return data
}

func dimensions(c *config.Config) []*cloudwatch.Dimension {
	ds := []*cloudwatch.Dimension{}
	for k, v := range c.StaticDimensions {
		d := &cloudwatch.Dimension{
			Name:  aws.String(k),
			Value: aws.String(v),
		}

		ds = append(ds, d)
	}
	return ds
}
