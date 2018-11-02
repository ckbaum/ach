package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/moov-io/ach"
)

var (
	ErrNotFound      = errors.New("Not Found")
	ErrAlreadyExists = errors.New("Already Exists")
)

// Service is a REST interface for interacting with ACH file structures
// TODO: Add ctx to function parameters to pass the client security token
type Service interface {
	// CreateFile creates a new ach file record and returns a resource ID
	CreateFile(f *ach.FileHeader) (string, error)
	// AddFile retrieves a file based on the File id
	GetFile(id string) (*ach.File, error)
	// GetFiles retrieves all files accessible from the client.
	GetFiles() []*ach.File
	// DeleteFile takes a file resource ID and deletes it from the store
	DeleteFile(id string) error
	// GetFileContents creates a valid plaintext file in memory assuming it has a FileHeader and at least one Batch record.
	GetFileContents(id string) (io.Reader, error)
	// ValidateFile
	ValidateFile(id string) error

	// CreateBatch creates a new batch within and ach file and returns its resource ID
	CreateBatch(fileID string, bh *ach.BatchHeader) (string, error)
	// GetBatch retrieves a batch based oin the file id and batch id
	GetBatch(fileID string, batchID string) (ach.Batcher, error)
	// GetBatches retrieves all batches associated with the file id.
	GetBatches(fileID string) []ach.Batcher
	// DeleteBatch takes a fileID and BatchID and removes the batch from the file
	DeleteBatch(fileID string, batchID string) error
}

// service a concrete implementation of the service.
type service struct {
	store Repository
}

// NewService creates a new concrete service
func NewService(r Repository) Service {
	return &service{
		store: r,
	}
}

// CreateFile add a file to storage
// TODO(adam): the HTTP endpoint accepts malformed bodies (and missing data)
func (s *service) CreateFile(fh *ach.FileHeader) (string, error) {
	// create a new file
	f := ach.NewFile()
	f.SetHeader(*fh)
	// set resource id's
	if fh.ID == "" {
		id := NextID()
		f.ID = id
		f.Header.ID = id
		f.Control.ID = id
	} else {
		f.ID = fh.ID
		f.Control.ID = fh.ID
	}
	if err := s.store.StoreFile(f); err != nil {
		return "", err
	}
	return f.ID, nil
}

// GetFile returns a files based on the supplied id
func (s *service) GetFile(id string) (*ach.File, error) {
	f, err := s.store.FindFile(id)
	if err != nil {
		return nil, ErrNotFound
	}
	return f, nil
}

func (s *service) GetFiles() []*ach.File {
	return s.store.FindAllFiles()
}

func (s *service) DeleteFile(id string) error {
	return s.store.DeleteFile(id)
}

func (s *service) GetFileContents(id string) (io.Reader, error) {
	f, err := s.GetFile(id)
	if err != nil {
		return nil, fmt.Errorf("problem reading file %s: %v", id, err)
	}
	if err := f.Create(); err != nil {
		return nil, fmt.Errorf("problem creating file %s: %v", id, err)
	}

	var buf bytes.Buffer
	w := ach.NewWriter(&buf)
	if err := w.Write(f); err != nil {
		return nil, fmt.Errorf("problem writing plaintext file %s: %v", id, err)
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	return &buf, nil
}

func (s *service) ValidateFile(id string) error {
	f, err := s.GetFile(id)
	if err != nil {
		return fmt.Errorf("problem reading file %s: %v", id, err)
	}
	return f.Validate()
}

func (s *service) CreateBatch(fileID string, bh *ach.BatchHeader) (string, error) {
	batch, err := ach.NewBatch(bh)
	if err != nil {
		return bh.ID, err
	}
	if bh.ID == "" {
		id := NextID()
		batch.SetID(id)
		batch.GetHeader().ID = id
		batch.GetControl().ID = id
	} else {
		batch.SetID(bh.ID)
		batch.GetControl().ID = bh.ID
	}

	if err := s.store.StoreBatch(fileID, batch); err != nil {
		return "", err
	}
	return bh.ID, nil
}

func (s *service) GetBatch(fileID string, batchID string) (ach.Batcher, error) {
	b, err := s.store.FindBatch(fileID, batchID)
	if err != nil {
		return nil, ErrNotFound
	}
	return b, nil
}

func (s *service) GetBatches(fileID string) []ach.Batcher {
	return s.store.FindAllBatches(fileID)
}

func (s *service) DeleteBatch(fileID string, batchID string) error {
	return s.store.DeleteBatch(fileID, batchID)
}

// NextID generates a new resource ID.
// Do not assume anything about the data structure.
//
// Multiple calls to NextID() have no concern about producing
// lexicographically ordered output.
func NextID() string {
	bs := make([]byte, 20)
	rand.Reader.Read(bs)

	h := sha1.New()
	h.Write(bs)
	return hex.EncodeToString(h.Sum(nil))[:16]
}