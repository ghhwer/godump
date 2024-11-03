package godump

import (
	"math/rand"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

// Enviroment setup functions
func ResetEnvironment() error {
	// Force the garbage collector to run to free up memory from any other tests
	runtime.GC()
	// We start by clearing the folder _test if it exists
	err := os.RemoveAll("./_test")
	if err != nil {
		return err
	}
	// We then create the folder _test
	err = os.MkdirAll("./_test", 0755)
	if err != nil {
		return err
	}
	return nil
}

func CountFilesInFolder(folderPath string) (int, error) {
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// --- Heapdump tests

func LoadUpHeap(SizeInBytes int, chanTest chan bool, sleepInSeconds int) {
	print("Loading the memory with ", SizeInBytes, " bytes\n")
	// Fill up the memory with ones and zeros
	// Allocate memory
	data := make([]byte, SizeInBytes)
	// Fill up the memory with random data
	for i := 0; i < SizeInBytes; i++ {
		data[i] = byte(rand.Intn(2))
	}
	// Signal that the memory is filled up
	time.Sleep(time.Duration(sleepInSeconds) * time.Second)
	chanTest <- true
}

func HeapDumpFillupMemoryTest(goHeapDumpConfigs *DumpHeapConfigs, TestWatchDogIntervalSeconds int, MemoryFileCreationSizeMB int) (int, error) {
	// We start by clearing the folder _test if it exists
	err := ResetEnvironment()
	if err != nil {
		return 0, err
	}
	// Heapdump tests
	goDumpConfs := &GoDumpConfigs{
		GoDumpHeap:         true,
		GoDumpGoroutine:    false,
		GoDumpPath:         "./_test",
		WatchdogIntervalMs: uint64(1000 * TestWatchDogIntervalSeconds), // 1s
		HeapDumpConfigs:    goHeapDumpConfigs,
	}
	gds, err := NewGoDumpService(goDumpConfs)
	if err != nil {
		return 0, err
	}
	wg := sync.WaitGroup{}
	programChanEnd := make(chan bool)
	err = gds.Start(programChanEnd, &wg)
	if err != nil {
		return 0, err
	}
	creationDone := make(chan bool)
	// Start filling up the memory 10MB (This should not trigger the heapdump)
	go LoadUpHeap(1024*1024*MemoryFileCreationSizeMB, creationDone, TestWatchDogIntervalSeconds*2)
	// Wait for the memory to be filled up
	<-creationDone
	// Wait for 5s so that the watchdog can run to completion
	time.Sleep(1000 * 5)
	// Check if the heapdump file was created
	filesCount, err := CountFilesInFolder("./_test")
	if err != nil {
		return 0, err
		// We can fail the test here
	}
	// Signal that the program has ended
	programChanEnd <- true
	wg.Wait()
	err = ResetEnvironment()
	if err != nil {
		return 0, err
	}
	return filesCount, nil
}

func TestHeapdumpBytesNoTriggerNoFile(t *testing.T) {
	hdc := &DumpHeapConfigs{
		HeapThresholdBytes:      1024 * 1024 * 25,
		HeapThresholdPercentage: 0,
	}
	fileCreatedCount, err := HeapDumpFillupMemoryTest(hdc, 5, 10)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if fileCreatedCount != 0 {
		t.Errorf("Error: Expected 0 files, got %v", fileCreatedCount)
	} else {
		println("Heapdump file not created [OK]")
	}
}

func TestHeapdumpBytesTriggerFile(t *testing.T) {
	hdc := &DumpHeapConfigs{
		HeapThresholdBytes:      1024 * 1024 * 25,
		HeapThresholdPercentage: 0,
	}
	fileCreatedCount, err := HeapDumpFillupMemoryTest(hdc, 5, 50)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if fileCreatedCount < 1 {
		t.Errorf("Error: Expected at least 1 file, got %v", fileCreatedCount)
	} else {
		println("Heapdump file created [OK]")
	}
}

func TestHeapdumpBytesTriggerFilePercentage(t *testing.T) {
	// Define the size of memory of this machine
	AvailableMemInBytes, err := getAvailableMemory()
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Define what 50MB represents in percentage of the available memory
	AvailableMemInMB := AvailableMemInBytes / (1024 * 1024)
	TargetObjSize := int(float64(AvailableMemInMB)*0.05) + (1) // + 1MB
	println("Available memory:", AvailableMemInMB, "MB")
	println("File has to be:", TargetObjSize, "MB to trigger the heapdump with 5 percent")

	hdc := &DumpHeapConfigs{
		HeapThresholdBytes:      0,
		HeapThresholdPercentage: 0.05,
	}
	fileCreatedCount, err := HeapDumpFillupMemoryTest(hdc, 5, TargetObjSize)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if fileCreatedCount < 1 {
		t.Errorf("Error: Expected at least 1 file, got %v", fileCreatedCount)
	} else {
		println("Heapdump file created [OK]")
	}
}

func TestHeapdumpBytesNoTriggerFilePercentage(t *testing.T) {
	// Define the size of memory of this machine
	AvailableMemInBytes, err := getAvailableMemory()
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Define what 50MB represents in percentage of the available memory
	AvailableMemInMB := AvailableMemInBytes / (1024 * 1024)
	TargetObjSize := int((float64(AvailableMemInMB) * 0.05) - (float64(AvailableMemInMB) * 0.025)) // - 2.5%
	if TargetObjSize < 0 {
		t.Errorf("Error: TargetObjSize is negative, either this is a potato or something is wrong with the getAvailableMemory function")
	}
	println("Available memory:", AvailableMemInMB, "MB")
	println("File has to be:", TargetObjSize, "MB to not trigger the heapdump with 5 percent")

	hdc := &DumpHeapConfigs{
		HeapThresholdBytes:      0,
		HeapThresholdPercentage: 0.05,
	}
	fileCreatedCount, err := HeapDumpFillupMemoryTest(hdc, 5, TargetObjSize)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if fileCreatedCount != 0 {
		t.Errorf("Error: Expected 0 files, got %v", fileCreatedCount)
	} else {
		println("Heapdump file not created [OK]")
	}
}

// --- Goroutine tests
func GoroutineHangingTask(wgTests *sync.WaitGroup, sleepInSeconds int) {
	time.Sleep(time.Duration(sleepInSeconds) * time.Second)
	wgTests.Done()
}

func GoroutinesTests(goroutineDumpConfigs *DumpGoroutineConfigs, TestWatchDogIntervalSeconds int, numGoRoutinesRunning int, timePerGoRoutine int) (int, error) {
	// We start by clearing the folder _test if it exists
	err := ResetEnvironment()
	if err != nil {
		return 0, err
	}
	// Goroutine tests
	goDumpConfs := &GoDumpConfigs{
		GoDumpHeap:           false,
		GoDumpGoroutine:      true,
		GoDumpPath:           "./_test",
		WatchdogIntervalMs:   uint64(1000 * TestWatchDogIntervalSeconds), // 1s
		GoroutineDumpConfigs: goroutineDumpConfigs,
	}
	gds, err := NewGoDumpService(goDumpConfs)
	if err != nil {
		return 0, err
	}
	wg := sync.WaitGroup{}
	programChanEnd := make(chan bool)
	err = gds.Start(programChanEnd, &wg)
	if err != nil {
		return 0, err
	}
	wgHanging := sync.WaitGroup{}
	// Start filling up the memory 10MB (This should not trigger the heapdump)
	for i := 0; i < numGoRoutinesRunning; i++ {
		wgHanging.Add(1)
		go GoroutineHangingTask(&wgHanging, timePerGoRoutine)
	}
	println("Waiting for the goroutines to finish")
	// Wait for all the goroutines to finish
	wgHanging.Wait()
	println("All goroutines finished")
	// Check if the heapdump file was created
	filesCount, err := CountFilesInFolder("./_test")
	if err != nil {
		return 0, err
		// We can fail the test here
	}
	// Signal that the program has ended
	programChanEnd <- true
	wg.Wait()
	//err = ResetEnvironment()
	//if err != nil {
	//	return 0, err
	//}
	return filesCount, nil
}

func TestGoroutineNoTriggerNoFile(t *testing.T) {
	gdc := &DumpGoroutineConfigs{
		GoroutineThreshold: 15,
	}
	fileCreatedCount, err := GoroutinesTests(gdc, 1, 10, 2) // 10 goroutines running for 2 seconds
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if fileCreatedCount != 0 {
		t.Errorf("Error: Expected 0 files, got %v", fileCreatedCount)
	} else {
		println("Goroutine file not created [OK]")
	}
}

func TestGoroutineTriggerFile(t *testing.T) {
	gdc := &DumpGoroutineConfigs{
		GoroutineThreshold: 15,
	}
	fileCreatedCount, err := GoroutinesTests(gdc, 1, 20, 2) // 20 goroutines running
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if fileCreatedCount < 1 {
		t.Errorf("Error: Expected at least 1 file, got %v", fileCreatedCount)
	} else {
		println("Goroutine file created [OK]")
	}
}

func TestGoroutineHangingNoTriggerFile(t *testing.T) {
	gdc := &DumpGoroutineConfigs{
		GoroutineHangingTimeMs: 1000 * 10, // 5s
	}
	fileCreatedCount, err := GoroutinesTests(gdc, 1, 2, 1) // 2 goroutines running
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if fileCreatedCount != 0 {
		t.Errorf("Error: Expected 0 files, got %v", fileCreatedCount)
	} else {
		println("Goroutine file not created [OK]")
	}
}

func TestGoroutineHangingTriggerFile(t *testing.T) {
	gdc := &DumpGoroutineConfigs{
		GoroutineHangingTimeMs: 1000 * 10, // 10s
	}
	fileCreatedCount, err := GoroutinesTests(gdc, 5, 2, 20) // 2 goroutines running
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if fileCreatedCount < 1 {
		t.Errorf("Error: Expected at least 1 file, got %v", fileCreatedCount)
	} else {
		println("Goroutine file created [OK]")
	}
}

// --- Bad input tests
