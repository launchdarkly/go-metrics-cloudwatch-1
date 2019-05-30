package metrics

import "sync/atomic"

// Counters hold an int64 value that can be incremented and decremented.
type GaugeCounter interface {
  Count() int64
  Dec(int64)
  Inc(int64)
  Snapshot() GaugeCounter
}

// GetOrRegisterCounter returns an existing GaugeCounter or constructs and registers
// a new StandardGaugeCounter.
func GetOrRegisterGaugeCounter(name string, r Registry) GaugeCounter {
  if nil == r {
    r = DefaultRegistry
  }
  return r.GetOrRegister(name, NewGaugeCounter).(GaugeCounter)
}

// NewGaugeCounter constructs a new StandardGaugeCounter.
func NewGaugeCounter() GaugeCounter {
  if UseNilMetrics {
    return NilGaugeCounter{}
  }
  return &StandardGaugeCounter{StandardCounter{0}}
}

// NewRegisteredCounter constructs and registers a new StandardGaugeCounter.
func NewRegisteredGaugeCounter(name string, r Registry) GaugeCounter {
  c := NewGaugeCounter()
  if nil == r {
    r = DefaultRegistry
  }
  r.Register(name, c)
  return c
}

// CounterSnapshot is a read-only copy of another Counter.
type GaugeCounterSnapshot int64

// Count returns the count at the time the snapshot was taken.
func (c GaugeCounterSnapshot) Count() int64 { return int64(c) }

// Dec panics.
func (GaugeCounterSnapshot) Dec(int64) {
  panic("Dec called on a GaugeCounterSnapshot")
}

// Inc panics.
func (GaugeCounterSnapshot) Inc(int64) {
  panic("Inc called on a GaugeCounterSnapshot")
}

// Snapshot returns the snapshot.
func (c GaugeCounterSnapshot) Snapshot() GaugeCounter { return c }

// NilCounter is a no-op Counter.
type NilGaugeCounter struct {
  NilCounter
}

// Dec is a no-op.
func (NilGaugeCounter) Dec(i int64) {}

// Snapshot is a no-op.
func (NilGaugeCounter) Snapshot() GaugeCounter { return NilGaugeCounter{} }

// NilCounter is a no-op Counter.
type StandardGaugeCounter struct {
  StandardCounter
}

// Dec decrements the counter by the given amount.
func (c *StandardGaugeCounter) Dec(i int64) {
  atomic.AddInt64(&c.count, -i)
}

// Snapshot returns a read-only copy of the counter.
func (c *StandardGaugeCounter) Snapshot() GaugeCounter {
  return GaugeCounterSnapshot(c.Count())
}

