// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil

import (
	"context"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/golang/mock/gomock"
)

type contextMatcher struct{}

func (m contextMatcher) Matches(x any) bool {
	_, ok := x.(context.Context)
	return ok
}

func (m contextMatcher) String() string {
	return "is context"
}

func AnyContext() gomock.Matcher {
	return contextMatcher{}
}

type typeMatcher struct {
	t reflect.Type
}

func (m typeMatcher) Matches(x any) bool {
	if x == nil {
		return false
	}
	return reflect.TypeOf(x) == m.t
}

func (m typeMatcher) String() string {
	return "is type " + m.t.String()
}

func OfType(example any) gomock.Matcher {
	return typeMatcher{t: reflect.TypeOf(example)}
}

type ec2OptionsMatcher struct{}

func (m ec2OptionsMatcher) Matches(x any) bool {
	// Check if it's a single function
	if fn, ok := x.(func(*ec2.Options)); ok {
		return fn != nil
	}
	// Check if it's a slice of functions
	v := reflect.ValueOf(x)
	if v.Kind() != reflect.Slice {
		return false
	}
	sliceType := reflect.TypeOf(x)
	return sliceType.Elem() == reflect.TypeFor[func(*ec2.Options)]()
}

func (m ec2OptionsMatcher) String() string {
	return "is EC2 options function or slice"
}

func EC2Options() gomock.Matcher {
	return ec2OptionsMatcher{}
}

type ec2InputMatcher struct {
	expectedType reflect.Type
}

func (m ec2InputMatcher) Matches(x any) bool {
	if x == nil {
		return false
	}
	return reflect.TypeOf(x) == m.expectedType
}

func (m ec2InputMatcher) String() string {
	return "is " + m.expectedType.String()
}

func EC2Input(example any) gomock.Matcher {
	return ec2InputMatcher{expectedType: reflect.TypeOf(example)}
}
