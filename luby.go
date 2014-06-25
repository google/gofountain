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

/*
Package fountain includes implementations of several fountain codes.

Fountain codes have the property that a very large (more or less unlimited)
number of code blocks can be generated from a fixed number of source
blocks. The original message can be recovered from any subset of sufficient
size of these code blocks, so even if some code blocks are lost, the message
can still be reconstructed once a sufficient number have been received.
So in a transmission system, the receiver need not notify the
transmitter about every code block, it only need notify the transmitter
when the source message has been fully reconstructed.

The overall approach used by this package is that there are various codec
implementations which follow the same overall algorithm -- splitting a source
message into source blocks, manipulating those source blocks to produce a
set of precode blocks, then for each code block to be produced, picking
constituent precode blocks to use to create the code block, and then using
an LT (Luby Transform) process to produce the code blocks.
*/
package fountain

import (
	"math/rand"
)

// Codec is an interface for fountain codes which follow the general
// scheme of preparing intermediate encoding representations based on the input
// message and picking LT composition indices given an integer code block number.
type Codec interface {
	// SourceBlocks returns the number of source blocks to be used in the
	// codec. Note that this may not be the same number as the number of intermediate
	// code blocks. It is the minimum number of encoded blocks necessary to
	// reconstruct the original message.
	SourceBlocks() int

	// GenerateIntermediateBlocks prepares a set of precode blocks given the input
	// message. The precode blocks may just be identically the blocks in the
	// original message, or it may be a transformation on those source blocks
	// derived from a codec-specific relationship.
	// For example, in Online codes, this consists of adding auxiliary blocks.
	// In a Raptor code, the entire set of source blocks is transformed into a
	// different set of precode blocks.
	GenerateIntermediateBlocks(message []byte, numBlocks int) []block

	// PickIndices asks the codec to select the (non-strict subset of the) precode
	// blocks to be used in the LT composition of a particular code block. These
	// blocks will then be XORed to produce the code block.
	PickIndices(codeBlockIndex int64) []int

	// NewDecoder creates a decoder suitable for use with blocks encoded using this
	// codec for a known message size (in bytes). The decoder will be initialized
	// and ready to receive incoming blocks for decoding.
	NewDecoder(messageLength int) Decoder
}

// LTBlock is an encoded block structure representing a block created using
// the LT transform.
type LTBlock struct {
	// BlockCode is the ID used to construct the encoded block.
	// The way in which the ID is mapped to the choice of blocks will vary by
	// codec.
	BlockCode int64

	// Data is the contents of the encoded block.
	Data []byte
}

// Decoder is an interface allowing decoding of fountain-code-encoded messages
// as the blocks are received.
type Decoder interface {
	// AddBlocks adds a set of encoded blocks to the decoder. Returns true if the
	// message can be fully decoded. False if there is insufficient information.
	AddBlocks(blocks []LTBlock) bool

	// Decode extracts the decoded message from the decoder. If the decoder does
	// not have sufficient information to produce an output, returns a nil slice.
	Decode() []byte
}

////////////////////////////////////////////////////////////////////////////////
// Implementation of Luby Transform codes.
// The Luby Transform (LT) converts a source text split into a number of source
// blocks into an unbounded set of code blocks, each of which is formed of an
// XOR operation of some subset of the source blocks.
// See "LT Codes" -- M. Luby (2002)

// lubyCodec contains the codec information for the Luby Transform encoder
// and decoder.
// Implements fountain.Codec.
type lubyCodec struct {
	// sourceBlocks is the number of source blocks (N) the source message is split into.
	sourceBlocks int

	// random is a source of randomness for sampling the degree distribution
	// and the source blocks when composing a code block.
	random *rand.Rand

	// degreeCDF is the degree distribution function from which encoding block
	// compositions are chosen.
	degreeCDF []float64
}

// NewLubyCodec creates a new Codec using the provided number of source blocks,
// PRNG, and degree distribution function.
// The intermediate blocks will be a roughly-equal-sized partition of the source
// message padded so that all blocks have equal size. The indices will be picked
// using the provided PRNG seeded with the BlockCode ID of the LTBlock
// to be created, according to the degree CDF provided.
func NewLubyCodec(sourceBlocks int, random *rand.Rand, degreeCDF []float64) Codec {
	return &lubyCodec{
		sourceBlocks: sourceBlocks,
		random:       random,
		degreeCDF:    degreeCDF}
}

// SourceBlocks retrieves the number of source blocks the codec is configured to use.
func (c *lubyCodec) SourceBlocks() int {
	return c.sourceBlocks
}

// PickIndices uses the provided PRNG to select a random number of source
// blocks with degree d, given by a random selection in the degreeCDF parameter.
// The degree distribution is how likely the encoder is to pick code blocks composed
// of d source blocks.
func (c *lubyCodec) PickIndices(codeBlockIndex int64) []int {
	c.random.Seed(codeBlockIndex)
	d := pickDegree(c.random, c.degreeCDF)
	return sampleUniform(c.random, d, c.sourceBlocks)
}

// GenerateIntermediateEncoding for the LubyCodec simply splits the source message
// into numBlocks blocks of roughly equal size, padding shorter ones so that all
// blocks are the same length.
func (c *lubyCodec) GenerateIntermediateBlocks(message []byte, numBlocks int) []block {
	long, short := partitionBytes(message, c.sourceBlocks)
	return equalizeBlockLengths(long, short)
}

// generateLubyTransformBlock generates a single code block from the set of
// source blocks, given the composition indices, by XORing the source blocks
// together.
func generateLubyTransformBlock(source []block, indices []int) block {
	var symbol block

	for _, i := range indices {
		if i < len(source) {
			symbol.xor(source[i])
		}
	}

	return symbol
}

// EncodeLTBlocks encodes a sequence of LT-encoded code blocks from the given message
// and the block IDs. Suitable for use with any fountain.Codec.
// Note: This method is destructive to the message array.
func EncodeLTBlocks(message []byte, encodedBlockIDs []int64, c Codec) []LTBlock {
	source := c.GenerateIntermediateBlocks(message, c.SourceBlocks())

	ltBlocks := make([]LTBlock, len(encodedBlockIDs))
	for i := range encodedBlockIDs {
		indices := c.PickIndices(encodedBlockIDs[i])
		ltBlocks[i].BlockCode = encodedBlockIDs[i]
		b := generateLubyTransformBlock(source, indices)
		ltBlocks[i].Data = make([]byte, b.length())
		copy(ltBlocks[i].Data, b.data)
	}
	return ltBlocks
}

// NewDecoder creates a luby transform decoder
func (c *lubyCodec) NewDecoder(messageLength int) Decoder {
	return newLubyDecoder(c, messageLength)
}

// lubyDecoder is the state required to decode a Luby Transform message.
type lubyDecoder struct {
	codec         *lubyCodec
	messageLength int

	// The sparse equation matrix used for decoding.
	matrix sparseMatrix
}

// newLubyDecoder creates a new decoder for a particular Luby Transform message.
// The codec parameters used to create the original encoding blocks must be provided.
// The decoder is only valid for decoding code blocks for a particular message.
func newLubyDecoder(c *lubyCodec, length int) *lubyDecoder {
	d := &lubyDecoder{codec: c, messageLength: length}
	d.matrix.coeff = make([][]int, c.SourceBlocks())
	d.matrix.v = make([]block, c.SourceBlocks())

	return d
}

// AddBlocks adds a set of encoded blocks to the decoder. Returns true if the
// message can be fully decoded. False if there is insufficient information.
func (d *lubyDecoder) AddBlocks(blocks []LTBlock) bool {
	for i := range blocks {
		indices := d.codec.PickIndices(blocks[i].BlockCode)
		d.matrix.addEquation(indices, block{data: blocks[i].Data})
	}
	return d.matrix.determined()
}

// Decode extracts the decoded message from the decoder. If the decoder does
// not have sufficient information to produce an output, returns a nil slice.
func (d *lubyDecoder) Decode() []byte {
	if !d.matrix.determined() {
		return nil
	}

	d.matrix.reduce()

	lenLong, lenShort, numLong, numShort := partition(d.messageLength, d.codec.SourceBlocks())
	return d.matrix.reconstruct(d.messageLength, lenLong, lenShort, numLong, numShort)
}
