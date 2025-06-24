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
package sentinel

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/Lunal98/go-sentinel/config"
	"github.com/Lunal98/go-sentinel/state"
	"github.com/Lunal98/go-sentinel/task"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	globalLogger "github.com/rs/zerolog/log"

	"github.com/spf13/viper"
)

var (
	configMutex       sync.RWMutex
	currentConfig     config.Config
	v                 *viper.Viper
	stateManager      *state.Manager
	taskScheduler     *task.Scheduler
	log               *zerolog.Logger
	userTaskHandlers  map[string]TaskHandler
	userStateHandlers map[string]StateHandler
)

func init() {
	userStateHandlers = make(map[string]StateHandler)
	userTaskHandlers = make(map[string]TaskHandler)
	templog := globalLogger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log = &templog
	log.Level(zerolog.InfoLevel)
	v = viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/go-sentinel/")
	v.AddConfigPath("$HOME/.go-sentinel/")
	v.AddConfigPath(".")
	v.SetEnvPrefix("go_sentinel")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if configPath := os.Getenv("GO_SENTINEL_CONFIG"); configPath != "" {
		v.SetConfigFile(configPath)
	}
}

type TaskHandler = task.TaskHandler
type StateHandler = state.StateHandler

func RegisterTaskHandler(name string, handl TaskHandler) {
	userTaskHandlers[name] = handl
	if taskScheduler != nil {
		taskScheduler.RegisterHandler(name, handl)
	}
}
func RegisterStateHandler(name string, handl StateHandler) {
	userStateHandlers[name] = handl
	if stateManager != nil {
		stateManager.RegisterHandler(name, handl)

	}
}

// SetConfigFile explicitly defines the path, name and extension of the config file.
func SetConfigFile(conf string) {
	v.SetConfigFile(conf)
}
func SetLogLevel(lvl zerolog.Level) {
	log.Level(lvl)
}

// Init initializes the configuration and sets up logging.
// It returns an error if initialization fails.
func Init() error {

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Warn().Str("path", v.ConfigFileUsed()).Msg("No config file found. Looking for configuration via environment variables.")
		} else {
			return err
		}
	} else {
		log.Info().Str("path", v.ConfigFileUsed()).Msg("Configuration loaded successfully from file")
	}

	configMutex.Lock()
	defer configMutex.Unlock()
	if err := v.Unmarshal(&currentConfig); err != nil {
		return err
	}

	if len(currentConfig.States) == 0 {
		return &NoStatesError{}
	}
	stateManager = state.NewManager(currentConfig.States, log)
	for i, n := range userStateHandlers {
		stateManager.RegisterHandler(i, n)
	}
	taskScheduler = task.NewScheduler(currentConfig.Tasks, log)
	for i, n := range userTaskHandlers {
		taskScheduler.RegisterHandler(i, n)
	}

	return nil
}
func Next() {
	if stateManager != nil {
		stateManager.Next()
	}
}

// Start runs the main logic of the service, including state management and task scheduling.
// It blocks until a termination signal is received or the context is cancelled.
func Start(ctx context.Context) {

	v.OnConfigChange(func(e fsnotify.Event) {
		log.Info().Str("event", e.String()).Msg("Config file changed, attempting to reload...")

		var newConfig config.Config
		configMutex.Lock()
		defer configMutex.Unlock()

		if err := v.Unmarshal(&newConfig); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal updated configuration. Keeping old config.")
			return
		}

		if len(newConfig.States) == 0 {
			log.Error().Msg("Updated configuration has no states. Keeping old config.")
			return
		}

		currentConfig = newConfig
		log.Info().Msg("Configuration reloaded successfully.")
		stateManager.SetStates(currentConfig.States)
		taskScheduler.SetTasks(currentConfig.Tasks, stateManager)
	})

	v.WatchConfig()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		stateManager.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		taskScheduler.Run(ctx, stateManager)
	}()

	//sigChan := make(chan os.Signal, 1)
	//signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	log.Info().Int("pid", os.Getpid()).Msg("Service daemon started. Waiting for signals.")

	<-ctx.Done()
	log.Info().Msg("Context cancelled, shutting down")

}

// NoStatesError is a custom error type for when no states are configured.
type NoStatesError struct{}

func (e *NoStatesError) Error() string {
	return "configuration must contain at least one state"
}
func GetCurrentState() string {
	return stateManager.CurrentStateName()
}
