package datagen

import (
	"strings"

	"go.k6.io/k6/js/modules"
)

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create k6/x/frostfs/registry module instances for each VU.
type RootModule struct{}

// Datagen represents an instance of the module for every VU.
type Datagen struct {
	vu modules.VU
}

// Ensure the interfaces are implemented correctly.
var (
	_ modules.Instance = &Datagen{}
	_ modules.Module   = &RootModule{}
)

func init() {
	modules.Register("k6/x/frostfs/datagen", new(RootModule))
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &Datagen{vu: vu}
	return mi
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (d *Datagen) Exports() modules.Exports {
	return modules.Exports{Default: d}
}

func (d *Datagen) Generator(size int, typ string) *Generator {
	g := NewGenerator(d.vu, size, strings.ToLower(typ))
	return &g
}
