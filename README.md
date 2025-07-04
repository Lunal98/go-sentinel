## README.md for go-sentinel


# go-sentinel: A Go-based Process & State Manager

go-sentinel is a lightweight, Go-based daemon designed to manage long-running processes (referred to as "states") and execute scheduled checks based on system conditions or time. It provides robust state management, graceful process lifecycle control, and dynamic configuration reloading, making it ideal for maintaining the desired operational state of your system.

## ✨ Features

* **State Management:** Define and transition between different operational states, each launching a specified long-running process with customizable handlers.
* **Check Scheduling:** Execute one-shot or periodic checks.
* **Conditional Check Execution:** Run checks only when the system is in a specific defined state.
* **Extensible Handlers:** Easily integrate new types of checks or states (e.g., mount checks, process restarts, custom cmd wrappers).
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

	// Register custom state and check handlers *before* starting sentinel.
	// Replace 'myCustomCheckHandler' and 'myCustomStateHandler' with your actual implementations.
	// You would typically define these structs and their methods elsewhere in your application.
	sentinel.RegisterCheckHandler("mycustomcheckhandler", &myCustomCheckHandler{})
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


#### Usage & API
The `sentinel` library offers a set of exposed functions for users to seamlessly integrate and control its behavior. Here's a breakdown of the key functions and their uses:

  * **`Init()`**: This function is crucial for initializing the library, reading the configuration, and setting up the internal components. It should be called once at the beginning of your application's lifecycle. For example:

    ```go
    if err := sentinel.Init(); err != nil {
        log.Fatalf("Failed to initialize sentinel: %v", err)
    }
    ```

  * **`SetConfigFile(path string)`**: Use this to explicitly define the path to your configuration file (e.g., `config.yaml`). If not set, Sentinel will look for `config.yaml` in standard locations.

    ```go
    sentinel.SetConfigFile("./my_custom_config.yaml")
    ```

  * **`SetLogLevel(level zerolog.Level)`**: This allows you to control the verbosity of the library's logging output. You can set it to levels like `zerolog.InfoLevel`, `zerolog.DebugLevel`, or `zerolog.ErrorLevel`.

    ```go
    sentinel.SetLogLevel(zerolog.DebugLevel)
    ```

  * **`RegisterCheckHandler(name string, handler CheckHandler)`**: This function is used to register custom functions that will be executed as "checks" within your defined states. The `name` provided here should match the check name specified in your configuration.

    ```go
    sentinel.RegisterCheckHandler("my_custom_check", myCheckLogic)
    ```

  * **`RegisterStateHandler(name string, handler StateHandler)`**: Similar to check handlers, this allows you to register custom functions that will be called when the system enters a specific state. The `name` should correspond to a state defined in your configuration.
    ```go
    sentinel.RegisterStateHandler("initial_state", myStateEntryLogic)
    ```
  * **`Start(ctx context.Context)`**: This is the main entry point to run the Sentinel service. It will block until the provided context is cancelled, orchestrating state transitions and check execution as defined in your configuration.

    ```go
    ctx, cancel := context.WithCancel(context.Background())
    // In a real application, you'd typically handle OS signals to cancel the context
    go func() {
        // Simulate some condition to stop the service
        time.Sleep(10 * time.Minute)
        cancel()
    }()
    sentinel.Start(ctx)
    ```

  * **`Next()`**: Transition to the next state

    ```go
    for {
		  select {
		  case s := <-sigChan:
			  switch s {
			  case syscall.SIGUSR1:
				  log.Info().Msg("SIGUSR1 received, rotating to the next state")
				  sentinel.Next()
        //...
        }
      }
    }
    ```

  * **`GetCurrentState()`**: This function allows you to retrieve the name of the currently active state.

    ```go
    currentState := sentinel.GetCurrentState()
    fmt.Printf("Current active state: %s\n", currentState)
    ```


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
# Checks define checks that make sure the dependcies of the States are in a healthy condition
checks:
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
    # Actions with type "process" will restart the current check if the specified process is not running
    action:
      type: process
      params:
        procname: '/usr/bin/clvc'

  ```



## 🛠️ Development

### Project Structure

  * `/`: Entry point, handles configuration loading, signal handling, and orchestrates the state and check managers.
  * `config/`: Defines the structure for `go-sentinel`'s YAML configuration.
  * `state/`: Manages the lifecycle of long-running processes ("states"), including starting, stopping, and transitioning between them.
  * `state/handlers`: Implements the check handlers.
  * `check/`: Implements the check scheduling logic and defines interfaces for custom check handlers.
  * `check/handlers`: Implements the check handlers.
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
### Adding New Check Handlers

To extend `go-sentinel` with new functionality, implement the `CheckHandler` interface:

```go
// check/registry.go
type CheckHandler interface {
    Execute(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error
}
```

Then, register your new handler using sentinel.RegisterCheckHandler in your main function 

```go

 sentinel.RegisterCheckHandler("your_new_action_type", &MyCustomCheckHandler{})


```
