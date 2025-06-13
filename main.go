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
package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/Lunal98/go-sentinel/config"
	"github.com/Lunal98/go-sentinel/state"
	"github.com/Lunal98/go-sentinel/task"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"

	"github.com/spf13/viper"
)

var (
	configMutex   sync.RWMutex
	currentConfig config.Config
	v             *viper.Viper
	stateManager  *state.Manager
	taskScheduler *task.Scheduler
	log           *zerolog.Logger
)

// Init initializes the configuration and sets up logging.
// It returns an error if initialization fails.
func init() {
	*log = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
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

// SetConfigFile explicitly defines the path, name and extension of the config file.
func SetConfigFile(conf string) {
	v.SetConfigFile(conf)
}
func SetLogLevel(lvl zerolog.Level) {
	log.Level(lvl)
}

// Initialize configuration
func Init() error {

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Warn().Str("path", v.ConfigFileUsed()).Msg("No config file found. Looking for configuration via environment variables.")
		} else {
			return err // Return error for unhandled cases
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
		return &NoStatesError{} // Custom error for clarity
	}

	return nil
}

// Start runs the main logic of the service, including state management and task scheduling.
// It blocks until a termination signal is received or the context is cancelled.
func Start(ctx context.Context) {

	stateManager = state.NewManager(currentConfig.States, log)
	taskScheduler = task.NewScheduler(currentConfig.Tasks, log)

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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	log.Info().Int("pid", os.Getpid()).Msg("Service daemon started. Waiting for signals.")

	for {
		select {
		case s := <-sigChan:
			switch s {
			case syscall.SIGHUP:
				log.Info().Msg("SIGHUP received, rotating to the next state")
				stateManager.Next()
			case syscall.SIGINT, syscall.SIGTERM:
				log.Info().Msg("Termination signal received, shutting down")
				return // Exit the loop and proceed to cleanup
			}
		case <-ctx.Done():
			log.Info().Msg("Context cancelled, shutting down")
			return // Exit the loop and proceed to cleanup
		}
	}
}

// NoStatesError is a custom error type for when no states are configured.
type NoStatesError struct{}

func (e *NoStatesError) Error() string {
	return "configuration must contain at least one state"
}

func main() {
	if err := Init(); err != nil {
		if _, ok := err.(*NoStatesError); ok {
			log.Fatal().Err(err).Msg("Initialization failed")
		} else {
			log.Fatal().Err(err).Msg("Failed to initialize service")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called to clean up resources

	Start(ctx)

	log.Info().Msg("Service daemon stopped gracefully")
}
