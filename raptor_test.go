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

func TestRaptorRand(t *testing.T) {
	var randTests = []struct {
		x uint32
		i uint32
		m uint32
		r uint32
	}{
		{1, 4, 150, 50},
		{20005, 19, 25, 6},
		{2180, 11, 1383483, 1166141},
	}

	for _, test := range randTests {
		if test.r != raptorRand(test.x, test.i, test.m) {
			t.Errorf("raptorRand(%d, %d, %d) = %d, should be %d",
				test.x, test.i, test.m, raptorRand(test.x, test.i, test.m), test.r)
		}
	}
}

func TestDeg(t *testing.T) {
	var degreeTests = []struct {
		x uint32
		d int
	}{
		{0, 1},
		{10000, 1},
		{10240, 1},
		{10241, 2},
		{10242, 2},
		{715000, 4},
		{1000000, 11},
		{1034300, 40},
		{1048575, 40},
		{1048576, 40},
	}

	for _, test := range degreeTests {
		if test.d != deg(test.x) {
			t.Errorf("deg(%d) = %d, should be %d", test.x, deg(test.x), test.d)
		}
	}
}

func TestIntermediateSymbols(t *testing.T) {
	var intermediateTests = []struct {
		k int
		l int
		s int
		h int
	}{
		{0, 4, 2, 2},
		{1, 8, 3, 4},
		{10, 23, 7, 6}, // from a Luby, Shokrollahi paper
		{13, 26, 7, 6},
		{14, 28, 7, 7},
		{500, 553, 41, 12},
		{5000, 5166, 151, 15},
	}

	for _, test := range intermediateTests {
		l, s, h := intermediateSymbols(test.k)
		if l != test.l || s != test.s || h != test.h {
			t.Errorf("intermediateSymbols(%d) = (%d, %d, %d), should be %d, %d, %d",
				test.k, l, s, h, test.l, test.s, test.h)
		}
	}
}

func TestTripleGenerator(t *testing.T) {
	var tripleTests = []struct {
		k int
		x uint16
		d int
		a uint32
		b uint32
	}{
		{0, 3, 2, 4, 3},
		{1, 4, 4, 2, 5},
		{4, 0, 10, 13, 1},
		{4, 4, 4, 6, 2},
		{500, 514, 2, 107, 279},
		{1000, 52918, 3, 1070, 121},
	}

	for _, test := range tripleTests {
		d, a, b := tripleGenerator(test.k, test.x)
		if d != test.d || a != test.a || b != test.b {
			t.Errorf("tripleGenerator(%d, %d) = (%d, %d, %d), should be %d, %d, %d",
				test.k, test.x, d, a, b, test.d, test.a, test.b)
		}
	}
}

func TestSystematicIndices(t *testing.T) {
	if systematicIndextable[4] != 18 {
		t.Errorf("Systematic index for 4 was %d, must be 18", systematicIndextable[4])
	}
	if systematicIndextable[21] != 2 {
		t.Errorf("Systematic index for 4 was %d, must be 2", systematicIndextable[4])
	}
	if systematicIndextable[8192] != 2665 {
		t.Errorf("Systematic index for 4 was %d, must be 2665", systematicIndextable[4])
	}
}

func TestLTIndices(t *testing.T) {
	var ltIndexTests = []struct {
		k       int
		x       uint16
		indices []int
	}{
		{4, 0, []int{1, 2, 3, 4, 6, 7, 8, 10, 11, 12}},
		{4, 4, []int{2, 3, 8, 9}},
		{100, 1, []int{51, 104}},
		{1000, 727, []int{306, 687, 1040}},
		{10, 57279, []int{19, 20, 21, 22}},
	}

	for _, test := range ltIndexTests {
		indices := findLTIndices(test.k, test.x)
		if !reflect.DeepEqual(indices, test.indices) {
			t.Errorf("findLTIndices(%d, %d) = %v, should be %v",
				test.k, test.x, indices, test.indices)
		}
	}
}

func TestRaptorDecoderConstruction(t *testing.T) {
	decoder := newRaptorDecoder(&raptorCodec{SymbolAlignmentSize: 1,
		NumSourceSymbols: 10}, 1)
	printMatrix(decoder.matrix, t)
	// From the first row of the constraint matrix. Test vectors from a paper by
	// Luby and Shokrollahi.
	if !reflect.DeepEqual(decoder.matrix.coeff[0], []int{0, 5, 6, 7, 10}) {
		t.Errorf("First matrix equation was %v, should be {0, 5, 6, 7, 10}",
			decoder.matrix.coeff[0])
	}
	// Fourth row
	if !reflect.DeepEqual(decoder.matrix.coeff[1], []int{1, 2, 3, 8, 13}) {
		t.Errorf("Second matrix equation was %v, should be {1, 2, 3, 8, 13}",
			decoder.matrix.coeff[0])
	}
	// Fifth row
	if !reflect.DeepEqual(decoder.matrix.coeff[2], []int{2, 3, 4, 7, 9, 14}) {
		t.Errorf("Third matrix equation was %v, should be {2, 3, 4, 7, 9, 14}",
			decoder.matrix.coeff[0])
	}
}

func printIntermediateEncoding(intermediate []block, t *testing.T) {
	t.Log("Intermediate Encoding Blocks")
	t.Log("----------------------------")
	kb := 0
	for s := range intermediate {
		t.Log("intermediate", kb, intermediate[s].data)
		kb++
	}
}

func TestIntermediateBlocks(t *testing.T) {
	blocks := [4]block{
		{data: []byte{0, 0, 0, 1}},
		{data: []byte{0, 0, 1, 0}},
		{data: []byte{0, 1, 0, 0}},
		{data: []byte{1, 0, 0, 0}},
	}

	srcBlocks := make([]block, len(blocks))
	for i := range blocks {
		srcBlocks[i].xor(blocks[i])
	}
	intermediate := raptorIntermediateBlocks(srcBlocks) // destructive to srcBlocks
	if len(intermediate) != 14 {
		t.Errorf("Length of intermediate blocks is %d, should be 14", len(intermediate))
	}
	printIntermediateEncoding(intermediate, t)

	t.Log("Finding intermediate equations")
	// test that intermediate ltEnc equations hold
	for i := 0; i < 4; i++ {
		block := ltEncode(4, uint16(i), intermediate)
		if !reflect.DeepEqual(block, blocks[i]) {
			t.Errorf("The result of LT encoding on the intermediate blocks for "+
				"block %d is %v, should be the source blocks %v", i,
				block.data, blocks[i].data)
		}
	}
}

func TestSystematicRaptorCode(t *testing.T) {
	c := NewRaptorCodec(13, 2)
	message := []byte("abcdefghijklmnopqrstuvwxyz")
	blocks := c.GenerateIntermediateBlocks(message, c.SourceBlocks())

	messageCopy := []byte("abcdefghijklmnopqrstuvwxyz")
	sourceLong, sourceShort := partitionBytes(messageCopy, c.SourceBlocks())
	sourceCopy := equalizeBlockLengths(sourceLong, sourceShort)

	for _, testIndex := range []int64{0, 1, 2, 3, 4, 5} {
		t.Logf("Testing %d", testIndex)
		b := ltEncode(13, uint16(testIndex), blocks)
		if !reflect.DeepEqual(b.data, sourceCopy[testIndex].data) {
			t.Errorf("LT encoding of CodeBlock=%d was (%v), should be the %d'th source block (%v)",
				testIndex, b.data, testIndex, sourceCopy[testIndex].data)
		}
	}
}

func TestIntermediateBlocks13(t *testing.T) {
	blocks := make([]block, 13)
	for i := range blocks {
		blocks[i].data = make([]byte, 13)
		blocks[i].data[i] = 1
	}

	srcBlocks := make([]block, 13)
	for i := range blocks {
		srcBlocks[i].xor(blocks[i])
	}
	intermediate := raptorIntermediateBlocks(srcBlocks) // destructive to srcBlocks
	if len(intermediate) != 26 {
		t.Errorf("Length of intermediate blocks is %d, should be 26", len(intermediate))
	}
	printIntermediateEncoding(intermediate, t)

	t.Log("Finding intermediate equations")
	// test that intermediate ltEnc equations hold
	for i := 0; i < 13; i++ {
		block := ltEncode(13, uint16(i), intermediate)
		if !reflect.DeepEqual(block, blocks[i]) {
			t.Errorf("The result of LT encoding on the intermediate blocks for "+
				"block %d is %v, should be the source blocks %v", i,
				block.data, blocks[i].data)
		}
	}

	ids := make([]int64, 45)
	random := rand.New(rand.NewSource(8923489))
	for i := range ids {
		ids[i] = int64(random.Intn(60000))
	}
	c := raptorCodec{NumSourceSymbols: 13, SymbolAlignmentSize: 13}
	srcBlocks = make([]block, 13)
	for i := range blocks {
		srcBlocks[i].xor(blocks[i])
	}

	message := make([]byte, 0)
	for i := range blocks {
		message = append(message, blocks[i].data...)
	}
	codeBlocks := EncodeLTBlocks(message, ids, &c) // destructive to srcBlocks

	t.Log("DECODE--------")
	decoder := newRaptorDecoder(&c, len(message))
	for i := 0; i < 17; i++ {
		decoder.AddBlocks([]LTBlock{codeBlocks[i]})
	}

	message = make([]byte, 0)
	for i := range blocks {
		message = append(message, blocks[i].data...)
	}

	if decoder.matrix.determined() {
		t.Log("Recovered:\n", decoder.matrix.v)
		out := decoder.Decode()
		if !reflect.DeepEqual(message, out) {
			t.Errorf("Decoding result must equal %v, got %v", message, out)
		}
	}
}

func TestRaptorCodec(t *testing.T) {
	c := NewRaptorCodec(13, 2)
	message := []byte("abcdefghijklmnopqrstuvwxyz")
	ids := make([]int64, 45)
	random := rand.New(rand.NewSource(8923489))
	for i := range ids {
		ids[i] = int64(random.Intn(60000))
	}

	messageCopy := make([]byte, len(message))
	copy(messageCopy, message)

	codeBlocks := EncodeLTBlocks(messageCopy, ids, c)

	t.Log("DECODE--------")
	decoder := newRaptorDecoder(c.(*raptorCodec), len(message))
	for i := 0; i < 17; i++ {
		decoder.AddBlocks([]LTBlock{codeBlocks[i]})
	}
	if decoder.matrix.determined() {
		t.Log("Recovered:\n", decoder.matrix.v)
		out := decoder.Decode()
		if !reflect.DeepEqual(message, out) {
			t.Errorf("Decoding result must equal %s, got %s", string(message), string(out))
		}
	}
}
