package xk6_frostfs

import (
	_ "github.com/TrueCloudLab/xk6-frostfs/internal/datagen"
	_ "github.com/TrueCloudLab/xk6-frostfs/internal/native"
	_ "github.com/TrueCloudLab/xk6-frostfs/internal/registry"
	_ "github.com/TrueCloudLab/xk6-frostfs/internal/s3"
	"go.k6.io/k6/js/modules"
)

const (
	version = "v0.1.0"
)

func init() {
	modules.Register("k6/x/frostfs", &FrostFS{Version: version})
}

type FrostFS struct {
	Version string
}
