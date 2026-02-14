package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/metrics"

	"github.com/mycreepy/fsdiff/internal/fsdiff"
)

var (
	shouldPrintVersion bool
	shouldPrintMetrics bool

	version = "develop"
	commit  = "HEAD"
	date    = "just now"
)

func main() {
	flag.BoolVar(&shouldPrintVersion, "v", false, "print version")
	flag.BoolVar(&shouldPrintMetrics, "m", false, "print metrics")

	flag.Parse()

	if shouldPrintVersion {
		printVersion()
		return
	}

	walker, err := fsdiff.New()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	err = walker.Run()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	if shouldPrintMetrics {
		printMetrics()
	}
}

func printVersion() {
	fmt.Printf("fsdiff version %s (commit %s) built at %s\n", version, commit, date)
}

func printMetrics() {
	samples := []metrics.Sample{
		{Name: "/sched/goroutines-created:goroutines"},
		{Name: "/cpu/classes/total:cpu-seconds"},
		{Name: "/cpu/classes/gc/total:cpu-seconds"},
		{Name: "/gc/heap/allocs:bytes"},
		{Name: "/gc/heap/allocs:objects"},
	}

	metrics.Read(samples)

	fmt.Println()
	fmt.Printf("goroutines: %s\n", sampleValueString(samples[0].Value))
	fmt.Printf("cpu seconds: %s\n", sampleValueString(samples[1].Value))
	fmt.Printf("gc cpu seconds: %s\n", sampleValueString(samples[2].Value))
	fmt.Printf("gc alloc bytes: %s\n", sampleValueString(samples[3].Value))
	fmt.Printf("gc alloc objects: %s\n", sampleValueString(samples[4].Value))
}

func sampleValueString(value metrics.Value) string {
	switch value.Kind() {
	case metrics.KindUint64:
		return fmt.Sprintf("%d", value.Uint64())
	case metrics.KindFloat64:
		return fmt.Sprintf("%g", value.Float64())
	case metrics.KindBad:
		return "bad"
	default:
		return "unknown"
	}
}
