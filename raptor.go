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
	"sort"
)

// The Raptor fountain code (also called the R10 code) from RFC 5053.
// This code nearly matches the performance of the random binary fountain, but
// does so with a hard cap on the degree distribution on code blocks, meaning
// that the decode process becomes linear instead of quadratic.
//
// In addition, it is a systematic code, meaning that the original source blocks
// are part of the encoding. This enables the source blocks to be sent simply,
// and then repair blocks constructed as needed using the code.
//
// A limitation is that the code supports a maximum of 8192 source blocks,
// thus requiring very large source messages to be split up into sub-messages if
// smaller packet sizes are a goal. Performance varies from the random fountain
// the most for higher loss rates and smaller numbers of source blocks. A reasonable
// expectation is that the encoding overhead due to using the code is a few percent.
//
// When encoding the message, the codec takes a list of block IDs and returns a
// set of raptor coded blocks generated according to the RFC. The contract in the
// RFC is that ids from 0 to K (codec.NumSourceSymbols) are identical to the source
// symbols. (That's what makes it a systematic code.) Such symbols should not be in
// the same packet as repair symbols with id >= K.
//
// A typical usage in a transmission system might be to just split the message and
// send the first K symbols normally. Then create the codec and generate repair
// symbols using random ESI values >= K until the message is reconstructed by
// the receiver.
//
// The BlockCode in the resulting LTBlocks will be a uint16-compatible value.
//
// IMPORTANT NOTE: encoding is destructive to the input message.

// raptorCodec describes the parameters needed to construct a raptor code. The codec
// governs the production of an unbounded set of LTBlocks from a given source message.
// If the total transfer size is such that the application wants to split the message
// into sub-blocks before transfer, that should be done independently of this codec
// in accordance with the instructions in RFC 5053. For many uses, there will be a
// value of NumSourceSymbols and SymbolAlignmentSize such that the resulting LTBlock
// size is of an acceptable length. Note: the RFC provides a way to pack multiple
// symbols per packet. If packet erasure is the loss model for the code, this
// ends up behaving like choosing a smaller NumSourceSymbols, in which case it's
// simpler to pass just one code symbol per packet.
// Implements fountain.Codec
type raptorCodec struct {
	// SymbolAlignmentSize = Al is the size of each symbol in the source message in bytes.
	// Usually 4. This is the XOR granularity in bytes. On 32-byte machines 4-byte XORs
	// will be most efficient. On the other hand, the code will perform with less overhead
	// with larger numbers of source blocks.
	SymbolAlignmentSize int

	// NumSourceSymbols = K. Must be in the range [4, 8192] (inclusive). This is
	// how many source symbols the input message will be divided into. If NumSourceSymbols
	// doesn't evenly divide the length of the message in units of SymbolAlignmentSize,
	// there will be null padding applied to the block.
	NumSourceSymbols int
}

// NewRaptorCodec creates a new R10 raptor codec using the provided number of
// source blocks and alignment size.
func NewRaptorCodec(sourceBlocks int, alignmentSize int) Codec {
	return &raptorCodec{
		NumSourceSymbols:    sourceBlocks,
		SymbolAlignmentSize: alignmentSize}
}

// SourceBlocks returns the number of source symbols used by the codec.
func (c *raptorCodec) SourceBlocks() int {
	return c.NumSourceSymbols
}

// RAND function from section 5.4.4.1
// x, i should be non-negative, m positive.
// Produces a pseudo-random value in the range [0, m-1]
func raptorRand(x, i, m uint32) uint32 {
	v0 := v0table[(x+i)%256]
	v1 := v1table[((x/256)+i)%256]
	return (v0 ^ v1) % m
}

// Deg function from section 5.4.4.2
// deg calculates the degree to be used in code block generation.
func deg(v uint32) int {
	f := [...]uint32{0, 10241, 491582, 712794, 831695, 948446, 1032189, 1048576}
	d := [...]int{0, 1, 2, 3, 4, 10, 11, 40}

	for j := 1; j < len(f)-1; j++ {
		if v < f[j] {
			return d[j]
		}
	}

	return d[len(d)-1]
}

// From RFC section 5.4.2.3 This function computes L, S, and H from K.
// K is the number of source symbols (limited to 2**16).
// The return values are:
// L is the number of intermediate symbols desired (K+S+H), followed by
// S, the number of LDPC symbols, followed by
// H, the number of half-symbols.
func intermediateSymbols(k int) (int, int, int) {
	// X is the smallest positive integer such that X*(X-1) >= 2*K
	x := int(math.Floor(math.Sqrt(2 * float64(k))))
	if x < 1 {
		x = 1
	}

	for (x * (x - 1)) < (2 * k) {
		x++
	}

	// S is the smallest prime such that S >= ceil(0.01*K) + X
	s := int(math.Ceil(0.01*float64(k))) + x
	s = smallestPrimeGreaterOrEqual(s)

	// H is the smallest integer such that choose(H, ceil(H/2)) >= K + S
	// choose(h, h/2) <= 4^(h/2), so begin with
	// h/2 ln(4) = ln K+S
	// h = ln(K+S)/ln(4)
	h := int(math.Floor(math.Log(float64(s)+float64(k)) / math.Log(4)))
	for centerBinomial(h) < k+s {
		h++
	}

	return k + s + h, s, h
}

// Triple generator from RFC section 5.4.4.4
// k is the number of source symbols.
// x is the (random) code symbol ID.
// The generator creates values (d, a, b) to be used in constructing intermediate blocks.
func tripleGenerator(k int, x uint16) (int, uint32, uint32) {
	l, _, _ := intermediateSymbols(k)
	lprime := smallestPrimeGreaterOrEqual(l)
	q := uint32(65521) // largest prime < 2^16
	jk := uint32(systematicIndextable[k])

	a := uint32((53591 + (uint64(jk) * 997)) % uint64(q))
	b := (10267 * (jk + 1)) % q
	y := uint32((uint64(b) + (uint64(x) * uint64(a))) % uint64(q))
	v := raptorRand(y, 0, 1048576) // 1048576 == 2^20
	d := deg(v)
	a = 1 + raptorRand(y, 1, uint32(lprime-1))
	b = raptorRand(y, 2, uint32(lprime))

	return d, a, b
}

// findLTIndices discovers the composition of the ESI=x LT code block for a
// raptor code. k is the number of source blocks.
func findLTIndices(k int, x uint16) []int {
	l, _, _ := intermediateSymbols(k)
	lprime := uint32(smallestPrimeGreaterOrEqual(l))
	d, a, b := tripleGenerator(k, x)

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

// ltEncode is the LT encoding function. RFC section 5.4.4.3
// c is the intermediate symbol vector, k is the number of source symbols.
// x is the symbol ID we are generating.
// The output is an code block containing the bytes of that symbol.
func ltEncode(k int, x uint16, c []block) block {
	indices := findLTIndices(k, x)

	result := block{}
	for _, i := range indices {
		result.xor(c[i])
	}

	return result
}

// raptorIntermediateBlocks takes the source blocks and follows the algorithm
// described in the RFC to generate the intermediate encoding which is then used
// as the LT source encoding for the systematic code. The properties of the intermediate
// encoding is that we will create L resulting blocks from K source blocks.
// The first K intermediate blocks have an LT relationship with the K source blocks --
// we basically use an unsystematic LT code using the tripleGenerator values where
// the X "ESI" is simply the source block index. We then generate S+H additional
// intermediate blocks.
//
// The S blocks are a low density parity check group of blocks which have contributions
// from three of the first K intermediate blocks arranged in successive clusters.
// This low density arrangement cycles column-wise in the decode matrix, meaning
// that there is extra decode-friendly information in the decode matrix to make the
// expected decoding operations use a sparse enough matrix so that it is efficient.
//
// The H blocks are composed in the decode matrix of many (half) of the first K
// intermediate blocks. The coding is taken from a gray code, and is intended to
// be more-or-less equivalent to a random binary fountain from the coding
// perspective, so when these blocks are chosen in the coded blocks, it ensures
// good performance by making sure there are no unrepresented source blocks.
//
// We then decode this -- the J(K) systematic index values are chosen such that
// the first L=K+S+H members of this set will produce an invertible decode matrix.
// The guarantees that the re-encoding of the first K code symbols are equal to
// the source symbols (ensuring a systematic code), and that the analysis of the
// overall code will follow the performance of the random binary fountain closely.
//
// This method is destructive to the source blocks.
func raptorIntermediateBlocks(source []block) []block {
	ltdecoder := newRaptorDecoder(&raptorCodec{SymbolAlignmentSize: 1,
		NumSourceSymbols: len(source)}, 1)
	for i := 0; i < len(source); i++ {
		indices := findLTIndices(len(source), uint16(i))
		ltdecoder.matrix.addEquation(indices, source[i])
	}

	ltdecoder.matrix.reduce()

	// panics if ~ltdecoder.determined. The J(K) selection should ensure that
	// never happens.
	intermediate := ltdecoder.matrix.v
	return intermediate
}

// GenerateIntermediateBlocks creates the pre-code representation given the
// message argument blocks. For the raptor code, this pre-code is generated by
// a reverse-coding process which ensures that for BlockCode=0, the 0th block of
// the incoming message is produced, and so on up to the 'len(message)-1'th BlockCode.
func (c *raptorCodec) GenerateIntermediateBlocks(message []byte, numBlocks int) []block {
	sourceLong, sourceShort := partitionBytes(message, numBlocks)
	source := equalizeBlockLengths(sourceLong, sourceShort)
	return raptorIntermediateBlocks(source)
}

// PickIndices chooses a set of indices for the provided CodeBlock index value
// which are used to compose an LTBlock. It functions by
func (c *raptorCodec) PickIndices(codeBlockIndex int64) []int {
	return findLTIndices(int(c.SourceBlocks()), uint16(codeBlockIndex))
}

// NewDecoder creates a new raptor decoder
func (c *raptorCodec) NewDecoder(messageLength int) Decoder {
	return newRaptorDecoder(c, messageLength)
}

// raptorDecoder is the state required for decoding a particular message prepared
// with the Raptor code. It must be initialized with the same raptorCodec parameters
// used for encoding, as well as the expected message length.
type raptorDecoder struct {
	codec         raptorCodec
	messageLength int

	// The sparse equation matrix used for decoding.
	matrix sparseMatrix
}

// newRaptorDecoder creates a new raptor decoder for a given message. The
// codec supplied must be the same one as the message was encoded with.
func newRaptorDecoder(c *raptorCodec, length int) *raptorDecoder {
	d := &raptorDecoder{codec: *c, messageLength: length}

	l, s, h := intermediateSymbols(c.NumSourceSymbols)

	// Add the S + H intermediate symbol composition equations.
	d.matrix.coeff = make([][]int, l)
	d.matrix.v = make([]block, l)

	k := c.NumSourceSymbols
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
		compositions[i] = append(compositions[i], k+i)
		d.matrix.addEquation(compositions[i], block{})
	}

	compositions = make([][]int, h)

	hprime := int(math.Ceil(float64(h) / 2))
	m := buildGraySequence(k+s, hprime)
	for i := 0; i < h; i++ {
		for j := 0; j < k+s; j++ {
			if bitSet(uint(m[j]), uint(i)) {
				compositions[i] = append(compositions[i], j)
			}
		}
		compositions[i] = append(compositions[i], k+s+i)
		d.matrix.addEquation(compositions[i], block{})
	}

	return d
}

// AddBlocks adds a set of encoded blocks to the decoder. Returns true if the
// message can be fully decoded. False if there is insufficient information.
func (d *raptorDecoder) AddBlocks(blocks []LTBlock) bool {
	for i := range blocks {
		indices := findLTIndices(d.codec.NumSourceSymbols, uint16(blocks[i].BlockCode))
		d.matrix.addEquation(indices, block{data: blocks[i].Data})
	}
	return d.matrix.determined()
}

// Decode extracts the decoded message from the decoder. If the decoder does
// not have sufficient information to produce an output, returns a nil slice.
func (d *raptorDecoder) Decode() []byte {
	if !d.matrix.determined() {
		return nil
	}

	d.matrix.reduce()

	// Now the intermediate blocks are held in d.matrix.v. Use the encoder function
	// to recover the source blocks.
	intermediate := d.matrix.v
	source := make([]block, d.codec.NumSourceSymbols)
	for i := 0; i < d.codec.NumSourceSymbols; i++ {
		source[i] = ltEncode(d.codec.NumSourceSymbols, uint16(i), intermediate)
	}

	lenLong, lenShort, numLong, numShort := partition(d.messageLength, d.codec.NumSourceSymbols)
	out := make([]byte, d.messageLength)
	out = out[0:0]
	for i := 0; i < numLong; i++ {
		out = append(out, source[i].data[0:lenLong]...)
	}
	for i := numLong; i < numLong+numShort; i++ {
		out = append(out, source[i].data[0:lenShort]...)
	}
	return out
}
