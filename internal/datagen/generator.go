package datagen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/dop251/goja"
	"github.com/go-loremipsum/loremipsum"
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
		typ    string
		offset int
	}

	GenPayloadResponse struct {
		Payload goja.ArrayBuffer
		Hash    string
	}
)

// TailSize specifies number of extra random bytes in the buffer tail.
const TailSize = 1024

var payloadTypes = []string{
	"text",
	"random",
	"",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewGenerator(vu modules.VU, size int, typ string) Generator {
	if size <= 0 {
		panic("size should be positive")
	}
	var found bool
	for i := range payloadTypes {
		if payloadTypes[i] == typ {
			found = true
		}
	}
	if !found {
		vu.InitEnv().Logger.Info("Unknown payload type '%s', random will be used.", typ)
	}
	return Generator{vu: vu, size: size, buf: nil, typ: typ, offset: 0}
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
	if g.buf == nil {
		switch g.typ {
		case "text":
			li := loremipsum.New()
			b := bytes.NewBuffer(nil)
			for b.Len() < g.size+TailSize {
				b.WriteString(li.Paragraph())
				b.WriteRune('\n')
			}
			g.buf = b.Bytes()
		default:
			// Allocate buffer with extra tail for sliding and populate it with random bytes
			g.buf = make([]byte, g.size+TailSize)
			rand.Read(g.buf) // Per docs, err is always nil here
		}
	}

	result := g.buf[g.offset : g.offset+g.size]

	// Shift the offset for the next call. If we've used our entire tail, then erase
	// the buffer so that on the next call it is regenerated anew
	g.offset += 1
	if g.offset+g.size > len(g.buf) {
		g.buf = nil
		g.offset = 0
	}

	return result
}
