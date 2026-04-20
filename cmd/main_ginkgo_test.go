package main

import (
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("safeFloat64ToFloat32OrZero", func() {
	DescribeTable("converts valid float32 values",
		func(v float64, e float32) {
			a, ok := safeFloat64ToFloat32OrZero(v)
			Expect(ok).To(BeTrue())
			Expect(a).To(Equal(e))
		},
		Entry("Zero", float64(0), float32(0)),
		Entry("Smallest Positive Non-Zero",
			float64(math.SmallestNonzeroFloat32), float32(math.SmallestNonzeroFloat32)),
		Entry("Smallest Negative Non-Zero",
			float64(-math.SmallestNonzeroFloat32), float32(-math.SmallestNonzeroFloat32)),
		Entry("Max float32", float64(math.MaxFloat32), float32(math.MaxFloat32)),
		Entry("Mix float32", float64(-math.MaxFloat32), float32(-math.MaxFloat32)),
	)

	DescribeTable("returns default value if provided value is not a valid float32",
		func(v float64) {
			a, ok := safeFloat64ToFloat32OrZero(v)
			Expect(ok).To(BeFalse(), "provided value '%v' is recognized as legit", v)
			Expect(a).To(Equal(float32(0)))
		},
		Entry("Above Max Float32", math.Nextafter(float64(math.MaxFloat32), math.MaxFloat64)),
		Entry("Below Min Float32", -(math.Nextafter(float64(math.MaxFloat32), math.MaxFloat64))),
		Entry("Max Float64", float64(math.MaxFloat64)),
		Entry("Min Float64", float64(-math.MaxFloat64)),
		Entry("NaN", math.NaN()),
		Entry("Inf", math.Inf(+1)),
		Entry("-Inf", math.Inf(-1)),
	)
})
