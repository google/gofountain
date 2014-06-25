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

// A block represents a contiguous range of data being encoded or decoded,
// or a block of coded data. Details of how the source text is split into blocks
// is governed by the particular fountain code used.
type block struct {
	// Data content of this source or code block.
	data []byte

	// How many padding bytes this block has at the end.
	padding int
}

// newBlock creates a new block with a given length. The block will initially be
// all padding.
func newBlock(len int) *block {
	return &block{padding: len}
}

// length returns the length of the block in bytes. Counts data bytes as well
// as any padding.
func (b *block) length() int {
	return len(b.data) + b.padding
}

func (b *block) empty() bool {
	return b.length() == 0
}

// A common operation is to XOR entire code blocks together with other blocks.
// When this is done, padding bytes count as 0 (that is XOR identity), and the
// destination block will be modified so that its data is large enough to
// contain the result of the XOR.
func (b *block) xor(a block) {
	if len(b.data) < len(a.data) {
		var inc = len(a.data) - len(b.data)
		b.data = append(b.data, make([]byte, inc)...)
		if b.padding > inc {
			b.padding -= inc
		} else {
			b.padding = 0
		}
	}

	for i := 0; i < len(a.data); i++ {
		b.data[i] ^= a.data[i]
	}
}

// partitionBytes partitions an input text into a sequence of p blocks. The
// sizes of the blocks will be given by the partition() function. The last
// block may have padding.
// Return values: the slice of longer blocks, the slice of shorter blocks.
// Within each block slice, all will have uniform lengths.
func partitionBytes(in []byte, p int) ([]block, []block) {
	sliceIntoBlocks := func(in []byte, num, length int) ([]block, []byte) {
		blocks := make([]block, num)
		for i := range blocks {
			if len(in) > length {
				blocks[i].data, in = in[:length], in[length:]
			} else {
				blocks[i].data, in = in, []byte{}
			}
			if len(blocks[i].data) < length {
				blocks[i].padding = length - len(blocks[i].data)
			}
		}
		return blocks, in
	}

	lenLong, lenShort, numLong, numShort := partition(len(in), p)
	long, in := sliceIntoBlocks(in, numLong, lenLong)
	short, _ := sliceIntoBlocks(in, numShort, lenShort)
	return long, short
}

// equalizeBlockLengths adds padding to all short blocks to make them equal in
// size to the long blocks. The caller should ensure that all the longBlocks
// have the same length.
// Returns a block slice containing all the long and short blocks.
func equalizeBlockLengths(longBlocks, shortBlocks []block) []block {
	if len(longBlocks) == 0 {
		return shortBlocks
	}
	if len(shortBlocks) == 0 {
		return longBlocks
	}

	for i := range shortBlocks {
		shortBlocks[i].padding += longBlocks[0].length() - shortBlocks[i].length()
	}

	blocks := make([]block, len(longBlocks)+len(shortBlocks))
	copy(blocks, longBlocks)
	copy(blocks[len(longBlocks):], shortBlocks)
	return blocks
}

// sparseMatrix is the block decoding data structure. It is a sparse matrix of
// XOR equations. The coefficients are the indices of the source blocks which
// are XORed together to produce the values. So if equation _i_ of the matrix is
// block_0 ^ block_2 ^ block_3 ^ block_9 = [0xD2, 0x38]
// that would be represented as coeff[i] = [0, 2, 3, 9], v[i].data = [0xD2, 0x38]
// Example: The sparse coefficient matrix
// | 0 1 1 0 |
// | 0 1 0 0 |
// | 0 0 1 1 |
// | 0 0 0 1 |
// would be represented as
// [ [ 1, 2],
//   [ 1 ],
//   [ 2, 3],
//   [ 3 ]]
// Every row has M[i][0] >= i. If we added components [2] to this matrix, it
// would replace the M[2] row ([2, 3]), and then the resulting component vector
// after cancellation against that row ([3]) would be used instead of the
// original equation. In this case, M[3] is already populated, so the new
// equation is redundant (and could be used for ECC, theoretically), but if we
// didn't have an entry in M[3], it would be placed there.
// The values were omitted from this discussion, but they follow along by doing
// XOR operations as the components are reduced during insertion.
type sparseMatrix struct {
	coeff [][]int
	v     []block
}

// xorRow performs a reduction of the given candidate equation (indices, b)
// with the specified matrix row (index s). It does so by XORing the values,
// and then taking the symmetric difference of the coefficients of that matrix
// row and the provided indices. (That is, the "set XOR".) Assumes both
// coefficient slices are sorted.
func (m *sparseMatrix) xorRow(s int, indices []int, b block) ([]int, block) {
	b.xor(m.v[s])

	var newIndices []int
	coeffs := m.coeff[s]
	var i, j int
	for i < len(coeffs) && j < len(indices) {
		index := indices[j]
		if coeffs[i] == index {
			i++
			j++
		} else if coeffs[i] < index {
			newIndices = append(newIndices, coeffs[i])
			i++
		} else {
			newIndices = append(newIndices, index)
			j++
		}
	}

	newIndices = append(newIndices, coeffs[i:]...)
	newIndices = append(newIndices, indices[j:]...)
	return newIndices, b
}

// addEquation adds an XOR equation to the decode matrix. The online decode
// strategy is a variant of that of Bioglio, Grangetto, and Gaeta
// (http://www.di.unito.it/~bioglio/Papers/CL2009-lt.pdf) It maintains the
// invariant that either coeff[i][0] == i or len(coeff[i]) == 0. That is, while
// adding an equation to the matrix, it ensures that the decode matrix remains
// triangular.
func (m *sparseMatrix) addEquation(components []int, b block) {
	// This loop reduces the incoming equation by XOR until it either fits into
	// an empty row in the decode matrix or is discarded as redundant.
	for len(components) > 0 && len(m.coeff[components[0]]) > 0 {
		s := components[0]
		if len(components) >= len(m.coeff[s]) {
			components, b = m.xorRow(s, components, b)
		} else {
			// Swap the existing row for the new one, reduce the existing one and
			// see if it fits elsewhere.
			components, m.coeff[s] = m.coeff[s], components
			b, m.v[s] = m.v[s], b
		}
	}

	if len(components) > 0 {
		m.coeff[components[0]] = components
		m.v[components[0]] = b
	}
}

// Check to see if the decode matrix is fully specified. This is true when
// all rows have non-empty coefficient slices.
// TODO(gbillock): is there a weakness here if an auxiliary block is unpopulated?
func (m *sparseMatrix) determined() bool {
	for _, r := range m.coeff {
		if len(r) == 0 {
			return false
		}
	}
	return true
}

// reduce performs Gaussian Elimination over the whole matrix. Presumes
// the matrix is triangular, and that the method is not called unless there is
// enough data for a solution.
// TODO(gbillock): Could profitably do this online as well?
func (m *sparseMatrix) reduce() {
	for i := len(m.coeff) - 1; i >= 0; i-- {
		for j := 0; j < i; j++ {
			ci, cj := m.coeff[i], m.coeff[j]
			for k := 1; k < len(cj); k++ {
				if cj[k] == ci[0] {
					m.v[j].xor(m.v[i])
					continue
				}
			}
		}
		// All but the leading coefficient in the rows have been reduced out.
		m.coeff[i] = m.coeff[i][0:1]
	}
}

// reconstruct pastes the fully reduced values in the sparse matrix result column
// into a new byte array and returns it. The length/number parameters are typically
// those given by partition().
// lenLong is how many long blocks there are.
// lenShort is how many short blocks there are (following the long blocks).
// numLong is how many bytes are in the long blocks.
// numShort is how many bytes the short blocks are.
func (m *sparseMatrix) reconstruct(totalLength, lenLong, lenShort, numLong, numShort int) []byte {
	out := make([]byte, totalLength)
	out = out[0:0]
	for i := 0; i < numLong; i++ {
		out = append(out, m.v[i].data[0:lenLong]...)
	}
	for i := numLong; i < numLong+numShort; i++ {
		out = append(out, m.v[i].data[0:lenShort]...)
	}

	return out
}
