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
	"sort"
)

// Note that these CDFs (cumulative distribution function) will be used for
// selecting source blocks for code block generation. A typical algorithm
// is to choose a number from a distribution, then pick uniformly that many
// source blocks to XOR into a code block. To use CDF mapping values, pick a
// random number r (0 <= r < 1) and then find the smallest i such that
// CDF[i] >= r.

// solitonDistribution returns a CDF mapping for the soliton distribution.
// N (the number of elements in the CDF) cannot be less than 1
// The CDF is one-based: the probability of picking 1 from the distribution
// is CDF[1].
func solitonDistribution(n int) []float64 {
	cdf := make([]float64, n+1)
	cdf[1] = 1 / float64(n)
	for i := 2; i < len(cdf); i++ {
		cdf[i] = cdf[i-1] + (1 / (float64(i) * float64(i-1)))
	}
	return cdf
}

// robustSolitonDistribution returns a CDF mapping for the robust solition
// distribution.
// This is an addition to the soliton distribution with three parameters,
// N, M, and delta.
// Before normalization, the correction pdf(i) = 1/i*M, for i=1..M-1,
// pdf(M) = ln(N/(M*delta))/M
// pdf(i) = 0 for i = M+1..N
// These values are added to the ideal soliton distribution, and then the
// result normalized.
// The CDF is one-based: the probability of picking 1 from the distribution
// is CDF[1].
func robustSolitonDistribution(n int, m int, delta float64) []float64 {
	pdf := make([]float64, n+1)

	pdf[1] = 1/float64(n) + 1/float64(m)
	total := pdf[1]
	for i := 2; i < len(pdf); i++ {
		pdf[i] = (1 / (float64(i) * float64(i-1)))
		if i < m {
			pdf[i] += 1 / (float64(i) * float64(m))
		}
		if i == m {
			pdf[i] += math.Log(float64(n)/(float64(m)*delta)) / float64(m)
		}
		total += pdf[i]
	}

	cdf := make([]float64, n+1)
	for i := 1; i < len(pdf); i++ {
		pdf[i] /= total
		cdf[i] = cdf[i-1] + pdf[i]
	}
	return cdf
}

// onlineSolitionDistribution returns a soliton-like distribution for
// Online Codes
// See http://pdos.csail.mit.edu/~petar/papers/maymounkov-bigdown-lncs.ps
// 'Rateless Codes and Big Downloads' by Maymounkov and Mazieres
// The distribution is described by a parameter epsilon, which is the
// the "overage factor" required to reconstruct the source message in an
// Online Code.
// F = ciel(ln(eps^2/4 / ln(1 - eps/2))
// and the pdf is pdf[1] = 1 - (1 + 1/F)/(1 + eps)
// pdf[i] = ((1 - pdf[1])F) / ((F-1)i(i-1)) for 2 <= i <= F
func onlineSolitonDistribution(eps float64) []float64 {
	f := math.Ceil(math.Log(eps*eps/4) / math.Log(1-(eps/2)))

	cdf := make([]float64, int(f+1))

	// First coefficient is 1 - ( (1 + 1/f) / (1+e) )
	rho := 1 - ((1 + (1 / f)) / (1 + eps))
	cdf[1] = rho

	// Subsequent i'th coefficient is (1-rho)*F / (F-1)i*(i-1)
	for i := 2; i <= int(f); i++ {
		rhoI := ((1 - rho) * f) / ((f - 1) * float64(i-1) * float64(i))
		cdf[i] = cdf[i-1] + rhoI
	}

	return cdf
}

// pickDegree returns the smallest index i such that cdf[i] > r
// (r a random number from the random generator)
// cdf must be sorted in ascending order.
func pickDegree(random *rand.Rand, cdf []float64) int {
	r := random.Float64()
	d := sort.SearchFloat64s(cdf, r)
	if cdf[d] > r {
		return d
	}

	if d < len(cdf)-1 {
		return d + 1
	} else {
		return len(cdf) - 1
	}
}

// sampleUniform picks num numbers from [0,max) uniformly.
// There will be no duplicates.
// If num >= max, simply returns a slice with all indices from 0 to max-1
// without touching the random number generator.
// The returned slice is sorted.
func sampleUniform(random *rand.Rand, num, max int) []int {
	if num >= max {
		picks := make([]int, max)
		for i := 0; i < max; i++ {
			picks[i] = i
		}
		return picks
	}

	picks := make([]int, num)
	seen := make(map[int]bool)
	for i := 0; i < num; i++ {
		p := random.Intn(max)
		for seen[p] {
			p = random.Intn(max)
		}
		picks[i] = p
		seen[p] = true
	}
	sort.Ints(picks)
	return picks
}

// partition is the block partitioning function from RFC 5053 S.5.3.1.2
// See http://tools.ietf.org/html/rfc5053
// Partitions a number i (a size) into j semi-equal pieces. The details are
// in the return values: there are jl longer pieces of size il, and js shorter
// pieces of size is.
func partition(i, j int) (il int, is int, jl int, js int) {
	il = int(math.Ceil(float64(i) / float64(j)))
	is = int(math.Floor(float64(i) / float64(j)))
	jl = i - (is * j)
	js = j - jl

	if jl == 0 {
		il = 0
	}
	if js == 0 {
		is = 0
	}

	return
}

// factorial calculates the factorial (x!) of the input argument.
func factorial(x int) int {
	f := 1
	for i := 1; i <= x; i++ {
		f *= i
	}
	return f
}

// centerBinomial calculates choose(x, ceil(x/2)) = x!/(x/2)!(x-(x/2)m!)
func centerBinomial(x int) int {
	return choose(x, x/2)
}

// choose calculates (n k) or n choose k. Tolerant of quite large n/k.
func choose(n int, k int) int {
	if k > n/2 {
		k = n - k
	}
	numerator := make([]int, n-k)
	denominator := make([]int, n-k)
	for i, j := k+1, 1; i <= n; i, j = i+1, j+1 {
		numerator[j-1] = int(i)
		denominator[j-1] = j
	}

	if len(denominator) > 0 {
		// find the first member of numerator not in denominator
		z := sort.SearchInts(numerator, denominator[len(denominator)-1])
		if z > 0 {
			numerator = numerator[z+1:]
			denominator = denominator[0 : len(denominator)-z-1]
		}
	}

	for j := len(denominator) - 1; j > 0; j-- {
		for i := len(numerator) - 1; i >= 0; i-- {
			if numerator[i]%denominator[j] == 0 {
				numerator[i] /= denominator[j]
				denominator[j] = 1
				break
			}
		}
	}
	f := 1
	for _, i := range numerator {
		f *= i
	}
	return f
}

// bitSet returns true if x has the b'th bit set
func bitSet(x uint, b uint) bool {
	return (x>>b)&1 == 1
}

// bitsSet returns how many bits in x are set.
// This algorithm basically uses shifts and ANDs to sum up the bits in
// a tree fashion.
func bitsSet(x uint64) int {
	x -= (x >> 1) & 0x5555555555555555
	x = (x & 0x3333333333333333) + ((x >> 2) & 0x3333333333333333)
	x = (x + (x >> 4)) & 0x0f0f0f0f0f0f0f0f
	return int((x * 0x0101010101010101) >> 56)
}

// grayCode calculates the gray code representation of the input argument
// The Gray code is a binary representation in which successive values differ
// by exactly one bit. See http://en.wikipedia.org/wiki/Gray_code
func grayCode(x uint64) uint64 {
	return (x >> 1) ^ x
}

// buildGraySequence returns a sequence (in ascending order) of "length" Gray numbers,
// all of which have exactly "b" bits set.
func buildGraySequence(length int, b int) []int {
	s := make([]int, length)
	i := 0
	for x := uint64(0); ; x++ {
		g := grayCode(x)
		if bitsSet(g) == b {
			s[i] = int(g)
			i++
			if i >= length {
				break
			}
		}
	}
	return s
}

// isPrime tests x for primality. Works on numbers less than the square of
// the largest smallPrimes array.
func isPrime(x int) bool {
	for _, p := range smallPrimes {
		if p*p > x {
			return true
		}
		if x%p == 0 {
			return false
		}
	}
	// Well, not really, but we don't know any higher primes for sure.
	return true
}

// smallestPrimeGreaterOrEqual returns the smallest prime greater than or equal to x
// TODO(gbillock): should handle up to 70000 or so
func smallestPrimeGreaterOrEqual(x int) int {
	if x <= smallPrimes[len(smallPrimes)-1] {
		p := sort.Search(len(smallPrimes), func(i int) bool {
			return smallPrimes[i] >= x
		})
		return smallPrimes[p]
	}
	for !isPrime(x) {
		x++
	}
	return x
}
