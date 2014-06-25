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

// The RU10 fountain is an unsystematic(*) fountain code which uses a degree
// distribution and intermediate block generation scheme similar to the
// R10 (Raptor) code. The unsystematic code has much lower independent block
// generation setup cost -- it requires only generating the auxiliary blocks,
// and does not require the full decode run of the algorithm to generate the
// intermediate encoding. This makes it cheaper in two ways for parallel
// execution. If all receivers can be expected to have the decoder in place,
// there is no decoding advantage to requiring the source blocks to appear in the
// code block set. But more to the point, if multiple encoders are working from
// the same source blocks, they can generate code blocks independently without
// having to worry if they are re-transmitting source blocks. Since the encoder
// state is cheaper and code blocks interchangeable, we sacrifice a little
// performance for that parallelizability and statelessness. We also gain a little
// in that this encoder variant needn't rely on the systematic number table
// of the R10 codec, and without that constraint, the number of possible code block
// ESI IDs is basically unlimited.
//
// (*) Well, not by design at least.

// This triple generator uses the Mersenne Twister to generate random seeds.
// k is the number of source symbols.
// x is the (random) code symbol ID.
// The generator creates values (d, a, b) to be used in constructing intermediate blocks.
func ru10TripleGenerator(k int, x int64) (int, uint32, uint32) {
	l, _, _ := intermediateSymbols(k)
	lprime := smallestPrimeGreaterOrEqual(l)

	// TODO(gbillock): nudge x as a function of k to get better overhead-failure curve?
	rand := rand.New(NewMersenneTwister64(x))

	v := uint32(rand.Int63() % 1048576)
	a := uint32(1 + (rand.Int63() % int64(lprime - 1)))
	b := uint32(rand.Int63() % int64(lprime))
	d := deg(v)

	return d, a, b
}

// ru10Codec implements the Raptor-alike fountain code.
// Implements fountain.Codec.
type ru10Codec struct {
	numSourceSymbols int

  symbolAlignmentSize int
}

// NewRU10Codec creates an unsystematic raptor-like fountain codec which uses an
// intermediate block generation algorithm similar to the Raptor R10 codec.
func NewRU10Codec(numSourceSymbols int, symbolAlignmentSize int) Codec {
  return &ru10Codec{
    numSourceSymbols: numSourceSymbols,
    symbolAlignmentSize: symbolAlignmentSize}
}

// SourceBlocks returns the number of source blocks the codec uses in the
// source message plus intermediate blocks added.
func (c *ru10Codec) SourceBlocks() int {
	return c.numSourceSymbols
}

// PickIndices uses the R10 distribution function to pick indices. It gets
// numbers from the triple generator.
func (c *ru10Codec) PickIndices(codeBlockIndex int64) []int {
	d, a, b := ru10TripleGenerator(c.numSourceSymbols, codeBlockIndex)
	l, _, _ := intermediateSymbols(c.numSourceSymbols)
	lprime := uint32(smallestPrimeGreaterOrEqual(l))

	if d > l {
		d = l
	}

	indices := make([]int, 0)
	for b >= uint32(l) {
		b = (b + a) % lprime
	}
	indices = append(indices, int(b))

	for j := 1; j < d; j++ {
		b = (b + a) % lprime
		for b >= uint32(l) {
			b = (b + a) % lprime
		}
		indices = append(indices, int(b))
	}

	sort.Ints(indices)
	return indices
}

// RU10 intermediate encoding consists of the source symbols plus additional
// intermediate symbols consisting of exactly the S and H blocks the R10 code
// uses. The difference is that the code is unsystematic -- the source blocks
// aren't necessarily going to be represented at the output -- so when we do
// the decode we don't need to translate from the intermediate symbols back to
// the source symbols: the source symbols are just the first K intermediate symbols.
func (c *ru10Codec) GenerateIntermediateBlocks(message []byte, numBlocks int) []block {
	sourceLong, sourceShort := partitionBytes(message, c.numSourceSymbols)
	source := equalizeBlockLengths(sourceLong, sourceShort)

	_, s, h := intermediateSymbols(c.numSourceSymbols)

	k := c.numSourceSymbols
	compositions := make([][]int, s)

	for i := 0; i < k; i++ {
		a := 1 + (int(math.Floor(float64(i)/float64(s))) % (s - 1))
		b := i % s
		compositions[b] = append(compositions[b], i)
		b = (b + a) % s
		compositions[b] = append(compositions[b], i)
		b = (b + a) % s
		compositions[b] = append(compositions[b], i)
	}
	for i := 0; i < s; i++ {
		b := generateLubyTransformBlock(source, compositions[i])
		source = append(source, b)
	}

	hprime := int(math.Ceil(float64(h) / 2))
	m := buildGraySequence(k+s, hprime)
	for i := 0; i < h; i++ {
		hcomposition := make([]int, 0)
		for j := 0; j < k+s; j++ {
			if bitSet(uint(m[j]), uint(i)) {
				hcomposition = append(hcomposition, j)
			}
		}
		b := generateLubyTransformBlock(source, hcomposition)
		source = append(source, b)
	}
	return source
}

// NewDecoder creates a new RU10 decoder
func (c *ru10Codec) NewDecoder(messageLength int) Decoder {
  return newRU10Decoder(c, messageLength)
}

// ru10Decoder is the corresponding decoder for fountain codes using the RU10 encoder.
type ru10Decoder struct {
	decoder *raptorDecoder
}

// newRU10Decoder creates a new raptor decoder for a given message. The
// codec supplied must be the same one as the message was encoded with.
func newRU10Decoder(c *ru10Codec, length int) *ru10Decoder {
	return &ru10Decoder{
		decoder: newRaptorDecoder(&raptorCodec{
      SymbolAlignmentSize: c.symbolAlignmentSize,
			NumSourceSymbols: c.numSourceSymbols},
			length),
	}
}

func (d *ru10Decoder) AddBlocks(blocks []LTBlock) bool {
	c := ru10Codec{
    symbolAlignmentSize: d.decoder.codec.SymbolAlignmentSize,
		numSourceSymbols: d.decoder.codec.NumSourceSymbols}
	for i := range blocks {
		indices := c.PickIndices(blocks[i].BlockCode)
		d.decoder.matrix.addEquation(indices, block{data: blocks[i].Data})
	}
	return d.decoder.matrix.determined()
}

func (d *ru10Decoder) Decode() []byte {
	if !d.decoder.matrix.determined() {
		return nil
	}

	d.decoder.matrix.reduce()

	// Now the intermediate blocks are held in d.decoder.matrix.v. The source
	// blocks are the first K intermediate blocks.
	intermediate := d.decoder.matrix.v

	lenLong, lenShort, numLong, numShort :=
		partition(d.decoder.messageLength, d.decoder.codec.NumSourceSymbols)
	out := make([]byte, d.decoder.messageLength)
	out = out[0:0]
	for i := 0; i < numLong; i++ {
		out = append(out, intermediate[i].data[0:lenLong]...)
	}
	for i := numLong; i < numLong+numShort; i++ {
		out = append(out, intermediate[i].data[0:lenShort]...)
	}
	return out
}
