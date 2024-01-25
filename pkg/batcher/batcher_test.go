package batcher

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func mockExecution(inputs []string) (map[string]string, error) {
	results := make(map[string]string)
	for _, input := range inputs {
		results[input] = input
	}
	return results, nil
}

func mockExecutionWithError(inputs []string) (map[string]string, error) {
	results := make(map[string]string)
	for _, input := range inputs {
		results[input] = input
	}
	return results, fmt.Errorf("mock execution error")
}

func TestBatcher(t *testing.T) {
	type testCase struct {
		name         string
		mockFunc     func(inputs []string) (map[string]string, error)
		maxEntries   int
		maxDelay     time.Duration
		tasks        []string
		expectErrors bool
		expectResult bool
	}

	tests := []testCase{
		{
			name:         "TestBatcher: single task",
			mockFunc:     mockExecution,
			maxEntries:   10,
			maxDelay:     1 * time.Second,
			tasks:        []string{"task1"},
			expectResult: true,
			expectErrors: false,
		},
		{
			name:         "TestBatcher: multiple tasks",
			mockFunc:     mockExecution,
			maxEntries:   10,
			maxDelay:     1 * time.Second,
			tasks:        []string{"task1", "task2", "task3"},
			expectResult: true,
			expectErrors: false,
		},
		{
			name:         "TestBatcher: same task",
			mockFunc:     mockExecution,
			maxEntries:   10,
			maxDelay:     1 * time.Second,
			tasks:        []string{"task1", "task1", "task1"},
			expectResult: true,
			expectErrors: false,
		},
		{
			name:         "TestBatcher: max capacity",
			mockFunc:     mockExecution,
			maxEntries:   5,
			maxDelay:     100 * time.Second,
			tasks:        []string{"task1", "task2", "task3", "task4", "task5"},
			expectResult: true,
			expectErrors: false,
		},
		{
			name:         "TestBatcher: max delay",
			mockFunc:     mockExecution,
			maxEntries:   100,
			maxDelay:     2 * time.Second,
			tasks:        []string{"task1", "task2", "task3", "task4"},
			expectResult: true,
			expectErrors: false,
		},
		{
			name:         "TestBatcher: no execution without max delay or max entries",
			mockFunc:     mockExecution,
			maxEntries:   10,
			maxDelay:     15 * time.Second,
			tasks:        []string{"task1", "task2", "task3"},
			expectResult: false,
		},
		{
			name:         "TestBatcher: error handling",
			mockFunc:     mockExecutionWithError,
			maxEntries:   10,
			maxDelay:     1 * time.Second,
			tasks:        []string{"errorTask"},
			expectErrors: true,
		},
		{
			name:         "TestBatcher: immediate execution",
			mockFunc:     mockExecution,
			maxEntries:   10,
			maxDelay:     0,
			tasks:        []string{"task1"},
			expectResult: true,
			expectErrors: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := New(tc.maxEntries, tc.maxDelay, tc.mockFunc)
			resultChans := make([]chan BatchResult[string], len(tc.tasks))

			var wg sync.WaitGroup

			for i := 0; i < len(tc.tasks); i++ {
				wg.Add(1)
				go func(taskNum int) {
					defer wg.Done()
					task := fmt.Sprintf("task%d", taskNum)
					resultChans[taskNum] = make(chan BatchResult[string], 1)
					b.AddTask(task, resultChans[taskNum])
				}(i)
			}

			wg.Wait()

			for i := 0; i < len(tc.tasks); i++ {
				select {
				case r := <-resultChans[i]:
					task := fmt.Sprintf("task%d", i)
					if tc.expectErrors && r.Err == nil {
						t.Errorf("Expected error for task %v, but got %v", task, r.Err)
					}
					if r.Result != task && tc.expectResult {
						t.Errorf("Expected result for task %v, but got %v", task, r.Result)
					}
				case <-time.After(10 * time.Second):
					if tc.expectResult {
						t.Errorf("Timed out waiting for result of task %d", i)
					}
				}
			}
		})
	}
}

func TestBatcherConcurrentTaskAdditions(t *testing.T) {
	numTasks := 100
	var wg sync.WaitGroup

	b := New(numTasks, 1*time.Second, mockExecution)
	resultChans := make([]chan BatchResult[string], numTasks)

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(taskNum int) {
			defer wg.Done()
			task := fmt.Sprintf("task%d", taskNum)
			resultChans[taskNum] = make(chan BatchResult[string], 1)
			b.AddTask(task, resultChans[taskNum])
		}(i)
	}

	wg.Wait()

	for i := 0; i < numTasks; i++ {
		r := <-resultChans[i]
		task := fmt.Sprintf("task%d", i)
		if r.Err != nil {
			t.Errorf("Expected no error for task %v, but got %v", task, r.Err)
		}
		if r.Result != task {
			t.Errorf("Expected result %v for task %v, but got %v", task, task, r.Result)
		}
	}
}
