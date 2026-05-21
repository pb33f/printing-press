// Copyright 2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package cmd

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

var runtimeMetricsInterval = time.Second

type runtimeMetricsSnapshot struct {
	Elapsed    time.Duration
	HeapAlloc  uint64
	TotalAlloc uint64
	Sys        uint64
	NumGC      uint32
	Goroutines int
}

type runtimeMetricsMonitor struct {
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

func startRuntimeMetricsMonitor(start time.Time, interval time.Duration, report func(runtimeMetricsSnapshot)) *runtimeMetricsMonitor {
	if interval <= 0 {
		interval = runtimeMetricsInterval
	}
	monitor := &runtimeMetricsMonitor{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go func() {
		defer close(monitor.done)
		reportRuntimeMetrics(start, report)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-monitor.stop:
				return
			case <-ticker.C:
				reportRuntimeMetrics(start, report)
			}
		}
	}()
	return monitor
}

func (m *runtimeMetricsMonitor) Close() {
	if m == nil {
		return
	}
	m.stopOnce.Do(func() {
		close(m.stop)
		<-m.done
	})
}

func reportRuntimeMetrics(start time.Time, report func(runtimeMetricsSnapshot)) {
	if report == nil {
		return
	}
	report(captureRuntimeMetrics(start))
}

func captureRuntimeMetrics(start time.Time) runtimeMetricsSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return runtimeMetricsSnapshot{
		Elapsed:    time.Since(start),
		HeapAlloc:  mem.HeapAlloc,
		TotalAlloc: mem.TotalAlloc,
		Sys:        mem.Sys,
		NumGC:      mem.NumGC,
		Goroutines: runtime.NumGoroutine(),
	}
}

func formatRuntimeMetrics(snapshot runtimeMetricsSnapshot) string {
	return fmt.Sprintf("elapsed %s · heap %s · reserved %s · allocated %s · collections %d · threads %d",
		roundDuration(snapshot.Elapsed),
		humanRuntimeBytes(snapshot.HeapAlloc),
		humanRuntimeBytes(snapshot.Sys),
		humanRuntimeBytes(snapshot.TotalAlloc),
		snapshot.NumGC,
		snapshot.Goroutines,
	)
}

func humanRuntimeBytes(size uint64) string {
	const maxInt64 = uint64(1<<63 - 1)
	if size > maxInt64 {
		size = maxInt64
	}
	return humanBytes(int64(size))
}
