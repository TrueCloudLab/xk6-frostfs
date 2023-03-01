package datagen

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
)

type (
	// Generator stores buffer of random bytes with some tail and returns data slices with
	// an increasing offset so that we receive a different set of bytes after each call without
	// re-generation of the entire buffer from scratch:
	//
	//   [<----------size----------><-tail->]
	//   [<----------slice0-------->........]
	//   [.<----------slice1-------->.......]
	//   [..<----------slice2-------->......]
	Generator struct {
		vu     modules.VU
		size   int
		buf    []byte
		offset int
	}

	GenPayloadResponse struct {
		Payload goja.ArrayBuffer
		Hash    string
	}
)

// TailSize specifies number of extra random bytes in the buffer tail.
const TailSize = 1024

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewGenerator(vu modules.VU, size int) Generator {
	if size <= 0 {
		panic("size should be positive")
	}
	return Generator{
		vu:   vu,
		size: size,
		buf:  make([]byte, size+TailSize),
	}
}

func (g *Generator) GenPayload(calcHash bool) GenPayloadResponse {
	data := g.nextSlice()

	dataHash := ""
	if calcHash {
		hashBytes := sha256.Sum256(data)
		dataHash = hex.EncodeToString(hashBytes[:])
	}

	payload := g.vu.Runtime().NewArrayBuffer(data)
	return GenPayloadResponse{Payload: payload, Hash: dataHash}
}

func (g *Generator) nextSlice() []byte {
	if g.offset >= TailSize {
		g.offset = 0
		rand.Read(g.buf) // Per docs, err is always nil here
	}

	result := g.buf[g.offset : g.offset+g.size]

	// Shift the offset for the next call. If we've used our entire tail, then erase
	// the buffer so that on the next call it is regenerated anew
	g.offset += 1
	return result
}
