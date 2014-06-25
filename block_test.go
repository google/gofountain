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
	"bytes"
	"reflect"
	"testing"
)

func TestBlockLength(t *testing.T) {
	var lengthTests = []struct {
		b   block
		len int
	}{
		{block{}, 0},
		{block{[]byte{1, 0, 1}, 0}, 3},
		{block{[]byte{1, 0, 1}, 1}, 4},
	}

	for _, i := range lengthTests {
		if i.b.length() != i.len {
			t.Errorf("Length of b is %d, should be %d", i.b.length(), i.len)
		}
		if (i.len == 0) != i.b.empty() {
			t.Errorf("Emptiness check error. Got %v, want %v", i.b.empty(), i.len == 0)
		}
	}
}

func TestBlockXor(t *testing.T) {
	var xorTests = []struct {
		a   block
		b   block
		out block
	}{
		{block{[]byte{1, 0, 1}, 0}, block{[]byte{1, 1, 1}, 0}, block{[]byte{0, 1, 0}, 0}},
		{block{[]byte{1}, 0}, block{[]byte{0, 14, 6}, 0}, block{[]byte{1, 14, 6}, 0}},
		{block{}, block{[]byte{100, 200}, 0}, block{[]byte{100, 200}, 0}},
		{block{[]byte{}, 5}, block{[]byte{0, 1, 0}, 0}, block{[]byte{0, 1, 0}, 2}},
		{block{[]byte{}, 5}, block{[]byte{0, 1, 0, 2, 3}, 0}, block{[]byte{0, 1, 0, 2, 3}, 0}},
		{block{[]byte{}, 5}, block{[]byte{0, 1, 0, 2, 3, 7}, 0}, block{[]byte{0, 1, 0, 2, 3, 7}, 0}},
		{block{[]byte{1}, 4}, block{[]byte{0, 1, 0, 2, 3, 7}, 0}, block{[]byte{1, 1, 0, 2, 3, 7}, 0}},
	}

	for _, i := range xorTests {
		t.Logf("...Testing %v XOR %v", i.a, i.b)
		originalLength := i.a.length()
		i.a.xor(i.b)
		if i.a.length() < originalLength {
			t.Errorf("Length shrunk. Got %d, want length >= %d", i.a.length(), originalLength)
		}
		if len(i.a.data) != len(i.b.data) {
			t.Errorf("a and b data should be same length after xor. a len=%d, b len=%d", len(i.a.data), len(i.b.data))
		}

		if !bytes.Equal(i.a.data, i.out.data) {
			t.Errorf("XOR value is %v : should be %v", i.a.data, i.out.data)
		}
	}
}

func TestPartitionBytes(t *testing.T) {
	a := make([]byte, 100)
	for i := 0; i < len(a); i++ {
		a[i] = byte(i)
	}

	var partitionTests = []struct {
		numPartitions     int
		lenLong, lenShort int
	}{
		{11, 1, 10},
		{3, 1, 2},
	}

	for _, i := range partitionTests {
		t.Logf("Partitioning %v into %d", a, i.numPartitions)
		long, short := partitionBytes(a, i.numPartitions)
		if len(long) != i.lenLong {
			t.Errorf("Got %d long blocks, should have %d", len(long), i.lenLong)
		}
		if len(short) != i.lenShort {
			t.Errorf("Got %d short blocks, should have %d", len(short), i.lenShort)
		}
		if short[len(short)-1].padding != 0 {
			t.Errorf("Should fit blocks exactly, have last padding %d", short[len(short)-1].padding)
		}
		if long[0].data[0] != 0 {
			t.Errorf("Long block should be first. First value is %v", long[0].data)
		}
	}
}

func TestEqualizeBlockLengths(t *testing.T) {
	b := []byte("abcdefghijklmnopq")
	var equalizeTests = []struct {
		numPartitions int
		length        int
		padding       int
	}{
		{1, 17, 0},
		{2, 9, 1},
		{3, 6, 1},
		{4, 5, 1},
		{5, 4, 1},
		{6, 3, 1},
		{7, 3, 1},
		{8, 3, 1},
		{9, 2, 1},
		{10, 2, 1},
		{16, 2, 1},
		{17, 1, 0},
	}

	for _, i := range equalizeTests {
		long, short := partitionBytes(b, i.numPartitions)
		blocks := equalizeBlockLengths(long, short)
		if len(blocks) != i.numPartitions {
			t.Errorf("Got %d blocks, should have %d", len(blocks), i.numPartitions)
		}
		for k := range blocks {
			if blocks[k].length() != i.length {
				t.Errorf("Got block length %d for block %d, should be %d",
					blocks[0].length(), k, i.length)
			}
		}
		if blocks[len(blocks)-1].padding != i.padding {
			t.Errorf("Padding of last block is %d, should be %d",
				blocks[len(blocks)-1].padding, i.padding)
		}
	}
}

func printMatrix(m sparseMatrix, t *testing.T) {
	t.Log("------- matrix -----------")
	for i := range m.coeff {
		t.Logf("%v = %v\n", m.coeff[i], m.v[i].data)
	}
}

func TestMatrixXorRow(t *testing.T) {
	var xorRowTests = []struct {
		arow   []int
		r      []int
		result []int
	}{
		{[]int{0, 1}, []int{2, 3}, []int{0, 1, 2, 3}},
		{[]int{0, 1}, []int{1, 2, 3}, []int{0, 2, 3}},
		{[]int{}, []int{1, 2, 3}, []int{1, 2, 3}},
		{[]int{1, 2, 3}, []int{}, []int{1, 2, 3}},
		{[]int{1}, []int{2}, []int{1, 2}},
		{[]int{1}, []int{1}, []int{}},
		{[]int{1, 2}, []int{1, 2, 3, 4}, []int{3, 4}},
		{[]int{3, 4}, []int{1, 2, 3, 4}, []int{1, 2}},
		{[]int{1, 2, 3, 4}, []int{1, 2}, []int{3, 4}},
		{[]int{0, 1, 2, 3, 4}, []int{1, 2}, []int{0, 3, 4}},
		{[]int{3, 4}, []int{1, 2, 3, 4, 5}, []int{1, 2, 5}},
		{[]int{3, 4, 8}, []int{1, 2, 3, 4, 5}, []int{1, 2, 5, 8}},
	}

	for _, test := range xorRowTests {
		m := sparseMatrix{coeff: [][]int{test.arow}, v: []block{block{[]byte{1}, 0}}}

		testb := block{[]byte{2}, 0}
		test.r, testb = m.xorRow(0, test.r, testb)

		// Needed since under DeepEqual the nil and the empty slice are not equal.
		if test.r == nil {
			test.r = make([]int, 0)
		}
		if !reflect.DeepEqual(test.r, test.result) {
			t.Errorf("XOR row result got %v, should be %v", test.r, test.result)
		}
		if !reflect.DeepEqual(testb, block{[]byte{3}, 0}) {
			t.Errorf("XOR row block got %v, should be %v", testb, block{[]byte{3}, 0})
		}
	}
}

func TestMatrixBasic(t *testing.T) {
	m := sparseMatrix{coeff: [][]int{{}, {}}, v: []block{block{}, block{}}}

	m.addEquation([]int{0}, block{data: []byte{1}})
	if m.determined() {
		t.Errorf("2-row matrix should not be determined after 1 equation")
		printMatrix(m, t)
	}

	m.addEquation([]int{0, 1}, block{data: []byte{2}})
	if !m.determined() {
		t.Errorf("2-row matrix should be determined after 2 equations")
		printMatrix(m, t)
	}

	printMatrix(m, t)

	if !reflect.DeepEqual(m.coeff[0], []int{0}) ||
		!reflect.DeepEqual(m.v[0].data, []byte{1}) {
		t.Errorf("Equation 0 got (%v = %v), want ([0] = [1])", m.coeff[0], m.v[0].data)
	}
	if !reflect.DeepEqual(m.coeff[1], []int{1}) ||
		!reflect.DeepEqual(m.v[1].data, []byte{3}) {
		t.Errorf("Equation 1 got (%v = %v), want ([1] = [3])", m.coeff[0], m.v[0].data)
	}

	m.reduce()
	if !reflect.DeepEqual(m.coeff[0], []int{0}) ||
		!reflect.DeepEqual(m.v[0].data, []byte{1}) {
		t.Errorf("Equation 0 got (%v = %v), want ([0] = [1])", m.coeff[0], m.v[0].data)
	}
	if !reflect.DeepEqual(m.coeff[1], []int{1}) ||
		!reflect.DeepEqual(m.v[1].data, []byte{3}) {
		t.Errorf("Equation 1 got (%v = %v), want ([1] = [3])", m.coeff[0], m.v[0].data)
	}
}

func TestMatrixLarge(t *testing.T) {
	m := sparseMatrix{coeff: make([][]int, 4), v: make([]block, 4)}

	m.addEquation([]int{2, 3}, block{data: []byte{1}})
	m.addEquation([]int{2}, block{data: []byte{2}})
	if m.determined() {
		t.Errorf("4-row matrix should not be determined after 2 equations")
		printMatrix(m, t)
	}
	printMatrix(m, t)

	// Should have triangular entries in {2} and {3} now.
	if len(m.coeff[2]) != 1 || m.v[2].data[0] != 2 {
		t.Errorf("Equation 2 got %v = %v, should be [2] = [2]", m.coeff[2], m.v[2])
	}
	if len(m.coeff[3]) != 1 || m.v[3].data[0] != 3 {
		t.Errorf("Equation 3 got %v = %v, should be [3] = [3]", m.coeff[3], m.v[3])
	}
	if len(m.coeff[0]) != 0 || len(m.coeff[1]) != 0 {
		t.Errorf("Equations 0 and 1 should be empty")
		printMatrix(m, t)
	}

	m.addEquation([]int{0, 1, 2, 3}, block{data: []byte{4}})
	if m.determined() {
		t.Errorf("4-row matrix should not be determined after 3 equations")
		printMatrix(m, t)
	}

	m.addEquation([]int{3}, block{data: []byte{3}})
	if m.determined() {
		t.Errorf("4-row matrix should not be determined after redundant equation")
		printMatrix(m, t)
	}

	m.addEquation([]int{0, 2}, block{data: []byte{8}})
	if !m.determined() {
		t.Errorf("4-row matrix should be determined after 4 equations")
		printMatrix(m, t)
	}

	// The matrix should now have entries in rows 0 and 1, but not equal to the
	// original equations.
	printMatrix(m, t)
	if !reflect.DeepEqual(m.coeff[0], []int{0, 2}) {
		t.Errorf("Got %v for coeff[0], expect [0, 2]", m.coeff[0])
	}
	if !reflect.DeepEqual(m.coeff[1], []int{1, 3}) {
		t.Errorf("Got %v for coeff[1], expect [1, 3]", m.coeff[1])
	}
}
