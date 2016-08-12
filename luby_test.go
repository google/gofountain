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
	"math/rand"
	"reflect"
	"testing"
)

func TestLubyTransformBlockGenerator(t *testing.T) {
	message := []byte("abcdefghijklmnopqrstuvwxyz")
	codec := NewLubyCodec(4, rand.New(NewMersenneTwister(200)), SolitonDistribution(4))

	wantIndices := [][]int{
		{0},
		{1},
		{3},
		{0, 1},
		{1, 2, 3},
	}

	// These magic seeds are chosen such that they will generate the block compositions
	// in wantIndices given the PRNG with which we initialized the codec above (the
	// Mersenne Twister with seed=200).
	encodeBlocks := []int64{7, 34, 5, 31, 25}
	for i := range wantIndices {
		indices := codec.PickIndices(encodeBlocks[i])
		if !reflect.DeepEqual(indices, wantIndices[i]) {
			t.Logf("Got %v indices for %d, want %v", indices, encodeBlocks[i], wantIndices[i])
		}
	}

	source := codec.GenerateIntermediateBlocks(message, codec.SourceBlocks())
	lubyBlocks := make([]LTBlock, len(encodeBlocks))
	for i := range encodeBlocks {
		b := generateLubyTransformBlock(source, wantIndices[i])
		lubyBlocks[i].BlockCode = encodeBlocks[i]
		lubyBlocks[i].Data = make([]byte, b.length())
		copy(lubyBlocks[i].Data, b.data)
	}

	if len(source) != codec.SourceBlocks() {
		t.Logf("Got %d encoded blocks, want %d", len(source), codec.SourceBlocks())
	}

	if string(lubyBlocks[0].Data) != "abcdefg" {
		t.Errorf("Data for {0} block is %v, should be 'abcdefg'", string(lubyBlocks[0].Data))
	}
	if string(lubyBlocks[1].Data) != "hijklmn" {
		t.Errorf("Data for {1} block is %v, should be 'hijklmn'", string(lubyBlocks[1].Data))
	}
	if string(lubyBlocks[2].Data) != "uvwxyz" {
		t.Errorf("Data for {1} block is %v, should be 'uvwxyz'", string(lubyBlocks[2].Data))
	}
	if lubyBlocks[3].Data[0] != 'a'^'h' {
		t.Errorf("Data[0] for {0, 1} block is %d, should be 'a'^'h' (%d)",
			lubyBlocks[3].Data[0], 'a'^'h')
	}
	if lubyBlocks[4].Data[0] != 'h'^'o'^'u' {
		t.Errorf("Data[0] for {1,2,3} block is %d, should be 'h'^'o'^'u' (%d)",
			lubyBlocks[3].Data[0], 'h'^'o'^'u')
	}
}

func TestLubyDecoder(t *testing.T) {
	message := []byte("abcdefghijklmnopqrstuvwxyz")
	codec := NewLubyCodec(4, rand.New(NewMersenneTwister(200)), SolitonDistribution(4))

	encodeBlocks := []int64{7, 34, 5, 31, 25}
	lubyBlocks := EncodeLTBlocks(message, encodeBlocks, codec)

	decoder := codec.NewDecoder(len(message))
	determined := decoder.AddBlocks(lubyBlocks)
	if !determined {
		t.Errorf("After adding code blocks, decoder is still undetermined.")
	}
	decoded := decoder.Decode()
	if !reflect.DeepEqual(decoded, message) {
		t.Errorf("Decoded luby transform message is %v, expected %v", decoded, message)
		t.Logf("String value = %v", string(decoded))
	}
}
