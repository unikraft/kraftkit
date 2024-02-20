// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package store

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"strings"
	"time"

	zip "api.zip"
	"github.com/dgraph-io/badger/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"kraftkit.sh/internal/retrytimeout"
)

type embeddedVersioner struct{}

// UpdateObject implements storage.Versioner
func (version *embeddedVersioner) UpdateObject(obj runtime.Object, resourceVersion uint64) error {
	panic("not implemented: kraftkit.sh/machine/store.embeddedVersioner.UpdateObject")
}

// UpdateList implements storage.Versioner
func (version *embeddedVersioner) UpdateList(obj runtime.Object, resourceVersion uint64, continueValue string, remainingItemCount *int64) error {
	panic("not implemented: kraftkit.sh/machine/store.embeddedVersioner.UpdateList")
}

// PrepareObjectForStorage implements storage.Versioner
func (version *embeddedVersioner) PrepareObjectForStorage(obj runtime.Object) error {
	panic("not implemented: kraftkit.sh/machine/store.embeddedVersioner.PrepareObjectForStorage")
}

// ObjectResourceVersion implements storage.Versioner
func (version *embeddedVersioner) ObjectResourceVersion(obj runtime.Object) (uint64, error) {
	panic("not implemented: kraftkit.sh/machine/store.embeddedVersioner.ObjectResourceVersion")
}

// ParseResourceVersion implements storage.Versioner
func (version *embeddedVersioner) ParseResourceVersion(resourceVersion string) (uint64, error) {
	panic("not implemented: kraftkit.sh/machine/store.embeddedVersioner.ParseResourceVersion")
}

// embedded is KraftKit's default internal storage mechanism which is based on
type embedded[Spec, Status any] struct {
	path      string
	versioner *embeddedVersioner
	db        *badger.DB
	bopts     badger.Options
	timeout   time.Duration
}

// NewEmbeddedStore returns a api.zip.Store-compatible storage interface based
// on the embeddable key-value database Badger.
func NewEmbeddedStore[Spec, Status any](path string) (zip.Store, error) {
	var err error

	if len(path) == 0 {
		path, err = os.MkdirTemp("", "")
		if err != nil {
			return nil, err
		}
	}

	storage := embedded[Spec, Status]{
		bopts:     badger.DefaultOptions(path),
		timeout:   5 * time.Second,
		path:      path,
		versioner: &embeddedVersioner{},
	}

	// TODO: Badger uses an internal `Infof` logger method entry which is too low
	// level to be considered "info" in the context of KraftKit's output.  This
	// should somehow be shifted into debug.
	//
	// For now, disable the logger entirely.  An option, `WithMachineStoreLogger`,
	// exists to enable it, though in this case it is only used if debugging is
	// enabled.  This will, however, report as "info" in the console which is
	// inconsistent with enabling "debugging".
	storage.bopts.Logger = nil

	return &storage, nil
}

// open the embedded key-value store
func (store *embedded[_, _]) open() error {
	var db *badger.DB

	db, err := badger.Open(store.bopts)
	if err != nil && strings.Contains(err.Error(), "permission denied") {
		return fmt.Errorf("could not open machine store: %v", err)
	} else if err != nil {
		// Perform a continuous re-try to check for the dir lock on the badger
		// database which may become free during a specified timeout period
		if err := retrytimeout.RetryTimeout(store.timeout, func() error {
			var err error
			db, err = badger.Open(store.bopts)
			if err != nil {
				return fmt.Errorf("could not open machine store: %v", err)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("could not open machine store: %v", err)
		}
	}

	store.db = db

	return nil
}

// close the embedded key-value store
func (store *embedded[_, _]) close() error {
	return store.db.Close()
}

// Versioner implements storage.Interface
func (store *embedded[_, _]) Versioner() storage.Versioner {
	return store.versioner
}

// RequestWatchProgress implements storage.Interface
func (store *embedded[_, _]) RequestWatchProgress(ctx context.Context) error {
	return fmt.Errorf("not implemented: zip.store.RequestWatchProgress")
}

// Create implements storage.Interface
func (store *embedded[_, _]) Create(ctx context.Context, key string, _, out runtime.Object, ttl uint64) error {
	if err := store.open(); err != nil {
		return err
	}

	defer store.close()

	b := bytes.Buffer{}
	if err := gob.NewEncoder(&b).Encode(out); err != nil {
		return fmt.Errorf("could not encode driver config for %s: %v", key, err)
	}

	txn := store.db.NewTransaction(true)
	if err := txn.SetEntry(badger.NewEntry([]byte(key), b.Bytes())); err != nil {
		return fmt.Errorf("could not save machine driver to store for %s: %v", key, err)
	}

	return txn.Commit()
}

// Delete implements storage.Interface
func (store *embedded[_, _]) Delete(ctx context.Context, key string, out runtime.Object, preconditions *storage.Preconditions, validateDeletion storage.ValidateObjectFunc, cachedExistingObject runtime.Object) error {
	if err := store.open(); err != nil {
		return err
	}

	defer store.close()

	txn := store.db.NewTransaction(true)

	if err := txn.Delete([]byte(key)); err != nil {
		return err
	}

	// TODO(nderjung): preconditions, validateDelete, cachedExistingObject

	return txn.Commit()
}

// Watch implements storage.Interface
func (store *embedded[_, _]) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	panic("not implemented: kraftkit.sh/machine/store.embedded.Watch")
}

// Get implements storage.Interface
func (store *embedded[_, _]) Get(ctx context.Context, key string, opts storage.GetOptions, objPtr runtime.Object) error {
	if err := store.open(); err != nil {
		return err
	}

	defer store.close()

	if err := store.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return fmt.Errorf("could not access store for %s: %v", key, err)
		}

		val, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("could not copy from store for %s: %v", key, err)
		}

		b := bytes.Buffer{}
		b.Write(val)

		return gob.NewDecoder(&b).Decode(objPtr)
	}); err != nil {
		return fmt.Errorf("could not read from store for %s: %v", key, err)
	}

	return nil
}

// GetList implements storage.Interface
func (store *embedded[Spec, Status]) GetList(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	if err := store.open(); err != nil {
		return err
	}

	defer store.close()

	// Re-cast the list
	list := listObj.(*zip.ObjectList[Spec, Status])

	// Truncate the list of results as we are about to re-populate
	list.Items = make([]zip.Object[Spec, Status], 0)

	if err := store.db.View(func(txn *badger.Txn) error {
		itr := txn.NewIterator(badger.IteratorOptions{
			Prefix:       []byte(key),
			PrefetchSize: 10, // TODO(nderjung): Arbitrarily picked
		})

		defer itr.Close()

		for itr.Rewind(); itr.Valid(); itr.Next() {
			val, err := itr.Item().ValueCopy(nil)
			if err != nil {
				return err
			}

			b := bytes.Buffer{}
			b.Write(val)

			var obj zip.Object[Spec, Status]

			if err := gob.NewDecoder(&b).Decode(&obj); err != nil {
				return err
			}

			list.Items = append(list.Items, obj)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("could not list from store at %s: %v", key, err)
	}

	return nil
}

// GuaranteedUpdate implements storage.Interface
func (store *embedded[_, _]) GuaranteedUpdate(ctx context.Context, key string, destination runtime.Object, ignoreNotFound bool, preconditions *storage.Preconditions, tryUpdate storage.UpdateFunc, cachedExistingObject runtime.Object) error {
	panic("not implemented: kraftkit.sh/machine/store.embedded.GuaranteedUpdate")
}

// Count implements storage.Interface
func (store *embedded[_, _]) Count(key string) (int64, error) {
	panic("not implemented: kraftkit.sh/machine/store.embedded.Count")
}
