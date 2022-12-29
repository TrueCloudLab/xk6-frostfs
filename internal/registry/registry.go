package registry

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"

	"go.k6.io/k6/js/modules"
)

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create k6/x/frostfs/registry module instances for each VU.
type RootModule struct {
	// Stores object registry by path of database file. We should have only single instance
	// of registry per each file
	registries map[string]*ObjRegistry
	// Stores object selector by name. We may have multiple selectors per database file
	selectors map[string]*ObjSelector
	// Mutex to sync access to the maps
	mu sync.Mutex
}

// Registry represents an instance of the module for every VU.
type Registry struct {
	vu   modules.VU
	root *RootModule
}

// Ensure the interfaces are implemented correctly.
var (
	_ modules.Instance = &Registry{}
	_ modules.Module   = &RootModule{}
)

func init() {
	rootModule := &RootModule{
		registries: make(map[string]*ObjRegistry),
		selectors:  make(map[string]*ObjSelector),
	}
	modules.Register("k6/x/frostfs/registry", rootModule)
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &Registry{vu: vu, root: r}
	return mi
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (r *Registry) Exports() modules.Exports {
	return modules.Exports{Default: r}
}

// Open creates a new instance of object registry that will store information about objects
// in the specified file. If repository instance for the file was previously created, then
// Open will return the existing instance of repository, because bolt database allows only
// one write connection at a time.
func (r *Registry) Open(dbFilePath string) *ObjRegistry {
	r.root.mu.Lock()
	defer r.root.mu.Unlock()
	return r.open(dbFilePath)
}

// Implementation of Open without mutex lock, so that it can be re-used in other methods.
func (r *Registry) open(dbFilePath string) *ObjRegistry {
	registry := r.root.registries[dbFilePath]
	if registry == nil {
		registry = NewObjRegistry(r.vu.Context(), dbFilePath)
		r.root.registries[dbFilePath] = registry
	}
	return registry
}

func (r *Registry) GetSelector(dbFilePath string, name string, cacheSize int, filter map[string]string) *ObjSelector {
	objFilter, err := parseFilter(filter)
	if err != nil {
		panic(err)
	}

	r.root.mu.Lock()
	defer r.root.mu.Unlock()

	selector := r.root.selectors[name]
	if selector == nil {
		registry := r.open(dbFilePath)
		selector = NewObjSelector(registry, cacheSize, objFilter)
		r.root.selectors[name] = selector
	} else if !reflect.DeepEqual(selector.filter, objFilter) {
		panic(fmt.Sprintf("selector %s already has been created with a different filter", name))
	}
	return selector
}

func parseFilter(filter map[string]string) (*ObjFilter, error) {
	objFilter := ObjFilter{}
	objFilter.Status = filter["status"]

	if ageStr := filter["age"]; ageStr != "" {
		age, err := strconv.ParseInt(ageStr, 10, 64)
		if err != nil {
			return nil, err
		}
		objFilter.Age = int(age)
	}

	return &objFilter, nil
}
