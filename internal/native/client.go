package native

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/TrueCloudLab/frostfs-sdk-go/checksum"
	"github.com/TrueCloudLab/frostfs-sdk-go/client"
	"github.com/TrueCloudLab/frostfs-sdk-go/container"
	"github.com/TrueCloudLab/frostfs-sdk-go/container/acl"
	cid "github.com/TrueCloudLab/frostfs-sdk-go/container/id"
	"github.com/TrueCloudLab/frostfs-sdk-go/netmap"
	"github.com/TrueCloudLab/frostfs-sdk-go/object"
	oid "github.com/TrueCloudLab/frostfs-sdk-go/object/id"
	"github.com/TrueCloudLab/frostfs-sdk-go/session"
	"github.com/TrueCloudLab/frostfs-sdk-go/user"
	"github.com/TrueCloudLab/frostfs-sdk-go/version"
	"github.com/TrueCloudLab/tzhash/tz"
	"github.com/TrueCloudLab/xk6-frostfs/internal/stats"
	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

type (
	Client struct {
		vu      modules.VU
		key     ecdsa.PrivateKey
		tok     session.Object
		cli     *client.Client
		bufsize int
	}

	PutResponse struct {
		Success  bool
		ObjectID string
		Error    string
	}

	DeleteResponse struct {
		Success bool
		Error   string
	}

	GetResponse struct {
		Success bool
		Error   string
	}

	VerifyHashResponse struct {
		Success bool
		Error   string
	}

	PutContainerResponse struct {
		Success     bool
		ContainerID string
		Error       string
	}

	PreparedObject struct {
		vu      modules.VU
		key     ecdsa.PrivateKey
		cli     *client.Client
		bufsize int

		hdr     object.Object
		payload []byte
	}
)

const defaultBufferSize = 64 * 1024

func (c *Client) SetBufferSize(size int) {
	if size < 0 {
		panic("buffer size must be positive")
	}
	if size == 0 {
		c.bufsize = defaultBufferSize
	} else {
		c.bufsize = size
	}
}

func (c *Client) Put(containerID string, headers map[string]string, payload goja.ArrayBuffer) PutResponse {
	cliContainerID := parseContainerID(containerID)

	tok := c.tok
	tok.ForVerb(session.VerbObjectPut)
	tok.BindContainer(cliContainerID)
	err := tok.Sign(c.key)
	if err != nil {
		panic(err)
	}

	var owner user.ID
	user.IDFromKey(&owner, c.key.PublicKey)

	attrs := make([]object.Attribute, len(headers))
	ind := 0
	for k, v := range headers {
		attrs[ind].SetKey(k)
		attrs[ind].SetValue(v)
		ind++
	}

	var o object.Object
	o.SetContainerID(cliContainerID)
	o.SetOwnerID(&owner)
	o.SetAttributes(attrs...)

	resp, err := put(c.vu, c.bufsize, c.cli, &tok, &o, payload.Bytes())
	if err != nil {
		return PutResponse{Success: false, Error: err.Error()}
	}

	return PutResponse{Success: true, ObjectID: resp.StoredObjectID().String()}
}

func (c *Client) Delete(containerID string, objectID string) DeleteResponse {
	cliContainerID := parseContainerID(containerID)
	cliObjectID := parseObjectID(objectID)

	tok := c.tok
	tok.ForVerb(session.VerbObjectDelete)
	tok.BindContainer(cliContainerID)
	tok.LimitByObjects(cliObjectID)
	err := tok.Sign(c.key)
	if err != nil {
		panic(err)
	}

	start := time.Now()

	var prm client.PrmObjectDelete
	prm.ByID(cliObjectID)
	prm.FromContainer(cliContainerID)
	prm.WithinSession(tok)

	_, err = c.cli.ObjectDelete(c.vu.Context(), prm)
	if err != nil {
		stats.Report(c.vu, objDeleteFails, 1)
		return DeleteResponse{Success: false, Error: err.Error()}
	}

	stats.Report(c.vu, objDeleteTotal, 1)
	stats.Report(c.vu, objDeleteDuration, metrics.D(time.Since(start)))
	return DeleteResponse{Success: true}
}

func (c *Client) Get(containerID, objectID string) GetResponse {
	cliContainerID := parseContainerID(containerID)
	cliObjectID := parseObjectID(objectID)

	tok := c.tok
	tok.ForVerb(session.VerbObjectGet)
	tok.BindContainer(cliContainerID)
	tok.LimitByObjects(cliObjectID)
	err := tok.Sign(c.key)
	if err != nil {
		panic(err)
	}

	start := time.Now()

	var prm client.PrmObjectGet
	prm.ByID(cliObjectID)
	prm.FromContainer(cliContainerID)
	prm.WithinSession(tok)

	var objSize = 0
	err = get(c.cli, prm, c.vu.Context(), c.bufsize, func(data []byte) {
		objSize += len(data)
	})
	if err != nil {
		stats.Report(c.vu, objGetFails, 1)
		return GetResponse{Success: false, Error: err.Error()}
	}

	stats.Report(c.vu, objGetTotal, 1)
	stats.Report(c.vu, objGetDuration, metrics.D(time.Since(start)))
	stats.ReportDataReceived(c.vu, float64(objSize))
	return GetResponse{Success: true}
}

func get(
	cli *client.Client,
	prm client.PrmObjectGet,
	ctx context.Context,
	bufSize int,
	onDataChunk func(chunk []byte),
) error {
	var buf = make([]byte, bufSize)

	objectReader, err := cli.ObjectGetInit(ctx, prm)
	if err != nil {
		return err
	}

	var o object.Object
	if !objectReader.ReadHeader(&o) {
		if _, err = objectReader.Close(); err != nil {
			return err
		}
		return errors.New("can't read object header")
	}

	n, _ := objectReader.Read(buf)
	for n > 0 {
		onDataChunk(buf[:n])
		n, _ = objectReader.Read(buf)
	}

	_, err = objectReader.Close()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) VerifyHash(containerID, objectID, expectedHash string) VerifyHashResponse {
	cliContainerID := parseContainerID(containerID)
	cliObjectID := parseObjectID(objectID)

	tok := c.tok
	tok.ForVerb(session.VerbObjectGet)
	tok.BindContainer(cliContainerID)
	tok.LimitByObjects(cliObjectID)
	err := tok.Sign(c.key)
	if err != nil {
		panic(err)
	}

	var prm client.PrmObjectGet
	prm.ByID(cliObjectID)
	prm.FromContainer(cliContainerID)
	prm.WithinSession(tok)

	hasher := sha256.New()
	err = get(c.cli, prm, c.vu.Context(), c.bufsize, func(data []byte) {
		hasher.Write(data)
	})
	if err != nil {
		return VerifyHashResponse{Success: false, Error: err.Error()}
	}
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return VerifyHashResponse{Success: true, Error: "hash mismatch"}
	}

	return VerifyHashResponse{Success: true}
}

func (c *Client) putCnrErrorResponse(err error) PutContainerResponse {
	stats.Report(c.vu, cnrPutFails, 1)
	return PutContainerResponse{Success: false, Error: err.Error()}
}

func (c *Client) PutContainer(params map[string]string) PutContainerResponse {
	stats.Report(c.vu, cnrPutTotal, 1)

	var cnr container.Container
	cnr.Init()

	var usr user.ID
	user.IDFromKey(&usr, c.key.PublicKey)

	container.SetCreationTime(&cnr, time.Now())
	cnr.SetOwner(usr)

	if basicACLStr, ok := params["acl"]; ok {
		var basicACL acl.Basic
		err := basicACL.DecodeString(basicACLStr)
		if err != nil {
			return c.putCnrErrorResponse(err)
		}

		cnr.SetBasicACL(basicACL)
	}

	placementPolicyStr, ok := params["placement_policy"]
	if ok {
		var placementPolicy netmap.PlacementPolicy
		err := placementPolicy.DecodeString(placementPolicyStr)
		if err != nil {
			return c.putCnrErrorResponse(err)
		}

		cnr.SetPlacementPolicy(placementPolicy)
	}

	containerName, hasName := params["name"]
	if hasName {
		container.SetName(&cnr, containerName)
	}

	var err error
	var nameScopeGlobal bool
	if nameScopeGlobalStr, ok := params["name_scope_global"]; ok {
		if nameScopeGlobal, err = strconv.ParseBool(nameScopeGlobalStr); err != nil {
			return c.putCnrErrorResponse(fmt.Errorf("invalid name_scope_global param: %w", err))
		}
	}

	if nameScopeGlobal {
		if !hasName {
			return c.putCnrErrorResponse(errors.New("you must provide container name if name_scope_global param is set"))
		}

		var domain container.Domain
		domain.SetName(containerName)

		container.WriteDomain(&cnr, domain)
	}

	start := time.Now()
	var prm client.PrmContainerPut
	prm.SetContainer(cnr)

	res, err := c.cli.ContainerPut(c.vu.Context(), prm)
	if err != nil {
		return c.putCnrErrorResponse(err)
	}

	var wp waitParams
	wp.setDefaults()

	if err = c.waitForContainerPresence(c.vu.Context(), res.ID(), &wp); err != nil {
		return c.putCnrErrorResponse(err)
	}

	stats.Report(c.vu, cnrPutDuration, metrics.D(time.Since(start)))
	return PutContainerResponse{Success: true, ContainerID: res.ID().EncodeToString()}
}

func (c *Client) Onsite(containerID string, payload goja.ArrayBuffer) PreparedObject {
	maxObjectSize, epoch, hhDisabled, err := parseNetworkInfo(c.vu.Context(), c.cli)
	if err != nil {
		panic(err)
	}
	data := payload.Bytes()
	ln := len(data)
	if ln > int(maxObjectSize) {
		// not sure if load test needs object transformation
		// with parent-child relation; if needs, then replace
		// this code with the usage of object transformer from
		// frostfs-loader or distribution.
		msg := fmt.Sprintf("payload size %d is bigger than network limit %d", ln, maxObjectSize)
		panic(msg)
	}

	cliContainerID := parseContainerID(containerID)

	var owner user.ID
	user.IDFromKey(&owner, c.key.PublicKey)

	apiVersion := version.Current()

	obj := object.New()
	obj.SetVersion(&apiVersion)
	obj.SetType(object.TypeRegular)
	obj.SetContainerID(cliContainerID)
	obj.SetOwnerID(&owner)
	obj.SetPayloadSize(uint64(ln))
	obj.SetCreationEpoch(epoch)

	var sha, hh checksum.Checksum
	sha.SetSHA256(sha256.Sum256(data))
	obj.SetPayloadChecksum(sha)
	if !hhDisabled {
		hh.SetTillichZemor(tz.Sum(data))
		obj.SetPayloadHomomorphicHash(hh)
	}

	return PreparedObject{
		vu:      c.vu,
		key:     c.key,
		cli:     c.cli,
		bufsize: c.bufsize,

		hdr:     *obj,
		payload: data,
	}
}

func (p PreparedObject) Put(headers map[string]string) PutResponse {
	obj := p.hdr

	attrs := make([]object.Attribute, len(headers))
	ind := 0
	for k, v := range headers {
		attrs[ind].SetKey(k)
		attrs[ind].SetValue(v)
		ind++
	}
	obj.SetAttributes(attrs...)

	id, err := object.CalculateID(&obj)
	if err != nil {
		return PutResponse{Success: false, Error: err.Error()}
	}
	obj.SetID(id)

	if err = object.CalculateAndSetSignature(p.key, &obj); err != nil {
		return PutResponse{Success: false, Error: err.Error()}
	}

	_, err = put(p.vu, p.bufsize, p.cli, nil, &obj, p.payload)
	if err != nil {
		return PutResponse{Success: false, Error: err.Error()}
	}

	return PutResponse{Success: true, ObjectID: id.String()}
}

func put(vu modules.VU, bufSize int, cli *client.Client, tok *session.Object,
	hdr *object.Object, payload []byte) (*client.ResObjectPut, error) {
	buf := make([]byte, bufSize)
	rdr := bytes.NewReader(payload)
	sz := rdr.Size()

	// starting upload
	start := time.Now()

	var prm client.PrmObjectPutInit
	if tok != nil {
		prm.WithinSession(*tok)
	}

	objectWriter, err := cli.ObjectPutInit(vu.Context(), prm)
	if err != nil {
		stats.Report(vu, objPutFails, 1)
		return nil, err
	}

	if !objectWriter.WriteHeader(*hdr) {
		stats.Report(vu, objPutFails, 1)
		_, err = objectWriter.Close()
		return nil, err
	}

	n, _ := rdr.Read(buf)
	for n > 0 {
		if !objectWriter.WritePayloadChunk(buf[:n]) {
			break
		}
		n, _ = rdr.Read(buf)
	}

	resp, err := objectWriter.Close()
	if err != nil {
		stats.Report(vu, objPutFails, 1)
		return nil, err
	}

	stats.Report(vu, objPutTotal, 1)
	stats.ReportDataSent(vu, float64(sz))
	stats.Report(vu, objPutDuration, metrics.D(time.Since(start)))

	return resp, nil
}

func parseNetworkInfo(ctx context.Context, cli *client.Client) (maxObjSize, epoch uint64, hhDisabled bool, err error) {
	ni, err := cli.NetworkInfo(ctx, client.PrmNetworkInfo{})
	if err != nil {
		return 0, 0, false, err
	}

	ninfo := ni.Info()

	return ninfo.MaxObjectSize(), ninfo.CurrentEpoch(), ninfo.HomomorphicHashingDisabled(), err
}

type waitParams struct {
	timeout      time.Duration
	pollInterval time.Duration
}

func (x *waitParams) setDefaults() {
	x.timeout = 120 * time.Second
	x.pollInterval = 5 * time.Second
}

func (c *Client) waitForContainerPresence(ctx context.Context, cnrID cid.ID, wp *waitParams) error {
	return waitFor(ctx, wp, func(ctx context.Context) bool {
		var prm client.PrmContainerGet
		prm.SetContainer(cnrID)

		_, err := c.cli.ContainerGet(ctx, prm)
		return err == nil
	})
}

func waitFor(ctx context.Context, params *waitParams, condition func(context.Context) bool) error {
	wctx, cancel := context.WithTimeout(ctx, params.timeout)
	defer cancel()
	ticker := time.NewTimer(params.pollInterval)
	defer ticker.Stop()
	wdone := wctx.Done()
	done := ctx.Done()
	for {
		select {
		case <-done:
			return ctx.Err()
		case <-wdone:
			return wctx.Err()
		case <-ticker.C:
			if condition(ctx) {
				return nil
			}
			ticker.Reset(params.pollInterval)
		}
	}
}

func parseContainerID(strContainerID string) cid.ID {
	var containerID cid.ID
	err := containerID.DecodeString(strContainerID)
	if err != nil {
		panic(err)
	}
	return containerID
}

func parseObjectID(strObjectID string) oid.ID {
	var cliObjectID oid.ID
	err := cliObjectID.DecodeString(strObjectID)
	if err != nil {
		panic(err)
	}
	return cliObjectID
}
