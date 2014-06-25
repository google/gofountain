// Copyright 2014 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fountain

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-5
}

func TestSolitonDistribution(t *testing.T) {
	// Test several randomly chosen values of n.
	tests := make([]int, 100)
	for i := range tests {
		tests[i] = rand.Intn(1e3) + 1 // Add 1 so that n will never be 0.
	}
	// Plus a couple of specific ones.
	tests = append(tests, 1, 10)

	for _, n := range tests {
		cdf := solitonDistribution(n)
		if len(cdf) != n+1 {
			t.Errorf("n=%d: Wrong length CDF: %d", n, len(cdf))
			t.Log("CDF=", cdf)
		}

		if !almostEqual(cdf[n], 1) {
			t.Errorf("n=%d: CDF[max] = %f, should be 1", n, cdf[n])
			t.Log("CDF=", cdf)
		}

		if !almostEqual(cdf[0], 0) {
			t.Errorf("n=%d: CDF[0] = %f, should be 0.0", n, cdf[0])
			t.Log("CDF=", cdf)
		}

		if !almostEqual(cdf[1], 1/float64(n)) {
			t.Errorf("n=%d: CDF[1] = %f, should be 1/n (=%f)", n, cdf[1], 1/float64(n))
			t.Log("CDF=", cdf)
		}
	}
}

func TestRobustSolitonDistribution(t *testing.T) {
	cdf := robustSolitonDistribution(10, 8, 0.1)
	if len(cdf) != 11 {
		t.Errorf("Wrong length CDF: %d, should be 11", len(cdf))
		t.Log("CDF=", cdf)
	}

	if !almostEqual(cdf[0], 0) {
		t.Errorf("CDF[0] = %f, should be 0.0", cdf[0])
		t.Log("CDF=", cdf)
	}

	if !almostEqual(cdf[1], .287474) {
		t.Errorf("CDF[1] = %f, should be 0.287474", cdf[1])
		t.Log("CDF=", cdf)
	}

	if !almostEqual(cdf[len(cdf)-1], 1) {
		t.Errorf("CDF[max] = %f, should be very nearly 1", cdf[9])
		t.Log("CDF=", cdf)
	}

	if (cdf[8]-cdf[7] < cdf[7]-cdf[6]) ||
		(cdf[8]-cdf[7] < cdf[9]-cdf[8]) {
		t.Errorf("CDF must have mode at M position(8): %v", cdf)
		t.Logf("PDF[7, 8, 9] = %v, %v, %v", cdf[7]-cdf[6], cdf[8]-cdf[7], cdf[9]-cdf[8])
		t.Log("CDF=", cdf)
	}
}

func TestOnlineSolitonDistribution(t *testing.T) {
	cdf := onlineSolitonDistribution(0.1)
	if len(cdf) != 118 {
		t.Errorf("Wrong length CDF: %d. Should be 118", len(cdf))
		t.Log("CDF=", cdf)
	}

	if (cdf[2] - cdf[1]) < (cdf[1] - cdf[0]) {
		t.Errorf("CDF should have mode in second position: %v", cdf[0:5])
		t.Log("CDF=", cdf)
	}

	if !almostEqual(cdf[0], 0) {
		t.Errorf("CDF[0] = %f, should be 0.0", cdf[0])
		t.Log("CDF=", cdf)
	}

	if !almostEqual(cdf[len(cdf)-1], 1) {
		t.Errorf("CDF[max] = %f, should be very nearly 1", cdf[len(cdf)-1])
		t.Log("CDF=", cdf)
	}

	cdf = onlineSolitonDistribution(0.01)
	if len(cdf) != 2116 {
		t.Errorf("Wrong length CDF for 0.0: %d, should be 2116", len(cdf))
	}
}

func TestPickDegree(t *testing.T) {
	cdf := onlineSolitonDistribution(0.25)
	random := rand.New(rand.NewSource(25))
	var numLessThanFive int
	for i := 0; i < 100; i++ {
		d := pickDegree(random, cdf)
		if d < 1 || d > len(cdf)-1 {
			t.Errorf("Degree out of bounds: %d", d)
		}
		if d < 5 {
			numLessThanFive++
		}
	}
	if numLessThanFive < 80 {
		t.Errorf("Too many large degrees: %d, should be < 80", numLessThanFive)
	}
}

func TestSampleUniform(t *testing.T) {
	random := rand.New(rand.NewSource(256))

	var sampleTests = []struct {
		num             int
		length          int
		expectedSamples []int
	}{
		{2, 5, []int{0, 4}},
		{2, 2, []int{0, 1}},
		{12, 2, []int{0, 1}},
	}

	for _, i := range sampleTests {
		out := sampleUniform(random, i.num, i.length)
		if !reflect.DeepEqual(out, i.expectedSamples) {
			t.Errorf("Bad sample. Got %v, want %v", out, i.expectedSamples)
		}
	}
}

func TestPartition(t *testing.T) {
	var partitionTests = []struct {
		totalSize                            int
		numPartitions                        int
		numLong, numShort, lenLong, lenShort int
	}{
		{100, 10, 0, 10, 0, 10},
		{100, 9, 1, 8, 12, 11},
		{100, 11, 1, 10, 10, 9},
	}

	for _, i := range partitionTests {
		il, is, jl, js := partition(i.totalSize, i.numPartitions)
		if jl+js != i.numPartitions {
			t.Errorf("Total blocks = %d, must be %d", il+jl, i.numPartitions)
		}
		if il*jl+is*js != i.totalSize {
			t.Errorf("Total sampled size = %d, must be %d", il*jl+is*js, i.totalSize)
		}
		if jl != i.numLong {
			t.Errorf("Bad number of long blocks. got %d, want %d", jl, i.numLong)
		}
		if js != i.numShort {
			t.Errorf("Bad number of short blocks. got %d, want %d", js, i.numShort)
		}
		if il != i.lenLong {
			t.Errorf("Bad long block length. got %d, want %d", il, i.lenLong)
		}
		if is != i.lenShort {
			t.Errorf("Bad short block length. got %d, want %d", is, i.lenShort)
		}
	}
}

func TestFactorial(t *testing.T) {
	var factorialTests = []struct {
		x int
		f int
	}{
		{0, 1},
		{1, 1},
		{5, 120},
		{10, 3628800},
		{14, 87178291200},
	}

	for _, test := range factorialTests {
		if test.f != factorial(test.x) {
			t.Errorf("%d! = %d, should be %d", test.x, factorial(test.x), test.f)
		}
	}
}

func TestBinomial(t *testing.T) {
	var binomialTests = []struct {
		x int
		b int
	}{
		{2, 2},
		{6, 20},
		{7, 35},
		{11, 462},
		{12, 924},
	}

	for _, test := range binomialTests {
		if test.b != centerBinomial(test.x) {
			t.Errorf("(%d, %d/2) = %d, should be %d", test.x, test.x, centerBinomial(test.x), test.b)
		}
	}
}

func TestChoose(t *testing.T) {
	var chooseTests = []struct {
		n    int
		k    int
		comb int
	}{
		{0, 0, 1},
		{1, 0, 1},
		{2, 1, 2},
		{7, 2, factorial(7) / (factorial(5) * factorial(2))},
		{5, 3, factorial(5) / (factorial(2) * factorial(3))},
		{12, 7, factorial(12) / (factorial(5) * factorial(7))},
		{12, 1, 12},
		{12, 2, 66},
		{52, 5, 2598960},
		{52, 1, 52},
		{52, 52, 1},
		{52, 0, 1},
	}
	for _, test := range chooseTests {
		if choose(test.n, test.k) != test.comb {
			t.Errorf("choose(%d, %d) = %d, should be %d",
				test.n, test.k, choose(test.n, test.k), test.comb)
		}
	}
}

func TestBitSet(t *testing.T) {
	var bitTests = []struct {
		x     uint
		b     uint
		equal bool
	}{
		{0, 0, false},
		{0, 1, false},
		{1, 0, true},
		{7, 1, true},
		{16, 3, false},
		{16, 4, true},
		{16, 5, false},
		{0x1000, 12, true},
		{0x4000, 14, true},
		{0x4000, 15, false},
	}

	for _, test := range bitTests {
		if bitSet(test.x, test.b) != test.equal {
			t.Errorf("%d bit set in %d = %t, should be %t", test.b, test.x, bitSet(test.x, test.b), test.equal)
		}
	}
}

// simpleBitsSet returns how many bits in x are set.
func simpleBitsSet(x uint64) int {
	var count int
	var i uint64
	s := uint64(1)
	for i = 0; i < 64; i++ {
		if (x & s) != 0 {
			count++
		}
		s <<= 1
	}
	return count
}

func TestBitsSet(t *testing.T) {
	var bitsSetTests = []struct {
		x uint64
		b int
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{4, 1},
		{6, 2},
		{7, 3},
		{11, 3},
		{12, 2},
		{1<<63 | 1<<62, 2},
		{0x66666666, 16},
		{0x46464646, 4*1 + 4*2},
		{0xffdd4411, 2*4 + 2*3 + 2*1 + 2*1},
	}

	for _, test := range bitsSetTests {
		if test.b != bitsSet(test.x) {
			t.Errorf("bits set in %d = %d, should be %d", test.x, bitsSet(test.x), test.b)
		}
	}

	for i := 0; i < 1000; i++ {
		x := uint64(rand.Int63())
		if simpleBitsSet(x) != bitsSet(x) {
			t.Errorf("bits set in %d = %d, should be %d", x, bitsSet(x), simpleBitsSet(x))
		}
	}
}

func TestGrayCode(t *testing.T) {
	var grayTests = []struct {
		x uint64
		g uint64
	}{
		{0, 0},
		{1, 1},
		{2, 3},
		{5, 7},
		{6, 5},
		{9, 13},
	}

	for _, test := range grayTests {
		if test.g != grayCode(test.x) {
			t.Errorf("grayCode(%d) = %d, should be %d", test.x, grayCode(test.x), test.g)
		}
	}
}

func TestGraySequence(t *testing.T) {
	var grayTests = []struct {
		length int
		b      int
		seq    []int
	}{
		{4, 3, []int{7, 13, 14, 11}},
		{6, 2, []int{3, 6, 5, 12, 10, 9}},
	}

	for _, test := range grayTests {
		if !reflect.DeepEqual(buildGraySequence(test.length, test.b), test.seq) {
			t.Errorf("gray sequence for %d = %v, should be %v", test.b, buildGraySequence(test.length, test.b), test.seq)
		}
	}
}

func TestSmallestPrime(t *testing.T) {
	var primeTests = []struct {
		x int
		p int
	}{
		{0, 2},
		{1, 2},
		{2, 2},
		{20, 23},
		{1426, 1427},
		{1427, 1427},
		{1427, 1427},
		{1997, 1997},
		{1998, 1999},
		{1999, 1999},
		{3301, 3301},
		{8522, 8527},
	}

	for _, test := range primeTests {
		if test.p != smallestPrimeGreaterOrEqual(test.x) {
			t.Errorf("smallestPrimeGreaterOrEqual(%d) = %d, should be %d", test.x, smallestPrimeGreaterOrEqual(test.x), test.p)
		}
	}
}

func TestIsPrime(t *testing.T) {
	var primeTests = []struct {
		x     int
		prime bool
	}{
		{2000, false},
		{2099, true},
		{2607, false},
		{9007, true},
	}

	for _, test := range primeTests {
		if test.prime != isPrime(test.x) {
			t.Errorf("isPrime(%d) = %t, should be %t", test.x, isPrime(test.x), test.prime)
		}
	}
}
