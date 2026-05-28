package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	dataoriented "github.com/gscript/gscript/benchmarks/data_oriented"
)

type report struct {
	Schema         string   `json:"schema"`
	GeneratedAtUTC string   `json:"generated_at_utc"`
	GoVersion      string   `json:"go_version"`
	GOOS           string   `json:"goos"`
	GOARCH         string   `json:"goarch"`
	BackendStatus  string   `json:"backend_status"`
	Results        []result `json:"results"`
}

type result struct {
	Benchmark           string  `json:"benchmark"`
	Layout              string  `json:"layout"`
	Implementation      string  `json:"implementation"`
	BackendStatus       string  `json:"backend_status"`
	N                   int     `json:"n"`
	Steps               int     `json:"steps,omitempty"`
	Repeats             int     `json:"repeats"`
	Seconds             float64 `json:"seconds"`
	NSPerItem           float64 `json:"ns_per_item"`
	ItemsPerSecond      float64 `json:"items_per_second"`
	Checksum            float64 `json:"checksum"`
	ChecksumDescription string  `json:"checksum_description"`
}

func main() {
	particles := flag.Int("particles", dataoriented.DefaultParticles, "particle count")
	steps := flag.Int("steps", dataoriented.DefaultSteps, "integration steps per repeat")
	vectors := flag.Int("vectors", dataoriented.DefaultVectors, "vector count")
	repeats := flag.Int("repeats", 5, "measured repeats per benchmark cell")
	flag.Parse()

	if *particles <= 0 || *steps <= 0 || *vectors <= 0 || *repeats <= 0 {
		fmt.Fprintln(os.Stderr, "particles, steps, vectors, and repeats must be positive")
		os.Exit(2)
	}

	results := []result{
		measureParticleAoS(*particles, *steps, *repeats),
		measureParticleSoA(*particles, *steps, *repeats),
		measureParticleSubsetXAoS(*particles, *steps, *repeats),
		measureParticleSubsetXSoA(*particles, *steps, *repeats),
		measureNormalizeAoS(*vectors, *repeats),
		measureNormalizeSoA(*vectors, *repeats),
	}

	payload := report{
		Schema:         "gscript.data_oriented_benchmark.v1",
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		GoVersion:      runtime.Version(),
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		BackendStatus:  "pending_gscript_typed_array_soa_backend",
		Results:        results,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(1)
	}
}

func measureParticleAoS(n, steps, repeats int) result {
	ps := dataoriented.NewParticlesAoS(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = dataoriented.IntegrateAoS(ps, steps, 0.016)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * steps * repeats)
	return makeResult("particle_integration", "aos", "go_reference_struct_slice", n, steps, repeats, seconds, items, checksum)
}

func measureParticleSoA(n, steps, repeats int) result {
	ps := dataoriented.NewParticlesSoA(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = dataoriented.IntegrateSoA(ps, steps, 0.016)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * steps * repeats)
	return makeResult("particle_integration", "soa", "go_reference_slice_columns", n, steps, repeats, seconds, items, checksum)
}

func measureParticleSubsetXAoS(n, steps, repeats int) result {
	ps := dataoriented.NewParticlesAoS(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = dataoriented.IntegrateXAoS(ps, steps, 0.016)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * steps * repeats)
	return makeResult("particle_subset_x", "aos", "go_reference_struct_slice", n, steps, repeats, seconds, items, checksum)
}

func measureParticleSubsetXSoA(n, steps, repeats int) result {
	ps := dataoriented.NewParticlesSoA(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = dataoriented.IntegrateXSoA(ps, steps, 0.016)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * steps * repeats)
	return makeResult("particle_subset_x", "soa", "go_reference_slice_columns", n, steps, repeats, seconds, items, checksum)
}

func measureNormalizeAoS(n, repeats int) result {
	vs := dataoriented.NewVec3AoS(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = dataoriented.NormalizeAoS(vs)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * repeats)
	return makeResult("vector_normalization", "aos", "go_reference_struct_slice", n, 0, repeats, seconds, items, checksum)
}

func measureNormalizeSoA(n, repeats int) result {
	vs := dataoriented.NewVec3SoA(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = dataoriented.NormalizeSoA(vs)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * repeats)
	return makeResult("vector_normalization", "soa", "go_reference_slice_columns", n, 0, repeats, seconds, items, checksum)
}

func makeResult(name, layout, impl string, n, steps, repeats int, seconds, items, checksum float64) result {
	nsPerItem := seconds * 1e9 / items
	return result{
		Benchmark:           name,
		Layout:              layout,
		Implementation:      impl,
		BackendStatus:       "pending_gscript_typed_array_soa_backend",
		N:                   n,
		Steps:               steps,
		Repeats:             repeats,
		Seconds:             seconds,
		NSPerItem:           nsPerItem,
		ItemsPerSecond:      items / seconds,
		Checksum:            checksum,
		ChecksumDescription: "deterministic numeric checksum; AoS and SoA cells should stay within floating-point tolerance",
	}
}
