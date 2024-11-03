package godump

import (
	"testing"
)

func TestBadInput(t *testing.T) {
	testCases := []struct {
		name   string
		config *GoDumpConfigs
	}{
		{
			name:   "NIL configs",
			config: nil,
		},
		{
			name: "Bad heapdump configs",
			config: &GoDumpConfigs{
				GoDumpHeap:         true,
				GoDumpGoroutine:    false,
				GoDumpPath:         "./_test",
				WatchdogIntervalMs: 1000,
			},
		},
		{
			name: "Bad goroutine configs",
			config: &GoDumpConfigs{
				GoDumpHeap:         false,
				GoDumpGoroutine:    true,
				GoDumpPath:         "./_test",
				WatchdogIntervalMs: 1000,
			},
		},
		{
			name: "Bad heapdump configs with invalid HeapDumpConfigs",
			config: &GoDumpConfigs{
				GoDumpHeap:         true,
				GoDumpGoroutine:    false,
				GoDumpPath:         "./_test",
				WatchdogIntervalMs: 1000,
				HeapDumpConfigs: &DumpHeapConfigs{
					HeapThresholdBytes:      0,
					HeapThresholdPercentage: 0,
				},
			},
		},
		{
			name: "Bad goroutine configs with invalid GoroutineDumpConfigs",
			config: &GoDumpConfigs{
				GoDumpHeap:         false,
				GoDumpGoroutine:    true,
				GoDumpPath:         "./_test",
				WatchdogIntervalMs: 1000,
				GoroutineDumpConfigs: &DumpGoroutineConfigs{
					GoroutineThreshold:     0,
					GoroutineHangingTimeMs: 0,
				},
			},
		},
		{
			name: "Bad heapdump HeapThresholdPercentage > 1",
			config: &GoDumpConfigs{
				GoDumpHeap:         true,
				GoDumpGoroutine:    false,
				GoDumpPath:         "./_test",
				WatchdogIntervalMs: 1000,
				HeapDumpConfigs: &DumpHeapConfigs{
					HeapThresholdBytes:      0,
					HeapThresholdPercentage: 1.1,
				},
			},
		},
		{
			name: "Bad config WatchdogIntervalMs == 0",
			config: &GoDumpConfigs{
				GoDumpHeap:         true,
				GoDumpGoroutine:    false,
				GoDumpPath:         "./_test",
				WatchdogIntervalMs: 0,
				HeapDumpConfigs: &DumpHeapConfigs{
					HeapThresholdBytes:      0,
					HeapThresholdPercentage: 0.1,
				},
			},
		},
		{
			name: "Bad config GoDumpPath == \"\"",
			config: &GoDumpConfigs{
				GoDumpHeap:         true,
				GoDumpGoroutine:    false,
				GoDumpPath:         "",
				WatchdogIntervalMs: 1000,
				HeapDumpConfigs: &DumpHeapConfigs{
					HeapThresholdBytes:      0,
					HeapThresholdPercentage: 0.1,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewGoDumpService(tc.config)
			if err == nil {
				t.Errorf("Error: Expected error, got nil")
			} else {
				t.Logf("Error: %v", err)
			}
		})
	}
}
