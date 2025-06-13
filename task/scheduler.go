/*
Copyright © 2025 Alex Bedo <alex98hun@gmail.com>

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
package task

import (
	"context"
	"sync"
	"time"

	"github.com/Lunal98/go-sentinel/config"
	"github.com/Lunal98/go-sentinel/state"
	"github.com/Lunal98/go-sentinel/task/handlers"

	"github.com/rs/zerolog"
)

// Scheduler manages the execution of one-shot and periodic tasks.
type Scheduler struct {
	tasks       []config.Task
	log         *zerolog.Logger
	mu          sync.Mutex
	cancelFuncs map[string]context.CancelFunc
}

type TaskHandler = handlers.TaskHandler

// NewScheduler creates a new task scheduler.
func NewScheduler(tasks []config.Task, logger *zerolog.Logger) *Scheduler {
	return &Scheduler{
		tasks:       tasks,
		log:         logger,
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

func (s *Scheduler) RegisterHandler(name string, handl handlers.TaskHandler) {
	handlers.Register(name, handl)
}

// Run starts the scheduler, which executes tasks based on their frequency.
func (s *Scheduler) Run(ctx context.Context, sm *state.Manager) {
	s.log.Info().Msg("Task scheduler started")

	s.mu.Lock()
	s.initializeTasks(ctx, sm)
	s.mu.Unlock()

	<-ctx.Done()
	s.log.Info().Msg("Task scheduler shutting down.")
	s.mu.Lock()
	s.stopAllPeriodicTasksUnsafe()
	s.mu.Unlock()
}

func (s *Scheduler) initializeTasks(ctx context.Context, sm *state.Manager) {
	s.stopAllPeriodicTasksUnsafe()

	for _, task := range s.tasks {
		switch task.Frequency.Type {
		case "oneshot":
			go s.executeTask(ctx, task, sm)
		case "periodic":
			taskCtx, cancel := context.WithCancel(ctx)
			s.cancelFuncs[task.Name] = cancel
			go s.runPeriodicTask(taskCtx, task, sm)
		default:
			s.log.Error().Str("task", task.Name).Str("type", task.Frequency.Type).Msg("Unknown task frequency type")
		}
	}
}

// SetTasks updates the list of tasks managed by the Scheduler.
// This function is safe for concurrent use. Any previously scheduled periodic tasks will be stopped,
// and the new tasks will be scheduled.
func (s *Scheduler) SetTasks(newTasks []config.Task, sm *state.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info().Msg("Updating tasks in scheduler.")

	s.stopAllPeriodicTasksUnsafe()

	s.tasks = newTasks

	s.initializeTasks(context.Background(), sm)
}

func (s *Scheduler) stopAllPeriodicTasksUnsafe() {
	for taskName, cancel := range s.cancelFuncs {
		s.log.Info().Str("task", taskName).Msg("Stopping periodic task.")
		cancel()
		delete(s.cancelFuncs, taskName)
	}
}

func (s *Scheduler) runPeriodicTask(ctx context.Context, task config.Task, sm *state.Manager) {
	duration, err := time.ParseDuration(task.Frequency.Time)
	if err != nil {
		s.log.Error().Err(err).Str("task", task.Name).Msg("Invalid time duration for periodic task")
		return
	}

	s.log.Info().Str("task", task.Name).Str("interval", duration.String()).Msg("Scheduling periodic task")
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeTask(ctx, task, sm)
		case <-ctx.Done():
			s.log.Debug().Str("task", task.Name).Msg("Stopping periodic task due to context cancellation.")
			return
		}
	}
}

func (s *Scheduler) executeTask(ctx context.Context, task config.Task, sm *state.Manager) {
	if task.Condition != nil && task.Condition.State != "" {
		if sm.CurrentStateName() != task.Condition.State {
			s.log.Debug().Str("task", task.Name).
				Str("required_state", task.Condition.State).
				Str("active_state", sm.CurrentStateName()).
				Msg("Skipping task due to state condition not met")
			return
		}
	}

	handler, ok := handlers.Registry[task.Action.Type]
	if !ok {
		s.log.Error().Str("task", task.Name).Str("type", task.Action.Type).Msg("No handler registered for action type")
		return
	}

	s.log.Debug().Str("task", task.Name).Msg("Executing task")
	err := handler.Execute(ctx, s.log, task.Action.Params)
	if err == nil {
		s.log.Debug().Str("task", task.Name).Msg("Task executed successfully")
	} else if err == handlers.ErrProcessNotRunning {
		s.log.Warn().Msg("Task executed successfully, but process is not running, restarting state")
		sm.Saferestart()
	} else {
		s.log.Error().Err(err).Str("task", task.Name).Msg("Task execution failed")
	}
}
