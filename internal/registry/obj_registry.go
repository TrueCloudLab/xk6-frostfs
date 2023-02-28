package registry

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"time"

	"go.etcd.io/bbolt"
)

type ObjRegistry struct {
	ctx    context.Context
	cancel context.CancelFunc
	boltDB *bbolt.DB
}

const (
	// Indicates that an object was created, but its data wasn't verified yet.
	statusCreated = "created"
)

const bucketName = "_object"

// ObjectInfo represents information about FrostFS object that has been created
// via gRPC/HTTP/S3 API.
type ObjectInfo struct {
	Id          uint64    // Identifier in bolt DB
	CreatedAt   time.Time // UTC date&time when the object was created
	CID         string    // Container ID in gRPC/HTTP
	OID         string    // Object ID in gRPC/HTTP
	S3Bucket    string    // Bucket name in S3
	S3Key       string    // Object key in S3
	Status      string    // Status of the object
	PayloadHash string    // SHA256 hash of object payload that can be used for verification
}

// NewObjRegistry creates a new instance of object registry that stores information
// about objects in the specified bolt database. As registry uses read-write
// connection to the database, there may be only one instance of object registry
// per database file at a time.
func NewObjRegistry(ctx context.Context, dbFilePath string) *ObjRegistry {
	options := bbolt.Options{Timeout: 100 * time.Millisecond}
	boltDB, err := bbolt.Open(dbFilePath, os.ModePerm, &options)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(ctx)

	objRepository := &ObjRegistry{
		ctx:    ctx,
		cancel: cancel,
		boltDB: boltDB,
	}
	return objRepository
}

func (o *ObjRegistry) AddObject(cid, oid, s3Bucket, s3Key, payloadHash string) error {
	return o.boltDB.Batch(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}

		id, err := b.NextSequence()
		if err != nil {
			return err
		}

		object := ObjectInfo{
			Id:          id,
			CreatedAt:   time.Now().UTC(),
			CID:         cid,
			OID:         oid,
			S3Bucket:    s3Bucket,
			S3Key:       s3Key,
			PayloadHash: payloadHash,
			Status:      statusCreated,
		}
		objectJson, err := json.Marshal(object)
		if err != nil {
			return err
		}

		return b.Put(encodeId(id), objectJson)
	})
}

func (o *ObjRegistry) SetObjectStatus(id uint64, newStatus string) error {
	return o.boltDB.Batch(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}

		objBytes := b.Get(encodeId(id))
		if objBytes == nil {
			return errors.New("object doesn't exist")
		}

		obj := new(ObjectInfo)
		if err := json.Unmarshal(objBytes, &obj); err != nil {
			return err
		}
		obj.Status = newStatus

		objBytes, err = json.Marshal(obj)
		if err != nil {
			return err
		}
		return b.Put(encodeId(id), objBytes)
	})
}

func (o *ObjRegistry) DeleteObject(id uint64) error {
	return o.boltDB.Batch(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}

		return b.Delete(encodeId(id))
	})
}

func (o *ObjRegistry) Close() error {
	o.cancel()
	return o.boltDB.Close()
}

func encodeId(id uint64) []byte {
	idBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(idBytes, id)
	return idBytes
}

func decodeId(idBytes []byte) uint64 {
	return binary.BigEndian.Uint64(idBytes)
}
