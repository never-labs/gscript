package dataoriented

import "math"

const (
	DefaultParticles = 32768
	DefaultSteps     = 64
	DefaultVectors   = 65536
	DefaultRecords   = 65536
)

type ParticleAoS struct {
	X, Y, Z    float64
	VX, VY, VZ float64
	Mass       float64
}

type ParticlesSoA struct {
	X, Y, Z    []float64
	VX, VY, VZ []float64
	Mass       []float64
}

type Vec3AoS struct {
	X, Y, Z float64
}

type Vec3SoA struct {
	X, Y, Z []float64
}

type RecordAoS struct {
	ID     int64
	Group  int32
	Active bool
	Value  float64
	Weight float64
}

type RecordsSoA struct {
	ID     []int64
	Group  []int32
	Active []bool
	Value  []float64
	Weight []float64
}

func NewParticlesAoS(n int) []ParticleAoS {
	ps := make([]ParticleAoS, n)
	for i := range ps {
		f := float64(i + 1)
		ps[i] = ParticleAoS{
			X:    0.001 * f,
			Y:    0.002 * float64((i%97)+1),
			Z:    0.003 * float64((i%53)+1),
			VX:   0.01 + 0.00001*f,
			VY:   -0.02 + 0.00002*float64(i%31),
			VZ:   0.015 - 0.00001*float64(i%17),
			Mass: 0.5 + 0.001*float64(i%23),
		}
	}
	return ps
}

func NewParticlesSoA(n int) ParticlesSoA {
	ps := ParticlesSoA{
		X: make([]float64, n), Y: make([]float64, n), Z: make([]float64, n),
		VX: make([]float64, n), VY: make([]float64, n), VZ: make([]float64, n),
		Mass: make([]float64, n),
	}
	for i := 0; i < n; i++ {
		f := float64(i + 1)
		ps.X[i] = 0.001 * f
		ps.Y[i] = 0.002 * float64((i%97)+1)
		ps.Z[i] = 0.003 * float64((i%53)+1)
		ps.VX[i] = 0.01 + 0.00001*f
		ps.VY[i] = -0.02 + 0.00002*float64(i%31)
		ps.VZ[i] = 0.015 - 0.00001*float64(i%17)
		ps.Mass[i] = 0.5 + 0.001*float64(i%23)
	}
	return ps
}

func IntegrateAoS(ps []ParticleAoS, steps int, dt float64) float64 {
	for step := 0; step < steps; step++ {
		for i := range ps {
			p := &ps[i]
			ax := -0.125*p.X + 0.0003*p.Mass
			ay := -0.125*p.Y + 0.0002*p.Mass
			az := -0.125*p.Z + 0.0001*p.Mass
			p.VX += ax * dt
			p.VY += ay * dt
			p.VZ += az * dt
			p.X += p.VX * dt
			p.Y += p.VY * dt
			p.Z += p.VZ * dt
		}
	}
	return ChecksumParticlesAoS(ps)
}

func IntegrateSoA(ps ParticlesSoA, steps int, dt float64) float64 {
	for step := 0; step < steps; step++ {
		for i := range ps.X {
			ax := -0.125*ps.X[i] + 0.0003*ps.Mass[i]
			ay := -0.125*ps.Y[i] + 0.0002*ps.Mass[i]
			az := -0.125*ps.Z[i] + 0.0001*ps.Mass[i]
			ps.VX[i] += ax * dt
			ps.VY[i] += ay * dt
			ps.VZ[i] += az * dt
			ps.X[i] += ps.VX[i] * dt
			ps.Y[i] += ps.VY[i] * dt
			ps.Z[i] += ps.VZ[i] * dt
		}
	}
	return ChecksumParticlesSoA(ps)
}

func IntegrateXAoS(ps []ParticleAoS, steps int, dt float64) float64 {
	for step := 0; step < steps; step++ {
		for i := range ps {
			ps[i].X += ps[i].VX * dt
		}
	}
	return ChecksumParticlesAoS(ps)
}

func IntegrateXSoA(ps ParticlesSoA, steps int, dt float64) float64 {
	for step := 0; step < steps; step++ {
		for i := range ps.X {
			ps.X[i] += ps.VX[i] * dt
		}
	}
	return ChecksumParticlesSoA(ps)
}

func ChecksumParticlesAoS(ps []ParticleAoS) float64 {
	sum := 0.0
	for i := range ps {
		weight := float64((i % 7) + 1)
		sum += ps[i].X*weight + ps[i].Y*0.5 + ps[i].Z*0.25 + ps[i].VX*0.125
	}
	return sum
}

func ChecksumParticlesSoA(ps ParticlesSoA) float64 {
	sum := 0.0
	for i := range ps.X {
		weight := float64((i % 7) + 1)
		sum += ps.X[i]*weight + ps.Y[i]*0.5 + ps.Z[i]*0.25 + ps.VX[i]*0.125
	}
	return sum
}

func NewVec3AoS(n int) []Vec3AoS {
	vs := make([]Vec3AoS, n)
	for i := range vs {
		f := float64(i + 1)
		vs[i] = Vec3AoS{X: 1.0 + 0.001*f, Y: 2.0 + 0.002*float64(i%101), Z: 3.0 + 0.003*float64(i%67)}
	}
	return vs
}

func NewVec3SoA(n int) Vec3SoA {
	vs := Vec3SoA{X: make([]float64, n), Y: make([]float64, n), Z: make([]float64, n)}
	for i := 0; i < n; i++ {
		f := float64(i + 1)
		vs.X[i] = 1.0 + 0.001*f
		vs.Y[i] = 2.0 + 0.002*float64(i%101)
		vs.Z[i] = 3.0 + 0.003*float64(i%67)
	}
	return vs
}

func NormalizeAoS(vs []Vec3AoS) float64 {
	for i := range vs {
		inv := 1.0 / math.Sqrt(vs[i].X*vs[i].X+vs[i].Y*vs[i].Y+vs[i].Z*vs[i].Z)
		vs[i].X *= inv
		vs[i].Y *= inv
		vs[i].Z *= inv
	}
	return ChecksumVec3AoS(vs)
}

func NormalizeSoA(vs Vec3SoA) float64 {
	for i := range vs.X {
		inv := 1.0 / math.Sqrt(vs.X[i]*vs.X[i]+vs.Y[i]*vs.Y[i]+vs.Z[i]*vs.Z[i])
		vs.X[i] *= inv
		vs.Y[i] *= inv
		vs.Z[i] *= inv
	}
	return ChecksumVec3SoA(vs)
}

func ChecksumVec3AoS(vs []Vec3AoS) float64 {
	sum := 0.0
	for i := range vs {
		sum += vs[i].X + 0.5*vs[i].Y + 0.25*vs[i].Z
	}
	return sum
}

func ChecksumVec3SoA(vs Vec3SoA) float64 {
	sum := 0.0
	for i := range vs.X {
		sum += vs.X[i] + 0.5*vs.Y[i] + 0.25*vs.Z[i]
	}
	return sum
}

func NewRecordsAoS(n int) []RecordAoS {
	rs := make([]RecordAoS, n)
	for i := range rs {
		rs[i] = RecordAoS{
			ID:     int64(i + 1),
			Group:  int32(i % 19),
			Active: i%3 != 0,
			Value:  1.0 + 0.125*float64(i%257),
			Weight: 0.5 + 0.01*float64(i%31),
		}
	}
	return rs
}

func NewRecordsSoA(n int) RecordsSoA {
	rs := RecordsSoA{
		ID:     make([]int64, n),
		Group:  make([]int32, n),
		Active: make([]bool, n),
		Value:  make([]float64, n),
		Weight: make([]float64, n),
	}
	for i := 0; i < n; i++ {
		rs.ID[i] = int64(i + 1)
		rs.Group[i] = int32(i % 19)
		rs.Active[i] = i%3 != 0
		rs.Value[i] = 1.0 + 0.125*float64(i%257)
		rs.Weight[i] = 0.5 + 0.01*float64(i%31)
	}
	return rs
}

func MakeRecordsSoABuffer(n int) RecordsSoA {
	return RecordsSoA{
		ID:     make([]int64, 0, n),
		Group:  make([]int32, 0, n),
		Active: make([]bool, 0, n),
		Value:  make([]float64, 0, n),
		Weight: make([]float64, 0, n),
	}
}

func SliceRecordsAoS(rs []RecordAoS, offset, length int) float64 {
	return ChecksumRecordsAoS(rs[offset : offset+length])
}

func SliceRecordsSoA(rs RecordsSoA, offset, length int) float64 {
	end := offset + length
	return ChecksumRecordsSoA(RecordsSoA{
		ID:     rs.ID[offset:end],
		Group:  rs.Group[offset:end],
		Active: rs.Active[offset:end],
		Value:  rs.Value[offset:end],
		Weight: rs.Weight[offset:end],
	})
}

func FilterRecordsAoS(src, dst []RecordAoS) ([]RecordAoS, float64) {
	dst = dst[:0]
	for i := range src {
		if keepRecord(src[i].Active, src[i].Group, src[i].Value, src[i].Weight) {
			dst = append(dst, src[i])
		}
	}
	return dst, ChecksumRecordsAoS(dst)
}

func FilterRecordsSoA(src, dst RecordsSoA) (RecordsSoA, float64) {
	dst = resetRecordsSoA(dst)
	for i := range src.ID {
		if keepRecord(src.Active[i], src.Group[i], src.Value[i], src.Weight[i]) {
			dst.ID = append(dst.ID, src.ID[i])
			dst.Group = append(dst.Group, src.Group[i])
			dst.Active = append(dst.Active, src.Active[i])
			dst.Value = append(dst.Value, src.Value[i])
			dst.Weight = append(dst.Weight, src.Weight[i])
		}
	}
	return dst, ChecksumRecordsSoA(dst)
}

func UnzipRecordsAoS(src []RecordAoS, dst RecordsSoA) (RecordsSoA, float64) {
	dst = resetRecordsSoA(dst)
	for i := range src {
		dst.ID = append(dst.ID, src[i].ID)
		dst.Group = append(dst.Group, src[i].Group)
		dst.Active = append(dst.Active, src[i].Active)
		dst.Value = append(dst.Value, src[i].Value)
		dst.Weight = append(dst.Weight, src[i].Weight)
	}
	return dst, ChecksumRecordsSoA(dst)
}

func CopyRecordsAoS(src, dst []RecordAoS) ([]RecordAoS, float64) {
	dst = dst[:0]
	dst = append(dst, src...)
	return dst, ChecksumRecordsAoS(dst)
}

func ChecksumRecordsAoS(rs []RecordAoS) float64 {
	sum := 0.0
	for i := range rs {
		active := 0.0
		if rs[i].Active {
			active = 1.0
		}
		sum += float64(rs[i].ID%97)*0.03125 + float64(rs[i].Group)*0.125 + active + rs[i].Value*rs[i].Weight
	}
	return sum
}

func ChecksumRecordsSoA(rs RecordsSoA) float64 {
	sum := 0.0
	for i := range rs.ID {
		active := 0.0
		if rs.Active[i] {
			active = 1.0
		}
		sum += float64(rs.ID[i]%97)*0.03125 + float64(rs.Group[i])*0.125 + active + rs.Value[i]*rs.Weight[i]
	}
	return sum
}

func keepRecord(active bool, group int32, value, weight float64) bool {
	return active && (group%5 == 0 || value*weight > 18.0)
}

func resetRecordsSoA(rs RecordsSoA) RecordsSoA {
	rs.ID = rs.ID[:0]
	rs.Group = rs.Group[:0]
	rs.Active = rs.Active[:0]
	rs.Value = rs.Value[:0]
	rs.Weight = rs.Weight[:0]
	return rs
}
