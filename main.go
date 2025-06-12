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
	taskhandlers "github.com/Lunal98/go-sentinel/task/handlers"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var configMutex sync.RWMutex
var currentConfig config.Config

func main() {

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/etc/go-sentinel/")
	v.AddConfigPath("$HOME/.go-sentinel/")
	v.AddConfigPath(".")
	v.SetEnvPrefix("go-sentinel")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if configPath := os.Getenv("go-sentinel_CONFIG"); configPath != "" {
		v.SetConfigFile(configPath)
		log.Info().Str("path", configPath).Msg("Using config file from go-sentinel_CONFIG environment variable")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Warn().Msg("No config file found. Looking for configuration via environment variables.")
		} else {
			log.Fatal().Err(err).Msg("Failed to read configuration file")
		}
	} else {
		log.Info().Str("path", v.ConfigFileUsed()).Msg("Configuration loaded successfully from file")
	}

	configMutex.Lock()
	if err := v.Unmarshal(&currentConfig); err != nil {
		configMutex.Unlock()
		log.Fatal().Err(err).Msg("Failed to unmarshal initial configuration")
	}
	configMutex.Unlock()
	if len(currentConfig.States) == 0 {

		log.Fatal().Msg("Configuration must contain at least one state")
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	configMutex.RLock()
	stateManager := state.NewManager(currentConfig.States, &log.Logger)
	configMutex.RUnlock()

	taskHandlers := make(taskhandlers.Registry)
	taskHandlers.Register("mount", &taskhandlers.MountTaskHandler{})
	taskHandlers.Register("process", &taskhandlers.ProcessTaskHandler{})

	configMutex.RLock()
	taskScheduler := task.NewScheduler(currentConfig.Tasks, taskHandlers, &log.Logger)
	configMutex.RUnlock()
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
				cancel()
				goto cleanup
			}
		case <-ctx.Done():
			log.Info().Msg("Context cancelled, shutting down")
			goto cleanup
		}
	}

cleanup:
	wg.Wait()
	log.Info().Msg("Service daemon stopped gracefully")
}
