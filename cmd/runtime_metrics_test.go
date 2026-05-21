// Copyright 2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// https://pb33f.io

package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatRuntimeMetrics(t *testing.T) {
	formatted := formatRuntimeMetrics(runtimeMetricsSnapshot{
		Elapsed:    2500 * time.Millisecond,
		HeapAlloc:  2 * 1024 * 1024,
		TotalAlloc: 8 * 1024 * 1024,
		Sys:        16 * 1024 * 1024,
		NumGC:      5,
		Goroutines: 12,
	})

	require.Contains(t, formatted, "elapsed 2.5s")
	require.Contains(t, formatted, "heap 2.0 MiB")
	require.Contains(t, formatted, "reserved 16.0 MiB")
	require.Contains(t, formatted, "allocated 8.0 MiB")
	require.Contains(t, formatted, "collections 5")
	require.Contains(t, formatted, "threads 12")
}

func TestRuntimeMetricsMonitorReportsImmediately(t *testing.T) {
	updates := make(chan runtimeMetricsSnapshot, 1)
	monitor := startRuntimeMetricsMonitor(time.Now(), time.Hour, func(snapshot runtimeMetricsSnapshot) {
		updates <- snapshot
	})
	defer monitor.Close()

	select {
	case snapshot := <-updates:
		require.GreaterOrEqual(t, snapshot.Goroutines, 1)
	case <-time.After(time.Second):
		t.Fatal("expected runtime metrics update")
	}
}

func TestRuntimeMetricsMonitorAllowsNilReport(t *testing.T) {
	monitor := startRuntimeMetricsMonitor(time.Now(), time.Hour, nil)
	require.NotNil(t, monitor)
	monitor.Close()
}
