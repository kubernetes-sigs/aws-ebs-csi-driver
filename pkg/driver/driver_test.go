/*
Copyright 2020 The Kubernetes Authors.

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

package driver

import (
	"reflect"
	"testing"
)

func TestWithEndpoint(t *testing.T) {
	value := "endpoint"
	options := &DriverOptions{}
	WithEndpoint(value)(options)
	if options.endpoint != value {
		t.Fatalf("expected endpoint option got set to %q but is set to %q", value, options.endpoint)
	}
}

func TestWithExtraVolumeTags(t *testing.T) {
	value := map[string]string{"foo": "bar"}
	options := &DriverOptions{}
	WithExtraVolumeTags(value)(options)
	if !reflect.DeepEqual(options.extraVolumeTags, value) {
		t.Fatalf("expected extraVolumeTags option got set to %+v but is set to %+v", value, options.extraVolumeTags)
	}
}

func TestWithMode(t *testing.T) {
	value := Mode("mode")
	options := &DriverOptions{}
	WithMode(value)(options)
	if options.mode != value {
		t.Fatalf("expected mode option got set to %q but is set to %q", value, options.mode)
	}
}

func TestWithVolumeAttachLimit(t *testing.T) {
	var value int64 = 42
	options := &DriverOptions{}
	WithVolumeAttachLimit(value)(options)
	if options.volumeAttachLimit != value {
		t.Fatalf("expected volumeAttachLimit option got set to %d but is set to %d", value, options.volumeAttachLimit)
	}
}
