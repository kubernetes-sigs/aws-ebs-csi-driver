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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testExpiration = time.Millisecond * 50
	testSleep      = time.Millisecond * 35
	testKey        = "key"
)

var (
	testValue1 = "value"
	testValue2 = "value2"
)

func TestExpiringCache(t *testing.T) {
	t.Parallel()

	cache := New[string, string](testExpiration)

	value, ok := cache.Get(testKey)
	assert.False(t, ok, "Should not be able to Get() value before Set()ing it")
	assert.Nil(t, value, "Value should be nil when Get() returns not ok")

	cache.Set(testKey, &testValue1)
	value, ok = cache.Get(testKey)
	assert.True(t, ok, "Should be able to Get() after Set()ing it")
	assert.Equal(t, &testValue1, value, "Should Get() the same value that was Set()")

	cache.Set(testKey, &testValue2)
	value, ok = cache.Get(testKey)
	assert.True(t, ok, "Should be able to Get() after Set()ing it (after overwrite)")
	assert.Equal(t, &testValue2, value, "Should Get() the same value that was Set() (after overwrite)")

	time.Sleep(testSleep)
	value, ok = cache.Get(testKey)
	assert.True(t, ok, "Should be able to Get() after sleeping less than the expiration delay")
	assert.Equal(t, &testValue2, value, "Should Get() the same value that was Set() (after sleep)")

	time.Sleep(testSleep * 2)
	value, ok = cache.Get(testKey)
	assert.False(t, ok, "Should not be able to Get() value after it expires")
	assert.Nil(t, value, "Value should be nil when Get() returns not ok (after expiration)")
}
