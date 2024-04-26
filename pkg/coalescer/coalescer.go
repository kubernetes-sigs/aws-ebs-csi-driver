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

// Package coalescer combines multiple requests made over a period of time into a single request
package coalescer

import (
	"time"

	"k8s.io/klog/v2"
)

// Coalescer is an interface to combine multiple requests made over a period of time into a single request
//
// When a request is received that matches an existing in-flight request, the coalescer will attempt to
// merge that request into the existing request pool using the provided mergeFunction
//
// When the delay on the request expires (determined by the time the first request comes in), the merged
// input is passed to the execution function, and the result to all waiting callers (those that were
// not rejected during the merge step)
type Coalescer[InputType comparable, ResultType any] interface {
	// Coalesce is a function to coalesce a given input
	// key = only requests with this same key will be coalesced (such as volume ID)
	// input = input to merge with other inputs
	// It is NOT guaranteed all callers receive the same result (for example, if
	// an input fails to merge, only that caller will receive an error)
	Coalesce(key string, input InputType) (ResultType, error)
}

// New is a function to creates a new coalescer and immediately begin processing requests
// delay = the time to wait for other requests to coalesce before executing
// mergeFunction = a function to merge a new input with the existing inputs
// (should return an error if the new input cannot be combined with the existing inputs,
// otherwise return the new merged input)
// executeFunction = the function to call when the delay expires
func New[InputType comparable, ResultType any](delay time.Duration,
	mergeFunction func(input InputType, existing InputType) (InputType, error),
	executeFunction func(key string, input InputType) (ResultType, error),
) Coalescer[InputType, ResultType] {
	c := coalescer[InputType, ResultType]{
		delay:           delay,
		mergeFunction:   mergeFunction,
		executeFunction: executeFunction,
		inputChannel:    make(chan newInput[InputType, ResultType]),
		timerChannel:    make(chan string),
		pendingInputs:   make(map[string]pendingInput[InputType, ResultType]),
	}

	go c.coalescerThread()
	return &c
}

// Type to store a result or error in channels
type result[ResultType any] struct {
	result ResultType
	err    error
}

// Type to send inputs from Coalesce() to coalescerThread() via channel
// Includes a return channel for the result
type newInput[InputType comparable, ResultType any] struct {
	key           string
	input         InputType
	resultChannel chan result[ResultType]
}

// Type to store pending inputs in the input map
type pendingInput[InputType comparable, ResultType any] struct {
	input          InputType
	resultChannels []chan result[ResultType]
}

type coalescer[InputType comparable, ResultType any] struct {
	delay           time.Duration
	mergeFunction   func(input InputType, existing InputType) (InputType, error)
	executeFunction func(key string, input InputType) (ResultType, error)

	inputChannel chan newInput[InputType, ResultType]
	timerChannel chan string

	pendingInputs map[string]pendingInput[InputType, ResultType]
}

func (c *coalescer[InputType, ResultType]) Coalesce(key string, input InputType) (ResultType, error) {
	resultChannel := make(chan result[ResultType])

	c.inputChannel <- newInput[InputType, ResultType]{
		key:           key,
		input:         input,
		resultChannel: resultChannel,
	}
	result := <-resultChannel

	if result.err != nil {
		return *new(ResultType), result.err
	} else {
		return result.result, nil
	}
}

func (c *coalescer[InputType, ResultType]) coalescerThread() {
	for {
		select {
		case i := <-c.inputChannel:
			klog.V(7).InfoS("coalescerThread: Input received", "key", i.key, "input", i.input)
			if pending, ok := c.pendingInputs[i.key]; ok {
				klog.V(7).InfoS("coalescerThread: Input matched existing input, attempting to merge", "key", i.key)
				newInput, err := c.mergeFunction(i.input, pending.input)

				if err == nil {
					klog.V(7).InfoS("coalescerThread: Merged input into existing inputs", "key", i.key)
					pending.input = newInput
					pending.resultChannels = append(pending.resultChannels, i.resultChannel)
					c.pendingInputs[i.key] = pending
				} else {
					klog.V(7).InfoS("coalescerThread: Failed to merge inputs into existing inputs", "key", i.key)
					i.resultChannel <- result[ResultType]{
						err: err,
					}
				}
			} else {
				klog.V(7).InfoS("coalescerThread: New input, setting up fresh coalesce operation", "key", i.key)
				c.pendingInputs[i.key] = pendingInput[InputType, ResultType]{
					input: i.input,
					resultChannels: []chan result[ResultType]{
						i.resultChannel,
					},
				}
				time.AfterFunc(c.delay, func() {
					c.timerChannel <- i.key
				})
			}

		case k := <-c.timerChannel:
			klog.V(7).InfoS("coalescerThread: Coalescing delay reached, spawning execution thread", "key", k)
			pending := c.pendingInputs[k]
			delete(c.pendingInputs, k)

			go func() {
				r, err := c.executeFunction(k, pending.input)
				klog.V(7).InfoS("coalescerThread: Finished executing", "key", k, "result", r, "error", err)
				result := result[ResultType]{
					result: r,
					err:    err,
				}
				for _, c := range pending.resultChannels {
					c <- result
				}
			}()
		}
	}
}
