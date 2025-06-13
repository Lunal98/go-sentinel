## README.md for go-sentinel


# go-sentinel: A Go-based Process & State Manager

go-sentinel is a lightweight, Go-based daemon designed to manage long-running processes (referred to as "states") and execute scheduled tasks based on system conditions or time. It provides robust state management, graceful process lifecycle control, and dynamic configuration reloading, making it ideal for maintaining the desired operational state of your system.

## ✨ Features

* **State Management:** Define and transition between different operational states, each launching a specified long-running process with customizable handlers.
* **Task Scheduling:** Execute one-shot or periodic tasks.
* **Conditional Task Execution:** Run tasks only when the system is in a specific defined state.
* **Extensible Handlers:** Easily integrate new types of tasks or states (e.g., mount checks, process restarts, custom cmd wrappers).
* **Robust Logging:** Provides detailed logging using `zerolog` for easy debugging and monitoring.

## 🚀 Getting Started

### Prerequisites

* Go (1.24 or higher recommended)

### Installation

    ```bash
    go get github.com/Lunal98/go-sentinel
    ```
### Basic Usage

go-sentinel is designed to be integrated directly into your Go applications. Here's a basic example:

```go
package main

import (
	"context"

	"os"
	"os/signal"
	"syscall"

	"github.com/Lunal98/go-sentinel"
	"github.com/rs/zerolog"                     // For log level configuration
)

func main() {

	// Optional: Set a specific configuration file path
	// sentinel.SetConfigFile("/etc/my-app/go-sentinel.yaml")

	
	sentinel.SetLogLevel(zerolog.ErrorLevel)

	// Initialize go-sentinel. This loads the configuration and sets up the logger.
	if err := sentinel.Init(); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize sentinel config")
	}

	// Register custom state and task handlers *before* starting sentinel.
	// Replace 'myCustomTaskHandler' and 'myCustomStateHandler' with your actual implementations.
	// You would typically define these structs and their methods elsewhere in your application.
	sentinel.RegisterTaskHandler("mycustomtaskhandler", &myCustomTaskHandler{})
	sentinel.RegisterStateHandler("mycustomstatehandler", &myCustomStateHandler{})

  sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	
	go sentinel.Start(ctx)
		for {
		select {
		case s := <-sigChan:
			switch s {
			case syscall.SIGHUP:
				log.Info().Msg("SIGHUP received, rotating to the next state")
				sentinel.Next()
			case syscall.SIGINT, syscall.SIGTERM:
				log.Info().Msg("Termination signal received, shutting down")
				return
			}
		case <-ctx.Done():
			log.Info().Msg("Context cancelled, shutting down")
			return
		}
	}

}
```

### Configuration

`go-sentinel` uses a YAML configuration file (e.g., `config.yaml`). It searches for this file in the following locations, in order:
1. `/etc/go-sentinel/`
2. `$HOME/.go-sentinel/`
3. The current working directory

You specify the path of the config path using the `GO_SENTINEL_CONFIG` environment variable.
Environment variables prefixed with `GO_SENTINEL_` (e.g., `GO_SENTINEL_STATES_0_NAME`) can override configuration values.

You can explicitly set the configuration file path using `sentinel.SetConfigFile("your/path/to/config.yaml")` before calling `sentinel.Init()`.


#### Example `config.yaml`

```yaml

# States define long-running processes that go-sentinel manages.
states:
  - name:
    type: cmd
    params:
      cmd: '/usr/bin/cvlc --loop /mnt/fileserver/asdasd'
  - name2:
    type: mycustomhandler
    params:
      cmd: '/usr/bin/fbi {{path}}'
      refreshinterval: 5s
      extensionfilter:
        - jpg
        - png
# Tasks define checks that make sure the dependcies of the States are in a healthy condition
tasks:
  - name: mount
    frequency:
      type: oneshot
    # Actions with type "mount" will attemt to run "mount -a" if the specified entry is found in fstab, logging an error otherwise
    action:
      type: mount
      params:
        device: //domain.example.com/smbshare
        dir: /mnt/fileserver
  - name: vlccheck
    frequency:
      type: periodic
      time: 5m
    condition:
      state: name
    # Actions with type "process" will restart the current task if the specified process is not running
    action:
      type: process
      params:
        procname: '/usr/bin/clvc'

  ```



## 🛠️ Development

### Project Structure

  * `/`: Entry point, handles configuration loading, signal handling, and orchestrates the state and task managers.
  * `config/`: Defines the structure for `go-sentinel`'s YAML configuration.
  * `state/`: Manages the lifecycle of long-running processes ("states"), including starting, stopping, and transitioning between them.
  * `state/handlers`: Implements the task handlers.
  * `task/`: Implements the task scheduling logic and defines interfaces for custom task handlers.
  * `task/handlers`: Implements the task handlers.
  * `utils/`: Contains utility functions (e.g., for checking mount status).
  

### Adding New State Handlers

To extend go-sentinel with new state types, implement the StateHandler interface defined in state/handlers/handlers.go:

```go
// state/handlers/handlers.go
type StateHandler interface {
    Start(ctx context.Context, state config.State, log *zerolog.Logger) (*exec.Cmd, error)
    Stop(cmd *exec.Cmd, log *zerolog.Logger) error
    Restart(ctx context.Context, oldCmd *exec.Cmd, state config.State, log *zerolog.Logger) (*exec.Cmd, error)
}
```


Then, register your new handler using sentinel.RegisterStateHandler in your main function
```go
sentinel.RegisterStateHandler("mycustomhandler", &MyCustomStateHandler{})
```
### Adding New Task Handlers

To extend `go-sentinel` with new functionality, implement the `TaskHandler` interface:

```go
// task/registry.go
type TaskHandler interface {
    Execute(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error
}
```

Then, register your new handler using sentinel.RegisterTaskHandler in your main function 

```go

 sentinel.RegisterTaskHandler("your_new_action_type", &MyCustomTaskHandler{})


```
