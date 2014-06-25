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
)

// MersenneTwister is an implementation of the MT19937 PRNG of Matsumoto and Nishimura.
// Following http://www.math.sci.hiroshima-u.ac.jp/~m-mat/MT/ARTICLES/mt.pdf
// Uses the 32-bit version of the algorithm.
// Satisfies math/rand.Source
type MersenneTwister struct {
	mt          [624]uint32
	index       int
	initialized bool
}

// NewMersenneTwister creates a new MT19937 PRNG with the given seed. The seed
// is converted to a 32-bit seed by XORing the high and low halves.
func NewMersenneTwister(seed int64) rand.Source {
	t := &MersenneTwister{}
	t.Seed(seed)

	return t
}

func (t *MersenneTwister) Seed(seed int64) {
	t.initialize(uint32(((seed >> 32) ^ seed) & math.MaxUint32))
}

// Int63 produces a new int64 value between 0 and 2^63-1 by combining the bits
// of two Uint32 values.
func (t *MersenneTwister) Int63() int64 {
	a := t.Uint32()
	b := t.Uint32()
	return (int64(a) << 31) ^ int64(b)
}

func (t *MersenneTwister) Uint32() uint32 {
	if !t.initialized {
		t.initialize(4357) // value from original paper; lets the twister do something reasonable when empty
	}

	// Every 624 calls, revolve the untempered seed matrix
	if t.index == 0 {
		t.generateUntempered()
	}

	y := t.mt[t.index]
	t.index++
	if t.index >= len(t.mt) {
		t.index = 0
	}
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18

	return y
}

func (t *MersenneTwister) initialize(seed uint32) {
	t.index = 0
	t.mt[0] = seed

	for i := 1; i < len(t.mt); i++ {
		// Improved initialization: not subject to the same runs of correlated numbers.
		t.mt[i] = (1812433253*(t.mt[i-1]^(t.mt[i-1]>>30)) + uint32(i)) & math.MaxUint32
		// Original paper did this
		// t.mt[i] = uint32(69069 * int64(t.mt[i-1]) & math.MaxUint32)
	}
	t.initialized = true
}

func (t *MersenneTwister) generateUntempered() {
	mag01 := [2]uint32{0x0, 0x9908b0df}
	for i := 0; i < len(t.mt); i++ {
		y := (t.mt[i] & 0x80000000) | (t.mt[(i+1)%len(t.mt)] & 0x7fffffff)
		t.mt[i] = (t.mt[(i+397)%len(t.mt)] ^ (y >> 1)) ^ mag01[y&0x01]
	}
}

// MersenneTwister64 is a 64-bit MT19937 PRNG after Nishimura.
// See http://dl.acm.org/citation.cfm?id=369540&dl=ACM&coll=DL&CFID=261426526&CFTOKEN=25107569
// Satisfies math/rand.Source
type MersenneTwister64 struct {
	mt          [312]uint64
	index       int
	initialized bool
}

// NewMersenneTwister64 creates a new 64-bit version of the MT19937 PRNG.
func NewMersenneTwister64(seed int64) rand.Source {
	t := &MersenneTwister64{}
	t.Seed(seed)

	return t
}

// Seed initializes the state of the PRNG with the given seed value.
func (t *MersenneTwister64) Seed(seed int64) {
	t.initialize(uint64(seed))
}

// Int63 returns the next value from the PRNG. This value is the low 63 bits
// of the Uint64 value.
func (t *MersenneTwister64) Int63() int64 {
	return int64(t.Uint64() & math.MaxInt64)
}

func (t *MersenneTwister64) initialize(seed uint64) {
	t.index = 0
	t.mt[0] = seed

	for i := 1; i < len(t.mt); i++ {
		t.mt[i] = 6364136223846793005*(t.mt[i-1]^(t.mt[i-1]>>62)) + uint64(i)
	}
	t.initialized = true
}

// SeedSlice allows the twister to be initialized with a slice of seed values.
func (t *MersenneTwister64) SeedSlice(seed []uint64) {
	t.initialize(19650218)

	length := len(seed)
	if len(t.mt) > length {
		length = len(t.mt)
	}

	i, j := 1, 0
	for k := 0; k < length; k++ {
		t.mt[i] = (t.mt[i] ^ ((t.mt[i-1] ^ (t.mt[i-1] >> 62)) * 3935559000370003845)) + seed[j] + uint64(j)
		i++
		j++
		if i >= len(t.mt) {
			t.mt[0] = t.mt[len(t.mt)-1]
			i = 1
		}
		if j >= len(seed) {
			j = 0
		}
	}

	for k := 0; k < len(t.mt)-1; k++ {
		t.mt[i] = (t.mt[i] ^ ((t.mt[i-1] ^ (t.mt[i-1] >> 62)) * 2862933555777941757)) - uint64(i)
		i++
		if i >= len(t.mt) {
			t.mt[0] = t.mt[len(t.mt)-1]
			i = 1
		}
	}

	t.mt[0] = 1 << 63
}

// Uint64 returns the next pseudo-random value from the twister.
func (t *MersenneTwister64) Uint64() uint64 {
	if !t.initialized {
		t.initialize(5489)
	}

	// Every 312 calls, revolve the untempered seed matrix
	if t.index == 0 {
		t.generateUntempered()
	}

	y := t.mt[t.index]
	t.index++
	if t.index >= len(t.mt) {
		t.index = 0
	}
	y ^= (y >> 29) & 0x5555555555555555
	y ^= (y << 17) & 0x71d67fffeda60000
	y ^= (y << 37) & 0xfff7eee000000000
	y ^= y >> 43

	return y
}

func (t *MersenneTwister64) generateUntempered() {
	mag01 := [2]uint64{0x0, 0xb5026f5aa96619e9}
	for i := 0; i < len(t.mt); i++ {
		y := (t.mt[i] & 0xffffffff80000000) | (t.mt[(i+1)%len(t.mt)] & 0x7fffffff)
		t.mt[i] = (t.mt[(i+156)%len(t.mt)] ^ (y >> 1)) ^ mag01[y&0x01]
	}
}
