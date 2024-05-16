// Copyright 2024 The Kubernetes Authors.
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

// Package batcher facilitates task aggregation and execution.
//
// Basic Usage:
// Instantiate a Batcher, set up its constraints, and then start adding tasks. As tasks accumulate,
// they are batched together for execution, either when a maximum task count is reached or a specified
// duration elapses. Results of the executed tasks are communicated asynchronously via channels.
//
// Example:
// Create a Batcher with a maximum of 10 tasks or a 5-second wait:
//
//	`b := batcher.New(10, 5*time.Second, execFunc)`
//
// Add a task and receive its result:
//
//	resultChan := make(chan batcher.BatchResult)
//	b.AddTask(myTask, resultChan)
//	result := <-resultChan
//
// Key Components:
//   - `Batcher`: The main component that manages task queueing, aggregation, and execution.
//   - `BatchResult`: A structure encapsulating the response for a task.
//   - `taskEntry`: Internal representation of a task and its associated result channel.
//
// Task Duplication:
// Batcher identifies tasks by content. For multiple identical tasks, each has a unique result channel.
// This distinction ensures that identical tasks return their results to the appropriate callers.
package batcher

import (
	"time"

	"k8s.io/klog/v2"
)

// Batcher manages the batching and execution of tasks. It collects tasks up to a specified limit (maxEntries) or
// waits for a defined duration (maxDelay) before triggering a batch execution. The actual task execution
// logic is provided by the execFunc, which processes tasks and returns their corresponding results. Tasks are
// queued via the taskChan and stored in pendingTasks until batch execution.
type Batcher[InputType comparable, ResultType interface{}] struct {
	// execFunc is the function responsible for executing a batch of tasks.
	// It returns a map associating each task with its result.
	execFunc func(inputs []InputType) (map[InputType]ResultType, error)

	// pendingTasks holds the tasks that are waiting to be executed in a batch.
	// Each task is associated with one or more result channels to account for duplicates.
	pendingTasks map[InputType][]chan BatchResult[ResultType]

	// taskChan is the channel through which new tasks are added to the Batcher.
	taskChan chan taskEntry[InputType, ResultType]

	// maxEntries is the maximum number of tasks that can be batched together for execution.
	maxEntries int

	// maxDelay is the maximum duration the Batcher waits before executing a batch operation,
	// regardless of how many tasks are in the batch.
	maxDelay time.Duration
}

// BatchResult encapsulates the response of a batched task.
// A task will either have a result or an error, but not both.
type BatchResult[ResultType interface{}] struct {
	Result ResultType
	Err    error
}

// taskEntry represents a single task waiting to be batched and its associated result channel.
// The result channel is used to communicate the task's result back to the caller.
type taskEntry[InputType comparable, ResultType interface{}] struct {
	task       InputType
	resultChan chan BatchResult[ResultType]
}

// New creates and returns a Batcher configured with the specified maxEntries and maxDelay parameters.
// Upon instantiation, it immediately launches the internal task manager as a goroutine to oversee batch operations.
// The provided execFunc is used to execute batch requests.
func New[InputType comparable, ResultType interface{}](entries int, delay time.Duration, fn func(inputs []InputType) (map[InputType]ResultType, error)) *Batcher[InputType, ResultType] {
	klog.V(7).InfoS("New: initializing Batcher", "maxEntries", entries, "maxDelay", delay)

	b := &Batcher[InputType, ResultType]{
		execFunc:     fn,
		pendingTasks: make(map[InputType][]chan BatchResult[ResultType]),
		taskChan:     make(chan taskEntry[InputType, ResultType], entries),
		maxEntries:   entries,
		maxDelay:     delay,
	}

	go b.taskManager()
	return b
}

// AddTask adds a new task to the Batcher's queue.
func (b *Batcher[InputType, ResultType]) AddTask(t InputType, resultChan chan BatchResult[ResultType]) {
	klog.V(7).InfoS("AddTask: queueing task", "task", t)
	b.taskChan <- taskEntry[InputType, ResultType]{task: t, resultChan: resultChan}
}

// taskManager runs as a goroutine, continuously managing the Batcher's internal state.
// It batches tasks and triggers their execution based on set constraints (maxEntries and maxDelay).
func (b *Batcher[InputType, ResultType]) taskManager() {
	klog.V(7).InfoS("taskManager: started taskManager")
	var timerCh <-chan time.Time

	exec := func() {
		timerCh = nil
		go b.execute(b.pendingTasks)
		b.pendingTasks = make(map[InputType][]chan BatchResult[ResultType])
	}

	for {
		select {
		case <-timerCh:
			klog.V(7).InfoS("taskManager: maxDelay execution")
			exec()

		case t := <-b.taskChan:
			if _, exists := b.pendingTasks[t.task]; exists {
				klog.InfoS("taskManager: duplicate task detected", "task", t.task)
			} else {
				b.pendingTasks[t.task] = make([]chan BatchResult[ResultType], 0)
			}
			b.pendingTasks[t.task] = append(b.pendingTasks[t.task], t.resultChan)

			if len(b.pendingTasks) == 1 {
				klog.V(7).InfoS("taskManager: starting maxDelay timer")
				timerCh = time.After(b.maxDelay)
			}

			if len(b.pendingTasks) == b.maxEntries {
				klog.V(7).InfoS("taskManager: maxEntries reached")
				exec()
			}
		}
	}
}

// execute is called by taskManager to execute a batch of tasks.
// It calls the Batcher's internal execFunc and then sends the results of each task to its corresponding result channels.
func (b *Batcher[InputType, ResultType]) execute(pendingTasks map[InputType][]chan BatchResult[ResultType]) {
	batch := make([]InputType, 0, len(pendingTasks))
	for task := range pendingTasks {
		batch = append(batch, task)
	}

	klog.V(7).InfoS("execute: calling execFunc", "batchSize", len(batch))
	resultsMap, err := b.execFunc(batch)
	if err != nil {
		klog.ErrorS(err, "execute: error executing batch")
	}

	klog.V(7).InfoS("execute: sending batch results", "batch", batch)
	for _, task := range batch {
		r := resultsMap[task]
		for _, ch := range pendingTasks[task] {
			select {
			case ch <- BatchResult[ResultType]{Result: r, Err: err}:
			default:
				klog.V(7).InfoS("execute: ignoring channel with no receiver")
			}
		}
	}
	klog.V(7).InfoS("execute: finished execution", "batchSize", len(batch))
}
