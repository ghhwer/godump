package godump

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"syscall"
	"time"
)

/* GoDump is a service that can be embedded in your application to provide easy automatic HEAP DUMP and GOROUTINE DUMPS
 It is useful for debugging memory leaks and goroutine leaks in your application
 It will be able to do that by having watchdogs that will monitor the application and take a heap dump or goroutine dump when the application is in a bad state
 The user will be able to provide different configurations to the service to control the behavior of the watchdogs for example:
	- The user can provide a threshold for the heap usage and the service will take a heap dump when the heap usage exceeds that threshold
	- The user can provide a threshold for the number of goroutines and the service will take a goroutine dump when the number of goroutines exceeds that threshold
*/

type DumpHeapConfigs struct {
	HeapThresholdBytes      uint64
	HeapThresholdPercentage float64
	HeapDumpPrefix          *string
}

type DumpGoroutineConfigs struct {
	GoroutineThreshold     int
	GoroutineHangingTimeMs int
	GoroutineDumpPrefix    *string
}

type GoDumpConfigs struct {
	GoDumpHeap           bool
	GoDumpGoroutine      bool
	GoDumpPath           string
	HeapDumpConfigs      *DumpHeapConfigs
	GoroutineDumpConfigs *DumpGoroutineConfigs
	WatchdogIntervalMs   int
}

func TakeHeapDump(goDumpConfigs *GoDumpConfigs) {
	prefix := "heapdump"
	if goDumpConfigs.HeapDumpConfigs.HeapDumpPrefix != nil {
		prefix = *goDumpConfigs.HeapDumpConfigs.HeapDumpPrefix
	}
	HeapDumpFile := goDumpConfigs.GoDumpPath + "/" + prefix + time.Now().Format("2006-01-02T15:04:05") + ".hprof"
	// Replace double slashes with single slashes
	HeapDumpFile = strings.Replace(HeapDumpFile, "//", "/", -1)
	// Take the heap dump and write it to the file
	f, err := os.Create(HeapDumpFile)
	if err != nil {
		// Could not create the file
		return
	}
	// Write the heap dump to the file
	pprof.WriteHeapProfile(f)
	f.Close()
}
func TakeGoroutineDump(goDumpConfigs *GoDumpConfigs, hangingStacks []GoStackAnalyzerRecord) {
	prefix := "goroutinedump"
	if goDumpConfigs.GoroutineDumpConfigs.GoroutineDumpPrefix != nil {
		prefix = *goDumpConfigs.GoroutineDumpConfigs.GoroutineDumpPrefix
	}
	GoroutineDumpFile := goDumpConfigs.GoDumpPath + "/" + prefix + time.Now().Format("2006-01-02T15:04:05") + ".txt"
	// Replace double slashes with single slashes
	GoroutineDumpFile = strings.Replace(GoroutineDumpFile, "//", "/", -1)
	// Write the goroutine dump to the file
	f, err := os.Create(GoroutineDumpFile)
	if err != nil {
		// Could not create the file
		return
	}
	defer f.Close()

	// Write the stack to the file
	// Write the current time to the file
	f.WriteString("GoRoutine Dump\n---\n")
	f.WriteString("Time: " + time.Now().Format("2006-01-02T15:04:05") + "\n")
	f.WriteString("---\n\n")
	// Write the stack trace to the file
	f.WriteString("Stack Trace:\n")
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	if n > 0 {
		f.WriteString(string(buf[:n]) + "\n")
	}
	// Write the number of goroutines to the file
	f.WriteString("---\n\n")
	f.WriteString("Number of Goroutines: " + fmt.Sprint(runtime.NumGoroutine()) + "\n")
	f.WriteString("Goroutines:\n")
	// Write the goroutine dump to the file
	pprof.Lookup("goroutine").WriteTo(f, 1)
	// Append the hanging goroutines IDs to the end of the file
	if len(hangingStacks) > 0 {
		f.WriteString("---\n\n")
		// Write the hanging goroutines to the file
		f.WriteString("\nHanging Goroutines Detected:\n")
		f.WriteString("Number of Hanging Goroutines: " + fmt.Sprint(len(hangingStacks)) + "\n")
		f.WriteString("Considered Hanging time (ms): " + fmt.Sprint(goDumpConfigs.GoroutineDumpConfigs.GoroutineHangingTimeMs) + "\n")
		for _, stack := range hangingStacks {
			// Convert the identifier to a string in hexadecimal
			eachStack := make(map[int]string)
			// Iterate through the stack and for each element we want to have 0x + the hexadecimal representation of the element
			for i := 0; i < len(stack.LastStacks.Stack0); i++ {
				eachStack[i] = fmt.Sprintf("0x%x", stack.LastStacks.Stack0[i])
			}
			// We now join the values of the map with a comma
			valuesStack := make([]string, len(eachStack))
			for i := 0; i < len(eachStack); i++ {
				valuesStack[i] = eachStack[i]
			}
			identifierString := strings.Join(valuesStack, ",")
			// Write the stack to the file
			f.WriteString(" * Last Change: " + stack.LastChange.Format("2006-01-02T15:04:05"))
			f.WriteString(" * Last Mesure: " + stack.CurrentMesure.Format("2006-01-02T15:04:05"))
			f.WriteString(" (Stack) -> [" + identifierString + "]\n")
		}
		// Close the file
	}
	f.Close()
}
func compareStacks(stack1, stack2 runtime.StackRecord) bool {
	return reflect.DeepEqual(stack1, stack2)
}

func getAvailableMemory() (uint64, error) {
	var info syscall.Sysinfo_t
	err := syscall.Sysinfo(&info)
	if err != nil {
		return 0, err
	}
	return info.Totalram * uint64(info.Unit), nil
}

/*
	 == Watchdogs ==
		We split the watchdogs into two functions to make it more performant we don't want to have to always check if we should be looking at bytes or percentage for the heap threshold
		Instead we will have two watchdogs, one for bytes and one for percentage and select the one to run based on the configuration
*/
func WatchHeapBytes(gd GoDumpService, ApplicationStopChannel chan bool, SafeExitWg *sync.WaitGroup) {
	// start watching the heap
	SafeExitWg.Add(1) // add one to the wait group
	LastMemStats := &runtime.MemStats{}
	// get the initial memory stats
	runtime.ReadMemStats(LastMemStats)
	for {
		select {
		case <-ApplicationStopChannel:
			// stop the watchdog
			SafeExitWg.Done()
			return
		case <-time.After(time.Duration(gd.configs.WatchdogIntervalMs) * time.Millisecond):
			// check the heap usage
			// if the heap usage exceeds the threshold, take a heap dump
			var CurrentMemStats runtime.MemStats
			runtime.ReadMemStats(&CurrentMemStats)
			// Check the heap allocation
			if CurrentMemStats.Alloc > uint64(gd.configs.HeapDumpConfigs.HeapThresholdBytes) {
				// take a heap dump
				TakeHeapDump(gd.configs)
			}
			// update the last memory stats
			LastMemStats = &CurrentMemStats
		}
	}
}

func WatchHeapPercentage(gd GoDumpService, ApplicationStopChannel chan bool, SafeExitWg *sync.WaitGroup, AvailableSystemMemory uint64) {
	// start watching the heap
	SafeExitWg.Add(1) // add one to the wait group
	LastMemStats := &runtime.MemStats{}
	// get the initial memory stats
	runtime.ReadMemStats(LastMemStats)
	for {
		select {
		case <-ApplicationStopChannel:
			// stop the watchdog
			SafeExitWg.Done()
			return
		case <-time.After(time.Duration(gd.configs.WatchdogIntervalMs) * time.Millisecond):
			// check the heap usage
			// if the heap usage exceeds the threshold, take a heap dump
			var CurrentMemStats runtime.MemStats
			runtime.ReadMemStats(&CurrentMemStats)
			// Check the heap allocation
			if CurrentMemStats.Alloc > uint64(float64(AvailableSystemMemory)*float64(gd.configs.HeapDumpConfigs.HeapThresholdPercentage)) {
				// take a heap dump
				TakeHeapDump(gd.configs)
			}
			// update the last memory stats
			LastMemStats = &CurrentMemStats
		}
	}
}

func WatchGoroutines(gd GoDumpService, ApplicationStopChannel chan bool, SafeExitWg *sync.WaitGroup) {
	// start watching the goroutines
	SafeExitWg.Add(1) // add one to the wait group
	for {
		select {
		case <-ApplicationStopChannel:
			// stop the watchdog
			SafeExitWg.Done()
			return
		case <-time.After(time.Duration(gd.configs.WatchdogIntervalMs) * time.Millisecond):
			// check the number of goroutines
			// if the number of goroutines exceeds the threshold, take a goroutine dump
			if runtime.NumGoroutine() > gd.configs.GoroutineDumpConfigs.GoroutineThreshold {
				// take a goroutine dump
				TakeGoroutineDump(gd.configs, []GoStackAnalyzerRecord{})
			}
		}
	}
}

type GoStackAnalyzerRecord struct {
	// the stack analyzer will be used to analyze the stack traces of the goroutines
	LastStacks    runtime.StackRecord
	CurrentStacks runtime.StackRecord
	CurrentMesure time.Time
	LastChange    time.Time
}

func WatchGoroutinesHanging(gd GoDumpService, ApplicationStopChannel chan bool, SafeExitWg *sync.WaitGroup) {
	// start watching the goroutines
	SafeExitWg.Add(1) // add one to the wait group
	GoStackAnalyzerRecords := make(map[[32]uintptr]*GoStackAnalyzerRecord)
	for {
		select {
		case <-ApplicationStopChannel:
			// stop the watchdog
			SafeExitWg.Done()
			return
		case <-time.After(time.Duration(gd.configs.WatchdogIntervalMs) * time.Millisecond):
			// We map the goroutine id to the stack trace
			// if the goroutine is still running, we update the stack trace in the map
			// if a goroutine is not running anymore we remove it from the map
			// if a goroutine has been having the same stack trace for a long time, we take a goroutine dump

			// Get the stack and the goroutine id for all the goroutines
			goroutines := make([]runtime.StackRecord, runtime.NumGoroutine())
			idsPresentOnThisRun := make(map[[32]uintptr]bool)
			n, ok := runtime.GoroutineProfile(goroutines)
			if !ok {
				// Could not get the goroutines
				continue
			}
			// Get the current time
			currentTime := time.Now()
			// Loop through the goroutines
			for i := 0; i < n; i++ {
				// Get the goroutine id
				goid := goroutines[i].Stack0
				// Get the stack trace
				stack := goroutines[i]
				// Check if the goroutine is in the map
				if _, ok := GoStackAnalyzerRecords[goid]; !ok {
					// The goroutine is not in the map, add it
					GoStackAnalyzerRecords[goid] = &GoStackAnalyzerRecord{
						LastStacks:    runtime.StackRecord{},
						CurrentStacks: stack,
						CurrentMesure: currentTime,
						LastChange:    currentTime,
					}
				} else {
					// The goroutine is in the map, update it
					GoStackAnalyzerRecords[goid].LastStacks = GoStackAnalyzerRecords[goid].CurrentStacks
					GoStackAnalyzerRecords[goid].CurrentStacks = stack
					GoStackAnalyzerRecords[goid].CurrentMesure = currentTime
				}
				idsPresentOnThisRun[goid] = true
			}
			// Check if there are goroutines that are not present anymore
			toDelete := [][32]uintptr{}
			for goid := range GoStackAnalyzerRecords {
				if _, ok := idsPresentOnThisRun[goid]; !ok {
					// The goroutine is not present anymore, remove it
					toDelete = append(toDelete, goid)
				}
			}
			// Remove the goroutines that are not present anymore
			for _, goid := range toDelete {
				delete(GoStackAnalyzerRecords, goid)
			}
			stacksRemainedTheSameForTooLong := []GoStackAnalyzerRecord{}
			// Check if any of the goroutines has the same stack trace for too long
			for _, record := range GoStackAnalyzerRecords {
				if compareStacks(record.LastStacks, record.CurrentStacks) {
					// The stack trace has not changed
					if currentTime.Sub(record.LastChange) > time.Duration(gd.configs.GoroutineDumpConfigs.GoroutineHangingTimeMs)*time.Millisecond {
						// The stack trace has not changed for too long
						stacksRemainedTheSameForTooLong = append(stacksRemainedTheSameForTooLong, *record)
					}
				} else {
					// The stack trace has changed
					record.LastChange = currentTime
				}
			}
			if len(stacksRemainedTheSameForTooLong) > 0 {
				// take a goroutine dump
				TakeGoroutineDump(gd.configs, stacksRemainedTheSameForTooLong)
			}
		}
	}
}

type GoDumpService struct {
	configs *GoDumpConfigs
}

func NewGoDumpService(configs *GoDumpConfigs) (*GoDumpService, error) {
	if configs == nil {
		return nil, fmt.Errorf("configs cannot be nil")
	}
	if configs.GoDumpHeap && configs.HeapDumpConfigs == nil {
		return nil, fmt.Errorf("the variable 'HeapDumpConfigs' cannot be nil when GoDumpHeap is true")
	}
	if configs.GoDumpGoroutine && configs.GoroutineDumpConfigs == nil {
		return nil, fmt.Errorf("the variable 'GoroutineDumpConfigs' cannot be nil when GoDumpGoroutine is true")
	}
	// Check configs
	if configs.GoDumpHeap {
		if configs.HeapDumpConfigs.HeapThresholdBytes == 0 && configs.HeapDumpConfigs.HeapThresholdPercentage == 0 {
			return nil, fmt.Errorf("the variable 'HeapThresholdBytes' and 'HeapThresholdPercentage' cannot be both 0")
		} else if configs.HeapDumpConfigs.HeapThresholdPercentage > 1 {
			return nil, fmt.Errorf("the variable 'HeapThresholdPercentage' cannot be greater than 1")
		}
	}
	if configs.GoDumpGoroutine {
		if configs.GoroutineDumpConfigs.GoroutineThreshold == 0 && configs.GoroutineDumpConfigs.GoroutineHangingTimeMs == 0 {
			return nil, fmt.Errorf("the variable 'GoroutineThreshold' and GoroutineHangingTimeMs' cannot be both 0")
		}
	}
	if configs.WatchdogIntervalMs == 0 {
		return nil, fmt.Errorf("the variable 'WatchdogIntervalMs' cannot be 0")
	}
	if configs.GoDumpPath == "" {
		return nil, fmt.Errorf("the variable 'GoDumpPath' cannot be empty")
	}
	return &GoDumpService{
		configs: configs,
	}, nil
}

func (gd *GoDumpService) Start(ApplicationStopChannel chan bool, SafeExitWg *sync.WaitGroup) error {
	// start the watchdogs
	AvailableSystemMemory, err := getAvailableMemory()
	if err != nil {
		return err
	}
	if gd.configs.GoDumpHeap {
		if gd.configs.HeapDumpConfigs.HeapThresholdBytes > 0 {
			go WatchHeapBytes(*gd, ApplicationStopChannel, SafeExitWg)
		}
		if gd.configs.HeapDumpConfigs.HeapThresholdPercentage > 0 {
			go WatchHeapPercentage(*gd, ApplicationStopChannel, SafeExitWg, AvailableSystemMemory)
		}
	}
	if gd.configs.GoDumpGoroutine {
		if gd.configs.GoroutineDumpConfigs.GoroutineThreshold > 0 {
			go WatchGoroutines(*gd, ApplicationStopChannel, SafeExitWg)
		}
		if gd.configs.GoroutineDumpConfigs.GoroutineHangingTimeMs > 0 {
			go WatchGoroutinesHanging(*gd, ApplicationStopChannel, SafeExitWg)
		}
	}
	return nil
}
