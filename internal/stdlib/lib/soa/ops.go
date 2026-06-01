package soa

// KernelArgs is the runtime-independent argument payload for two-column numeric
// SOA kernels such as addScaled.
type KernelArgs struct {
	Dst   string
	Src   string
	Scale float64
}

// NewKernelArgs records numeric kernel arguments after the runtime has adapted
// script values into plain Go values.
func NewKernelArgs(dst, src string, scale float64) KernelArgs {
	return KernelArgs{Dst: dst, Src: src, Scale: scale}
}

// AffineArgs is the runtime-independent argument payload for affine kernels.
type AffineArgs struct {
	KernelArgs
	Bias float64
}

// NewAffineArgs records affine kernel arguments after script value adaptation.
func NewAffineArgs(dst, src string, scale, bias float64) AffineArgs {
	return AffineArgs{KernelArgs: NewKernelArgs(dst, src, scale), Bias: bias}
}

// AffineTerm is one term in soa.affineMany after table/value adaptation.
type AffineTerm struct {
	Dst   string
	Src   string
	Scale float64
	Bias  float64
}

// NewAffineTerm records one affineMany term after script value adaptation.
func NewAffineTerm(dst, src string, scale, bias float64) AffineTerm {
	return AffineTerm{Dst: dst, Src: src, Scale: scale, Bias: bias}
}

// DefaultAffineBias preserves soa.affineMany's optional bias default.
func DefaultAffineBias(hasBias bool, bias float64) float64 {
	if !hasBias {
		return 0
	}
	return bias
}

// SliceRange converts SOA's 1-based inclusive public slice range into the
// runtime's zero-based start and exclusive-style end argument pair.
func SliceRange(firstOneBased, lastInclusive int64) (start int, end int) {
	return int(firstOneBased - 1), int(lastInclusive)
}
