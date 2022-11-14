// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package machine

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"path/filepath"
	"time"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/retrytimeout"

	"github.com/dgraph-io/badger/v3"
)

type MachineStore struct {
	db      *badger.DB
	bopts   badger.Options
	timeout time.Duration
}

type MachineStoreOption func(ms *MachineStore) error

// WithMachineStoreLogger sets the Badger DB logger interface
func WithMachineStoreLogger(l badger.Logger) MachineStoreOption {
	return func(ms *MachineStore) error {
		ms.bopts.Logger = l
		return nil
	}
}

// WithMachineStoreTimeout sets a timeout to use when connection to the store
func WithMachineStoreTimeout(timeout time.Duration) MachineStoreOption {
	return func(ms *MachineStore) error {
		ms.timeout = timeout
		return nil
	}
}

// NewMachineStoreFromPath prepares a `*MachineStore` to use to manipulate,
// save, list, view, etc. in the machine store.
func NewMachineStoreFromPath(dir string, msopts ...MachineStoreOption) (*MachineStore, error) {
	if len(dir) == 0 {
		dir = config.DefaultRuntimeDir
	}

	dir = filepath.Join(dir, "machinestore")
	ms := &MachineStore{
		bopts:   badger.DefaultOptions(dir),
		timeout: 5 * time.Second,
	}

	// TODO: Badger uses an internal `Infof` logger method entry which is too low
	// level to be considered "info" in the context of KraftKit's output.  This
	// should somehow be shifted into debug.
	//
	// For now, disable the logger entirely.  An option, `WithMachineStoreLogger`,
	// exists to enable it, though in this case it is only used if debugging is
	// enabled.  This will, however, report as "info" in the console which is
	// inconsistent with enabling "debugging".
	ms.bopts.Logger = nil

	for _, o := range msopts {
		if err := o(ms); err != nil {
			return nil, fmt.Errorf("could not apply machine store option: %v", err)
		}
	}

	return ms, nil
}

func (ms *MachineStore) connect() error {
	var db *badger.DB

	// Perform a continuous re-try to check for the dir lock on the badger
	// database which may become free during a specified timeout period
	if err := retrytimeout.RetryTimeout(ms.timeout, func() error {
		var err error
		db, err = badger.Open(ms.bopts)
		if err != nil {
			return fmt.Errorf("could not open machine store: %v", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("could not open machine store: %v", err)
	}

	ms.db = db

	return nil
}

const (
	suffixMachineConfig = "_machineconfig"
	suffixMachineState  = "_machinestate"
	suffixDriverConfig  = "_driverconfig"
)

func keyMachineConfig(mid MachineID) []byte {
	return []byte(mid.String() + suffixMachineConfig)
}

func keyMachineState(mid MachineID) []byte {
	return []byte(mid.String() + suffixMachineState)
}

func keyDriverConfig(mid MachineID) []byte {
	return []byte(mid.String() + suffixDriverConfig)
}

// SaveMachineConfig saves the machine config `mcfg` for the machine based on
// the MachineID `mid`.
func (ms *MachineStore) SaveMachineConfig(mid MachineID, mcfg MachineConfig) error {
	if err := ms.connect(); err != nil {
		return err
	}

	defer ms.close()

	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	if err := e.Encode(mcfg); err != nil {
		return fmt.Errorf("could not encode machine config for %s: %v", mid.ShortString(), err)
	}

	txn := ms.db.NewTransaction(true)
	if err := txn.SetEntry(badger.NewEntry(keyMachineConfig(mid), b.Bytes())); err != nil {
		return fmt.Errorf("could not save machine config to store for %s: %v", mid.ShortString(), err)
	}

	return txn.Commit()
}

// LookupMachineConfig uses pass-by-reference to return the machine config for
// the machine defined by the MachineID `mid` to the variable `mcfg`.
func (ms *MachineStore) LookupMachineConfig(mid MachineID, mcfg any) error {
	if err := ms.connect(); err != nil {
		return err
	}

	defer ms.close()

	if err := ms.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(keyMachineConfig(mid))
		if err != nil {
			return fmt.Errorf("could not access machine config from store for %s: %v", mid.ShortString(), err)
		}

		val, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("could not copy machine config from store for %s: %v", mid.ShortString(), err)
		}

		b := bytes.Buffer{}
		b.Write(val)

		return gob.NewDecoder(&b).Decode(mcfg)
	}); err != nil {
		return fmt.Errorf("could not read machine config from store for %s: %v", mid.ShortString(), err)
	}

	return nil
}

// SaveMachineState saves the machine `state` for the machine based on the
// MachineID `mid`.
func (ms *MachineStore) SaveMachineState(mid MachineID, state MachineState) error {
	if err := ms.connect(); err != nil {
		return err
	}

	defer ms.close()

	txn := ms.db.NewTransaction(true)
	if err := txn.SetEntry(badger.NewEntry(keyMachineState(mid), []byte(state.String()))); err != nil {
		return fmt.Errorf("could not save machine state to store for %s: %v", mid.ShortString(), err)
	}

	return txn.Commit()
}

// LookupMachineState returns the machine state in the store for the machine
// defined by the MachineID `mid`.
func (ms *MachineStore) LookupMachineState(mid MachineID) (MachineState, error) {
	state := MachineStateUnknown

	if err := ms.connect(); err != nil {
		return state, err
	}

	defer ms.close()

	if err := ms.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(keyMachineState(mid))
		if err != nil {
			return fmt.Errorf("could not access machine config from store for %s: %v", mid.ShortString(), err)
		}

		val, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("could not copy machine config from store for %s: %v", mid.ShortString(), err)
		}

		state = MachineState(string(val))
		return nil
	}); err != nil {
		return MachineStateUnknown, fmt.Errorf("could not read machine config from store for %s: %v", mid.ShortString(), err)
	}

	return state, nil
}

// SaveDriverConfig saves the driver config `dcfg` for the machine based on the
// MachineID `mid`.
func (ms *MachineStore) SaveDriverConfig(mid MachineID, dcfg any) error {
	if err := ms.connect(); err != nil {
		return err
	}

	defer ms.close()

	b := bytes.Buffer{}
	if err := gob.NewEncoder(&b).Encode(dcfg); err != nil {
		return fmt.Errorf("could not encode driver config for %s: %v", mid.ShortString(), err)
	}

	txn := ms.db.NewTransaction(true)
	if err := txn.SetEntry(badger.NewEntry(keyDriverConfig(mid), b.Bytes())); err != nil {
		return fmt.Errorf("could not save machine driver to store for %s: %v", mid.ShortString(), err)
	}

	return txn.Commit()
}

// LookupDriverConfig uses pass-by-reference to return the driver config for
// the machine defined by the MachineID `mid` to the variable `dcfg`,
func (ms *MachineStore) LookupDriverConfig(mid MachineID, dcfg any) error {
	if err := ms.connect(); err != nil {
		return err
	}

	defer ms.close()

	if err := ms.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(keyDriverConfig(mid))
		if err != nil {
			return fmt.Errorf("could not access driver config from store for %s: %v", mid.ShortString(), err)
		}

		val, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("could not copy driver config from store for %s: %v", mid.ShortString(), err)
		}

		b := bytes.Buffer{}
		b.Write(val)

		return gob.NewDecoder(&b).Decode(dcfg)
	}); err != nil {
		return fmt.Errorf("could not read driver config from store for %s: %v", mid.ShortString(), err)
	}

	return nil
}

// Purge completely removes all reference of configuration from the store based
// on the MachineID `mid`.
func (ms *MachineStore) Purge(mid MachineID) error {
	if err := ms.connect(); err != nil {
		return err
	}

	defer ms.close()

	txn := ms.db.NewTransaction(true)

	var errs []error

	if err := txn.Delete([]byte(keyDriverConfig(mid))); err != nil {
		errs = append(errs, err)
	}

	if err := txn.Delete([]byte(keyMachineConfig(mid))); err != nil {
		errs = append(errs, err)
	}

	if err := txn.Delete([]byte(keyMachineState(mid))); err != nil {
		errs = append(errs, err)
	}

	if err := txn.Commit(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		msg := "could not purge machine"
		for _, err := range errs {
			msg += ": " + err.Error()
		}

		return fmt.Errorf(msg)
	}

	return nil
}

func (ms *MachineStore) close() error {
	return ms.db.Close()
}

// ListAllMachineIDs returns a slice of all machine's saved to the store.
func (ms *MachineStore) ListAllMachineIDs() ([]MachineID, error) {
	if err := ms.connect(); err != nil {
		return nil, err
	}

	defer ms.close()

	found := make(map[MachineID]bool)

	opt := badger.DefaultIteratorOptions
	opt.PrefetchSize = 10

	// Iterate over 1000 items
	if err := ms.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(opt)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			mid := MachineID(it.Item().Key()[:MachineIDLen])
			if _, ok := found[mid]; !ok {
				found[mid] = true
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	mids := make([]MachineID, len(found))
	i := 0
	for k := range found {
		mids[i] = k
		i++
	}

	return mids, nil
}

// ListAllMachineConfigs returns a map of all machine configs saved in the store
// where the index to the map is the machine's ID.
func (ms *MachineStore) ListAllMachineConfigs() (map[MachineID]MachineConfig, error) {
	if err := ms.connect(); err != nil {
		return nil, err
	}

	defer ms.close()

	found := make(map[MachineID]MachineConfig)

	opt := badger.DefaultIteratorOptions
	opt.PrefetchSize = 10

	if err := ms.db.View(func(txn *badger.Txn) error {
		var option MachineConfig
		it := txn.NewIterator(opt)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			if string(it.Item().Key()[MachineIDLen:]) != suffixMachineConfig {
				continue
			}

			mid := MachineID(it.Item().Key()[:MachineIDLen])
			if _, ok := found[mid]; !ok {
				val, err := it.Item().ValueCopy(nil)
				if err != nil {
					return err
				}

				b := bytes.Buffer{}
				b.Write(val)

				if gob.NewDecoder(&b).Decode(&option); err != nil {
					return err
				}

				found[mid] = option
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return found, nil
}
