package native

import (
	"fmt"
	"math"
	"time"

	"github.com/TrueCloudLab/frostfs-sdk-go/client"
	neofsecdsa "github.com/TrueCloudLab/frostfs-sdk-go/crypto/ecdsa"
	"github.com/TrueCloudLab/frostfs-sdk-go/session"
	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create k6/x/neofs/native module instances for each VU.
type RootModule struct{}

// Native represents an instance of the module for every VU.
type Native struct {
	vu modules.VU
}

// Ensure the interfaces are implemented correctly.
var (
	_ modules.Instance = &Native{}
	_ modules.Module   = &RootModule{}

	objPutTotal, objPutFails, objPutDuration          *metrics.Metric
	objGetTotal, objGetFails, objGetDuration          *metrics.Metric
	objDeleteTotal, objDeleteFails, objDeleteDuration *metrics.Metric
	cnrPutTotal, cnrPutFails, cnrPutDuration          *metrics.Metric
)

func init() {
	modules.Register("k6/x/neofs/native", new(RootModule))
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &Native{vu: vu}
	return mi
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (n *Native) Exports() modules.Exports {
	return modules.Exports{Default: n}
}

func (n *Native) Connect(endpoint, hexPrivateKey string) (*Client, error) {
	var (
		cli client.Client
		pk  *keys.PrivateKey
		err error
	)

	pk, err = keys.NewPrivateKey()
	if len(hexPrivateKey) != 0 {
		pk, err = keys.NewPrivateKeyFromHex(hexPrivateKey)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}

	var prmInit client.PrmInit
	prmInit.ResolveNeoFSFailures()
	prmInit.SetDefaultPrivateKey(pk.PrivateKey)
	cli.Init(prmInit)

	var prmDial client.PrmDial
	prmDial.SetServerURI(endpoint)
	prmDial.SetTimeout(5 * time.Second)

	err = cli.Dial(prmDial)
	if err != nil {
		return nil, fmt.Errorf("dial endpoint: %w", err)
	}

	// generate session token
	exp := uint64(math.MaxUint64)
	var prmSessionCreate client.PrmSessionCreate
	prmSessionCreate.SetExp(exp)
	sessionResp, err := cli.SessionCreate(n.vu.Context(), prmSessionCreate)
	if err != nil {
		return nil, fmt.Errorf("dial endpoint: %w", err)
	}

	var id uuid.UUID
	err = id.UnmarshalBinary(sessionResp.ID())
	if err != nil {
		return nil, fmt.Errorf("session token: %w", err)
	}

	var key neofsecdsa.PublicKey
	err = key.Decode(sessionResp.PublicKey())
	if err != nil {
		return nil, fmt.Errorf("invalid public session key: %w", err)
	}

	var tok session.Object

	tok.SetID(id)
	tok.SetAuthKey(&key)
	tok.SetExp(exp)

	// register metrics
	registry := metrics.NewRegistry()
	objPutTotal, _ = registry.NewMetric("neofs_obj_put_total", metrics.Counter)
	objPutFails, _ = registry.NewMetric("neofs_obj_put_fails", metrics.Counter)
	objPutDuration, _ = registry.NewMetric("neofs_obj_put_duration", metrics.Trend, metrics.Time)

	objGetTotal, _ = registry.NewMetric("neofs_obj_get_total", metrics.Counter)
	objGetFails, _ = registry.NewMetric("neofs_obj_get_fails", metrics.Counter)
	objGetDuration, _ = registry.NewMetric("neofs_obj_get_duration", metrics.Trend, metrics.Time)

	objDeleteTotal, _ = registry.NewMetric("neofs_obj_delete_total", metrics.Counter)
	objDeleteFails, _ = registry.NewMetric("neofs_obj_delete_fails", metrics.Counter)
	objDeleteDuration, _ = registry.NewMetric("neofs_obj_delete_duration", metrics.Trend, metrics.Time)

	cnrPutTotal, _ = registry.NewMetric("neofs_cnr_put_total", metrics.Counter)
	cnrPutFails, _ = registry.NewMetric("neofs_cnr_put_fails", metrics.Counter)
	cnrPutDuration, _ = registry.NewMetric("neofs_cnr_put_duration", metrics.Trend, metrics.Time)

	return &Client{
		vu:      n.vu,
		key:     pk.PrivateKey,
		tok:     tok,
		cli:     &cli,
		bufsize: defaultBufferSize,
	}, nil
}
