package logging

import (
	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/js/modules"
)

// Logging represents an instance of the module for every VU.
type Logging struct {
	vu modules.VU
}

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create k6/x/frostfs/logging module instances for each VU.
type RootModule struct{}

// Ensure the interfaces are implemented correctly.
var (
	_ modules.Instance = &Logging{}
	_ modules.Module   = &RootModule{}
)

func init() {
	modules.Register("k6/x/frostfs/logging", new(RootModule))
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (n *Logging) Exports() modules.Exports {
	return modules.Exports{Default: n}
}

type logger struct {
	logrus.FieldLogger
}

func (n *Logging) New() logger {
	return logger{FieldLogger: n.vu.InitEnv().Logger}
}

func (l logger) WithFields(fields *goja.Object) logger {
	lg := l.FieldLogger
	for _, k := range fields.Keys() {
		lg = lg.WithField(k, fields.Get(k))
	}
	return logger{FieldLogger: lg}
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	lg, ok := vu.InitEnv().Logger.(*logrus.Logger)
	if !ok {
		return &Logging{vu: vu}
	}

	format := lg.Formatter
	switch f := format.(type) {
	case *logrus.TextFormatter:
		f.ForceColors = true
		f.FullTimestamp = true
		f.TimestampFormat = "15:04:05"
	case *logrus.JSONFormatter:
		f.TimestampFormat = "15:04:05"
	}

	return &Logging{vu: vu}
}
