// Copyright 2019 gf Author(https://github.com/gogf/gf). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gsession

import (
	"encoding/json"
	"os"
	"time"

	"github.com/gogf/gf/crypto/gaes"

	"github.com/gogf/gf/os/gtimer"

	"github.com/gogf/gf/container/gset"
	"github.com/gogf/gf/encoding/gbinary"

	"github.com/gogf/gf/os/gtime"

	"github.com/gogf/gf/os/glog"

	"github.com/gogf/gf/os/gfile"
)

// StorageFile implements the Session Storage interface with file system.
type StorageFile struct {
	ttl           time.Duration
	path          string
	cryptoKey     []byte
	cryptoEnabled bool
	updatingIdSet *gset.StrSet
}

var (
	DefaultStorageFilePath          = gfile.Join(gfile.TempDir(), "gsessions")
	DefaultStorageFileCryptoKey     = []byte("Session storage file crypto key!")
	DefaultStorageFileCryptoEnabled = false
	DefaultStorageFileLoopInterval  = time.Minute
)

func init() {
	tmpPath := "/tmp"
	if gfile.Exists(tmpPath) && gfile.IsWritable(tmpPath) {
		DefaultStorageFilePath = gfile.Join(tmpPath, "gsessions")
	}
}

// NewStorageFile creates and returns a file storage object for session.
func NewStorageFile(ttl time.Duration, path ...string) *StorageFile {
	storagePath := DefaultStorageFilePath
	if len(path) > 0 && path[0] != "" {
		storagePath, _ = gfile.Search(path[0])
		if storagePath == "" {
			glog.Panicf("'%s' does not exist", path[0])
		}
		if !gfile.IsWritable(storagePath) {
			glog.Panicf("'%s' is not writable", path[0])
		}
	}
	if storagePath != "" {
		if err := gfile.Mkdir(storagePath); err != nil {
			glog.Panicf("mkdir '%s' failed: %v", path[0], err)
		}
	}
	s := &StorageFile{
		ttl:           ttl,
		path:          storagePath,
		cryptoKey:     DefaultStorageFileCryptoKey,
		cryptoEnabled: DefaultStorageFileCryptoEnabled,
		updatingIdSet: gset.NewStrSet(true),
	}
	gtimer.AddSingleton(DefaultStorageFileLoopInterval, func() {
		for _, id := range s.updatingIdSet.Slice() {
			s.doUpdateTTL(id)
		}
	})
	return s
}

// SetCryptoKey sets the crypto key for session storage.
// The crypto key is used when crypto feature is enabled.
func (s *StorageFile) SetCryptoKey(key []byte) {
	s.cryptoKey = key
}

// SetCryptoEnabled enables/disables the crypto feature for session storage.
func (s *StorageFile) SetCryptoEnabled(enabled bool) {
	s.cryptoEnabled = enabled
}

// sessionFilePath returns the storage file path for given session id.
func (s *StorageFile) sessionFilePath(id string) string {
	return gfile.Join(s.path, id)
}

// Get return the session data for given session id.
func (s *StorageFile) Get(id string) map[string]interface{} {
	path := s.sessionFilePath(id)
	data := gfile.GetBytes(path)
	if len(data) > 8 {
		timestampMilli := gbinary.DecodeToInt64(data[:8])
		if timestampMilli+s.ttl.Nanoseconds()/1e6 < gtime.Millisecond() {
			return nil
		}
		var err error
		content := data[8:]
		// Decrypt with AES.
		if s.cryptoEnabled {
			content, err = gaes.Decrypt(data[8:], DefaultStorageFileCryptoKey)
			if err != nil {
				return nil
			}
		}
		var m map[string]interface{}
		if err = json.Unmarshal(content, &m); err != nil {
			return nil
		}
		return m
	}
	return nil
}

// Set updates the content for session id.
// Note that the parameter <content> is the serialized bytes for session map.
func (s *StorageFile) Set(id string, data map[string]interface{}) error {
	path := s.sessionFilePath(id)
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// Encrypt with AES.
	if s.cryptoEnabled {
		content, err = gaes.Encrypt(content, DefaultStorageFileCryptoKey)
		if err != nil {
			return err
		}
	}
	file, err := gfile.OpenWithFlag(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	if _, err = file.Write(gbinary.EncodeInt64(gtime.Millisecond())); err != nil {
		return err
	}
	if _, err = file.Write(content); err != nil {
		return err
	}
	return file.Close()
}

// UpdateTTL updates the TTL for session id.
// It just adds the session id to the async handling queue.
func (s *StorageFile) UpdateTTL(id string) error {
	s.updatingIdSet.Add(id)
	return nil
}

// doUpdateTTL updates the TTL for session id.
func (s *StorageFile) doUpdateTTL(id string) error {
	path := s.sessionFilePath(id)
	file, err := gfile.OpenWithFlag(path, os.O_WRONLY)
	if err != nil {
		return err
	}
	if _, err = file.WriteAt(gbinary.EncodeInt64(gtime.Millisecond()), 0); err != nil {
		return err
	}
	return file.Close()
}
