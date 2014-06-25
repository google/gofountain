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

// TestBinaryDecoder ensures that the binary fountain produces a round-trip
// decodeable message using the corresponding decoder.
func TestBinaryDecoder(t *testing.T) {
	c := NewBinaryCodec(13)
	message := []byte("abcdefghijklmnopqrstuvwxyz")
	ids := make([]int64, 45)
	random := rand.New(rand.NewSource(8923489))
	for i := range ids {
		ids[i] = int64(random.Intn(100000))
	}

	blocks := EncodeLTBlocks(message, ids, c)
	t.Log("blocks =", blocks)

	d := newBinaryDecoder(c.(*binaryCodec), len(message))

	for i := 0; i < 16; i++ {
		d.AddBlocks([]LTBlock{blocks[i]})
		if testing.Verbose() {
			printMatrix(d.matrix, t)
		}
	}

	d.matrix.reduce()
	t.Log("REDUCE")
	printMatrix(d.matrix, t)

	decoded := d.Decode()
	printMatrix(d.matrix, t)
	if !reflect.DeepEqual(decoded, message) {
		t.Errorf("Decoded message doesn't match original. Got %v, want %v", decoded, message)
	}
}

// TestbinaryDecoderBlockTable tests many combinations of fountain block ID
// combinations to ensure that the codec has the expected reconstruction
// properties.
func TestBinaryDecoderBlockTable(t *testing.T) {
	c := NewBinaryCodec(13)

	message := []byte("abcdefghijklmnopqrstuvwxyz")
	random := rand.New(rand.NewSource(8234923))

	moreBlocksNeeded := 0
	for i := 0; i < 100; i++ {
		r := rand.New(rand.NewSource(random.Int63()))
		ids := make([]int64, 45)
		for i := range ids {
			ids[i] = int64(r.Intn(100000))
		}
		blocks := EncodeLTBlocks(message, ids, c)

		d := newBinaryDecoder(c.(*binaryCodec), len(message))
		d.AddBlocks(blocks[0:30])
		if !d.matrix.determined() {
			moreBlocksNeeded++
			d.AddBlocks(blocks[31:46])
		}
		decoded := d.Decode()
		if !reflect.DeepEqual(decoded, message) {
			t.Errorf("Decoded message doesn't match original. Got %v, want %v", decoded, message)
		}
	}

	if moreBlocksNeeded > 2 {
		t.Errorf("Needed too many high-block-count decoding sequences: %d", moreBlocksNeeded)
	}
}

// TestBinaryDecodeMessageTable tests a large number of source messages to make
// sure they are all reconstructed accurately. This provides assurance that the
// decoder is functioning accurately.
func TestBinaryDecodeMessageTable(t *testing.T) {
	c := NewBinaryCodec(10)
	random := rand.New(rand.NewSource(8234982))
	for i := 0; i < 100; i++ {
		r := rand.New(rand.NewSource(random.Int63()))
		messageLen := r.Intn(1000) + 1000
		message := make([]byte, messageLen)
		for j := 0; j < len(message); j++ {
			message[j] = byte(r.Intn(200))
		}
		ids := make([]int64, 50)
		for i := range ids {
			ids[i] = int64(r.Intn(100000))
		}
		blocks := EncodeLTBlocks(message, ids, c)

		d := newBinaryDecoder(c.(*binaryCodec), len(message))
		d.AddBlocks(blocks[0:25])
		if !d.matrix.determined() {
			t.Errorf("Message should be determined after 25 blocks")
		} else {
			decoded := d.Decode()
			if !reflect.DeepEqual(decoded, message) {
				t.Errorf("Incorrect message decode. Length=%d, message=%v", len(message), message)
			}
		}
	}
}
