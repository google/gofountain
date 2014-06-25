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
)

// Random binary fountain code. In this code, the constituent source blocks in
// a code block are selected randomly and independently.

// BinaryCodec contains the codec information for the random binary fountain
// encoder and decoder.
// Implements fountain.Codec
type binaryCodec struct {
	// numSourceBlocks is the number of source blocks (N) the source message is split into.
	numSourceBlocks int
}

// NewBinaryCodec returns a codec implementing the binary fountain code,
// where source blocks composing each LT block are chosen randomly and independently.
func NewBinaryCodec(numSourceBlocks int) Codec {
	return &binaryCodec{numSourceBlocks: numSourceBlocks}
}

// SourceBlocks returns the number of source blocks used in the codec.
func (c *binaryCodec) SourceBlocks() int {
	return c.numSourceBlocks
}

// PickIndices finds the source indices for a code block given an ID and
// a random seed. Uses the Mersenne Twister internally.
func (c *binaryCodec) PickIndices(codeBlockIndex int64) []int {
	random := rand.New(NewMersenneTwister(codeBlockIndex))

	var indices []int
	for b := 0; b < c.SourceBlocks(); b++ {
		if random.Intn(2) == 1 {
			indices = append(indices, b)
		}
	}

	return indices
}

// GenerateIntermediateBlocks simply returns the partition of the input message
// into source blocks. It does not perform any additional precoding.
func (c *binaryCodec) GenerateIntermediateBlocks(message []byte, numBlocks int) []block {
	long, short := partitionBytes(message, c.numSourceBlocks)
	source := equalizeBlockLengths(long, short)

	return source
}

// NewDecoder creates a new binary fountain code decoder
func (c *binaryCodec) NewDecoder(messageLength int) Decoder {
	return newBinaryDecoder(c, messageLength)
}

// binaryDecoder is the state required to decode a combinatoric fountain
// code message.
type binaryDecoder struct {
	codec         binaryCodec
	messageLength int

	// The sparse equation matrix used for decoding.
	matrix sparseMatrix
}

// newBinaryDecoder creates a new decoder for a particular message.
// The codec parameters used to create the original encoding blocks must be provided.
// The decoder is only valid for decoding code blocks for a particular message.
func newBinaryDecoder(c *binaryCodec, length int) *binaryDecoder {
	return &binaryDecoder{
		codec:         *c,
		messageLength: length,
		matrix: sparseMatrix{
			coeff: make([][]int, c.numSourceBlocks),
			v:     make([]block, c.numSourceBlocks),
		}}
}

// AddBlocks adds a set of encoded blocks to the decoder. Returns true if the
// message can be fully decoded. False if there is insufficient information.
func (d *binaryDecoder) AddBlocks(blocks []LTBlock) bool {
	for i := range blocks {
		d.matrix.addEquation(d.codec.PickIndices(blocks[i].BlockCode),
			block{data: blocks[i].Data})
	}
	return d.matrix.determined()
}

// Decode extracts the decoded message from the decoder. If the decoder does
// not have sufficient information to produce an output, returns a nil slice.
func (d *binaryDecoder) Decode() []byte {
	if !d.matrix.determined() {
		return nil
	}

	d.matrix.reduce()

	lenLong, lenShort, numLong, numShort := partition(d.messageLength, d.codec.numSourceBlocks)
	return d.matrix.reconstruct(d.messageLength, lenLong, lenShort, numLong, numShort)
}
