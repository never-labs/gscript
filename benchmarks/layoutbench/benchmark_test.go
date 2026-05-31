package layoutbench

import "testing"

var sinkFloat float64

func BenchmarkParticleIntegrationAoS(b *testing.B) {
	ps := NewParticlesAoS(DefaultParticles)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFloat = IntegrateAoS(ps, DefaultSteps, 0.016)
	}
}

func BenchmarkParticleIntegrationSoA(b *testing.B) {
	ps := NewParticlesSoA(DefaultParticles)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFloat = IntegrateSoA(ps, DefaultSteps, 0.016)
	}
}

func BenchmarkParticleSubsetXAoS(b *testing.B) {
	ps := NewParticlesAoS(DefaultParticles)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFloat = IntegrateXAoS(ps, DefaultSteps, 0.016)
	}
}

func BenchmarkParticleSubsetXSoA(b *testing.B) {
	ps := NewParticlesSoA(DefaultParticles)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFloat = IntegrateXSoA(ps, DefaultSteps, 0.016)
	}
}

func BenchmarkVectorNormalizeAoS(b *testing.B) {
	vs := NewVec3AoS(DefaultVectors)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFloat = NormalizeAoS(vs)
	}
}

func BenchmarkVectorNormalizeSoA(b *testing.B) {
	vs := NewVec3SoA(DefaultVectors)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFloat = NormalizeSoA(vs)
	}
}

func BenchmarkRecordSliceAoS(b *testing.B) {
	rs := NewRecordsAoS(DefaultRecords)
	offset := DefaultRecords / 4
	length := DefaultRecords / 2
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFloat = SliceRecordsAoS(rs, offset, length)
	}
}

func BenchmarkRecordSliceSoA(b *testing.B) {
	rs := NewRecordsSoA(DefaultRecords)
	offset := DefaultRecords / 4
	length := DefaultRecords / 2
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFloat = SliceRecordsSoA(rs, offset, length)
	}
}

func BenchmarkRecordFilterAoS(b *testing.B) {
	src := NewRecordsAoS(DefaultRecords)
	dst := make([]RecordAoS, 0, DefaultRecords)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var checksum float64
		dst, checksum = FilterRecordsAoS(src, dst)
		sinkFloat = checksum
	}
}

func BenchmarkRecordFilterSoA(b *testing.B) {
	src := NewRecordsSoA(DefaultRecords)
	dst := MakeRecordsSoABuffer(DefaultRecords)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var checksum float64
		dst, checksum = FilterRecordsSoA(src, dst)
		sinkFloat = checksum
	}
}

func BenchmarkRecordUnzipAoS(b *testing.B) {
	src := NewRecordsAoS(DefaultRecords)
	dst := MakeRecordsSoABuffer(DefaultRecords)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var checksum float64
		dst, checksum = UnzipRecordsAoS(src, dst)
		sinkFloat = checksum
	}
}

func BenchmarkRecordCopyAoS(b *testing.B) {
	src := NewRecordsAoS(DefaultRecords)
	dst := make([]RecordAoS, 0, DefaultRecords)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var checksum float64
		dst, checksum = CopyRecordsAoS(src, dst)
		sinkFloat = checksum
	}
}
