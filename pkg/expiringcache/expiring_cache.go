/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package expiringcache

import (
	"sync"
	"time"
)

// ExpiringCache is a thread-safe "time expiring" cache that
// automatically removes objects that are not accessed for a
// configurable delay
//
// It is used in various places where we need to cache data for an
// unknown amount of time, to prevent memory leaks
//
// From the consumer's perspective, it behaves similarly to a map
// KeyType is the type of the object that is used as a key
// ValueType is the type of the object that is stored
type ExpiringCache[KeyType comparable, ValueType any] interface {
	// Get operates identically to retrieving from a map, returning
	// the value and/or boolean indicating if the value existed in the map
	//
	// Multiple callers can receive the same value simultaneously from Get,
	// it is the caller's responsibility to ensure they are not modified
	Get(key KeyType) (value *ValueType, ok bool)
	// Set operates identically to setting a value in a map, adding an entry
	// or overriding the existing value for a given key
	Set(key KeyType, value *ValueType)
}

type timedValue[ValueType any] struct {
	value *ValueType
	timer *time.Timer
}

type expiringCache[KeyType comparable, ValueType any] struct {
	expirationDelay time.Duration
	values          map[KeyType]timedValue[ValueType]
	mutex           sync.Mutex
}

// New returns a new ExpiringCache
// for a given KeyType, ValueType, and expiration delay
func New[KeyType comparable, ValueType any](expirationDelay time.Duration) ExpiringCache[KeyType, ValueType] {
	return &expiringCache[KeyType, ValueType]{
		expirationDelay: expirationDelay,
		values:          make(map[KeyType]timedValue[ValueType]),
	}
}

func (c *expiringCache[KeyType, ValueType]) Get(key KeyType) (*ValueType, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if v, ok := c.values[key]; ok {
		v.timer.Reset(c.expirationDelay)
		return v.value, true
	} else {
		return nil, false
	}
}

func (c *expiringCache[KeyType, ValueType]) Set(key KeyType, value *ValueType) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if v, ok := c.values[key]; ok {
		v.timer.Reset(c.expirationDelay)
		v.value = value
		c.values[key] = v
	} else {
		c.values[key] = timedValue[ValueType]{
			timer: time.AfterFunc(c.expirationDelay, func() {
				c.mutex.Lock()
				defer c.mutex.Unlock()

				delete(c.values, key)
			}),
			value: value,
		}
	}
}
