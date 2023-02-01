package logging

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/js/modules"
)

// InitTimestamp makes `console.log` print full timestamp instead of raw seconds.
func InitTimestamp(vu modules.VU) {
	lg, ok := vu.InitEnv().Logger.(*logrus.Logger)
	if !ok {
		return
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
}

// LogWithField adds default field to a modules.VU logger.
func LogWithField(vu modules.VU, name string, value interface{}) {
	lg, ok := vu.InitEnv().Logger.(*logrus.Logger)
	if !ok {
		return
	}

	lg.AddHook(defaultFieldHook{name: name, value: value})
}

type defaultFieldHook struct {
	name  string
	value interface{}
}

func (defaultFieldHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.InfoLevel}
}
func (h defaultFieldHook) Fire(e *logrus.Entry) error {
	e.Data[h.name] = h.value
	return nil
}
