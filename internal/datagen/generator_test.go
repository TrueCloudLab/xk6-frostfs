package datagen

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
)

func TestGenerator(t *testing.T) {
	vu := &modulestest.VU{
		RuntimeField: goja.New(),
	}

	t.Run("fails on negative size", func(t *testing.T) {
		require.Panics(t, func() {
			_ = NewGenerator(vu, -1, "")
		})
	})

	t.Run("fails on zero size", func(t *testing.T) {
		require.Panics(t, func() {
			_ = NewGenerator(vu, 0, "")
		})
	})

	t.Run("creates slice of specified size", func(t *testing.T) {
		size := 10
		g := NewGenerator(vu, size, "")
		slice := g.nextSlice()
		require.Len(t, slice, size)
	})

	t.Run("creates a different slice on each call", func(t *testing.T) {
		g := NewGenerator(vu, 1000, "")
		slice1 := g.nextSlice()
		slice2 := g.nextSlice()
		// Each slice should be unique (assuming that 1000 random bytes will never coincide
		// to be identical)
		assert.NotEqual(t, slice1, slice2)
	})

	t.Run("keeps generating slices after consuming entire tail", func(t *testing.T) {
		g := NewGenerator(vu, 1000, "")
		initialSlice := g.nextSlice()
		for i := 0; i < TailSize; i++ {
			g.nextSlice()
		}
		// After we looped around our tail and returned to the beginning we should have a
		// unique slice - not the same as in the beginning
		sliceAfterTail := g.nextSlice()
		assert.NotEqual(t, initialSlice, sliceAfterTail)
	})
}
