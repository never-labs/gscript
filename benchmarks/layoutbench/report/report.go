package report

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"time"

	layoutbench "github.com/never-labs/leia/benchmarks/layoutbench"
)

type Report struct {
	Schema         string   `json:"schema"`
	GeneratedAtUTC string   `json:"generated_at_utc"`
	GoVersion      string   `json:"go_version"`
	GOOS           string   `json:"goos"`
	GOARCH         string   `json:"goarch"`
	BackendStatus  string   `json:"backend_status"`
	Results        []Result `json:"results"`
}

type Result struct {
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

func RunCLI(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("layout_bench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	particles := fs.Int("particles", layoutbench.DefaultParticles, "particle count")
	steps := fs.Int("steps", layoutbench.DefaultSteps, "integration steps per repeat")
	vectors := fs.Int("vectors", layoutbench.DefaultVectors, "vector count")
	repeats := fs.Int("repeats", 5, "measured repeats per benchmark cell")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *particles <= 0 || *steps <= 0 || *vectors <= 0 || *repeats <= 0 {
		fmt.Fprintln(stderr, "particles, steps, vectors, and repeats must be positive")
		return 2
	}

	payload := Build(*particles, *steps, *vectors, *repeats)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintf(stderr, "encode report: %v\n", err)
		return 1
	}
	return 0
}

func Build(particles, steps, vectors, repeats int) Report {
	return Report{
		Schema:         "leia.layout_benchmark.v1",
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		GoVersion:      runtime.Version(),
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		BackendStatus:  "pending_leia_typed_array_soa_backend",
		Results: []Result{
			measureParticleAoS(particles, steps, repeats),
			measureParticleSoA(particles, steps, repeats),
			measureParticleSubsetXAoS(particles, steps, repeats),
			measureParticleSubsetXSoA(particles, steps, repeats),
			measureNormalizeAoS(vectors, repeats),
			measureNormalizeSoA(vectors, repeats),
		},
	}
}

func measureParticleAoS(n, steps, repeats int) Result {
	ps := layoutbench.NewParticlesAoS(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = layoutbench.IntegrateAoS(ps, steps, 0.016)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * steps * repeats)
	return makeResult("particle_integration", "aos", "go_reference_struct_slice", n, steps, repeats, seconds, items, checksum)
}

func measureParticleSoA(n, steps, repeats int) Result {
	ps := layoutbench.NewParticlesSoA(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = layoutbench.IntegrateSoA(ps, steps, 0.016)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * steps * repeats)
	return makeResult("particle_integration", "soa", "go_reference_slice_columns", n, steps, repeats, seconds, items, checksum)
}

func measureParticleSubsetXAoS(n, steps, repeats int) Result {
	ps := layoutbench.NewParticlesAoS(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = layoutbench.IntegrateXAoS(ps, steps, 0.016)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * steps * repeats)
	return makeResult("particle_subset_x", "aos", "go_reference_struct_slice", n, steps, repeats, seconds, items, checksum)
}

func measureParticleSubsetXSoA(n, steps, repeats int) Result {
	ps := layoutbench.NewParticlesSoA(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = layoutbench.IntegrateXSoA(ps, steps, 0.016)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * steps * repeats)
	return makeResult("particle_subset_x", "soa", "go_reference_slice_columns", n, steps, repeats, seconds, items, checksum)
}

func measureNormalizeAoS(n, repeats int) Result {
	vs := layoutbench.NewVec3AoS(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = layoutbench.NormalizeAoS(vs)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * repeats)
	return makeResult("vector_normalization", "aos", "go_reference_struct_slice", n, 0, repeats, seconds, items, checksum)
}

func measureNormalizeSoA(n, repeats int) Result {
	vs := layoutbench.NewVec3SoA(n)
	var checksum float64
	start := time.Now()
	for i := 0; i < repeats; i++ {
		checksum = layoutbench.NormalizeSoA(vs)
	}
	seconds := time.Since(start).Seconds()
	items := float64(n * repeats)
	return makeResult("vector_normalization", "soa", "go_reference_slice_columns", n, 0, repeats, seconds, items, checksum)
}

func makeResult(name, layout, impl string, n, steps, repeats int, seconds, items, checksum float64) Result {
	nsPerItem := seconds * 1e9 / items
	return Result{
		Benchmark:           name,
		Layout:              layout,
		Implementation:      impl,
		BackendStatus:       "pending_leia_typed_array_soa_backend",
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
