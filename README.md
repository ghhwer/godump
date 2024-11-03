# README
![Coverage](https://img.shields.io/badge/Coverage-94.1%25-brightgreen)

### Table of Contents
- [Overview](#overview)
- [Usage](#usage)
- [Example Usage](#example-usage)
- [Program Output](#program-output)
- [Motivation](#motivation)
- [Caution](#caution)
- [Testing and Contributing](#testing-and-contributing)

### Overview
`godump` is a Go library that aids in testing, debugging, and profiling Go programs by capturing heap and goroutine dumps. With `godump`, you can trigger controlled dumps based on customizable conditions, allowing you to gather valuable profiling data automatically. This is especially beneficial in production environments, where you can enable `godump` dynamically to gain insights without needing to restart your application.

### Usage
To use `godump`, configure either a `DumpHeapConfigs` or `DumpGoroutineConfigs` structure with desired options, then initialize the service using the `NewGoDumpService` function. The service will continue running until the program ends, signaled via a channel.

#### Configuration Structures
- **DumpHeapConfigs**: Allows you to set thresholds for heap usage. Configurable options include:
  - `HeapThresholdBytes`: A heap dump is triggered when the heap size exceeds this byte value.
  - `HeapThresholdPercentage`: A heap dump is triggered when the heap size exceeds this percentage of total memory.

- **DumpGoroutineConfigs**: Allows you to set thresholds for goroutine behavior. Configurable options include:
  - `GoroutineThreshold`: A goroutine dump is triggered when the goroutine count exceeds this number.
  - `GoroutineHangingTimeMs`: Goroutines running longer than this time (in milliseconds) are considered "hanging" and will trigger a dump.

When **both** flags for `GoDumpHeap` and `GoDumpGoroutine` are set to true, `godump` will spawn separate goroutines to monitor heap and goroutine status. If neither flag is set, `godump` remains inactive, ensuring minimal resource usage.

#### Example Usage
Below are example setups for both heap and goroutine dumps. 

##### Heap Dump Example
```go
package main

import (
	"log"
	"sync"

	godump "github.com/ghhwer/godump" // Replace with the actual package name
)

func main() {
	// Set up the heap dump configuration
	hdc := &godump.DumpHeapConfigs{
		HeapThresholdBytes: 1024 * 1024 * 25, // Trigger heap dump if heap exceeds 25 MB
	}

	// Initialize the GoDump service
	gds, err := godump.NewGoDumpService(
		godump.GoDumpConfigs{
			GoDumpHeap: true,      // Enable heap dumping
			GoDumpPath: "./_test", // Set the path to store the dumps
		})
	if err != nil {
		log.Fatal(err)
	}

	// Signal program end
	programChanEnd := make(chan bool)
	wg := &sync.WaitGroup{}

	// Start the service and wait for it to end
	err = gds.Start(programChanEnd, wg)
	if err != nil {
		log.Fatal(err)
	}

	// Your application logic here...

	// Signal program end and wait for the service to complete
	programChanEnd <- true
	wg.Wait()
}
```

##### Goroutine Dump Example
```go
package main

import (
	"log"
	"sync"

	godump "github.com/ghhwer/godump" // Replace with the actual package name
)

func main() {
	// Set up the goroutine dump configuration
	gdc := &godump.DumpGoroutineConfigs{
		GoroutineThreshold: 15, // Trigger goroutine dump if goroutines exceed this count
	}

	// Initialize the GoDump service
	gds, err := godump.NewGoDumpService(
		godump.GoDumpConfigs{
			GoDumpGoroutine: true, // Enable goroutine dumping
			GoDumpPath:      "./_test", // Set the path to store the dumps
		})
	if err != nil {
		log.Fatal(err)
	}

	// Signal program end
	programChanEnd := make(chan bool)
	wg := &sync.WaitGroup{}

	// Start the service and wait for it to end
	err = gds.Start(programChanEnd, wg)
	if err != nil {
		log.Fatal(err)
	}

	// Your application logic here...

	// Signal program end and wait for the service to complete
	programChanEnd <- true
	wg.Wait()
}
```

You can also configure both `GoDumpHeap` and `GoDumpGoroutine` together to monitor both metrics.

### Program Output
The program creates files under the directory specified by `GoDumpPath`:
- **Heap Dumps**: Files named `heapdump-<timestamp>.hprof` contain memory data for analysis using `pprof`.
- **Goroutine Dumps**: Files named `goroutinedump-<timestamp>.txt` provide stack traces of goroutines.

You can analyze heap dumps with Go's native `pprof` tool:
```bash
go tool pprof -http=:8080 heapdump-{timestamp}.hprof
```

Goroutine dump files are readable directly using a text editor or command-line tools.

For an example please check the [example_output](example_output) directory. Where you can find an output of the program that was captured during the execution of the test.

### Motivation
The primary motivation behind `godump` is to provide an easy-to-activate profiling tool, especially valuable for production scenarios. For example, if your application suffers from memory leaks, you can enable `godump` dynamically by setting flags, which enables dump collection without the need for a restart. This collected data can be analyzed to locate potential performance bottlenecks or memory issues.

Additionally, `godump` is designed to avoid resource consumption when not in use. When both `GoDumpHeap` and `GoDumpGoroutine` are disabled, `godump` remains completely inactive, ensuring no impact on application performance.

### Caution
**⚠️ Important Caution**: Use `godump` with caution and at your own risk. While it is designed to be lightweight, enabling it in production can introduce unforeseen overhead and may interact with application state. Be sure to thoroughly test in a staging environment before deploying broadly in production.

### Testing and Contributing
#### Running Tests
To test `godump`, run:
```bash
go test -v -cover .
```
Testing covers typical heap and goroutine dump scenarios. You can also adjust thresholds in the test suite (`godump_test.go`) to simulate specific conditions.

#### Contributing
Contributions are welcome! Follow these guidelines for code contributions:
- Fork the repository and create a new branch.
- Make your changes, thoroughly test them, and submit a pull request to the `develop` branch.
- Direct pull requests to `main` will not be accepted.
