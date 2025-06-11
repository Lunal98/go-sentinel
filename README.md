## README.md for go-sentinel


# go-sentinel: A Go-based Process & State Manager

go-sentinel is a lightweight, Go-based daemon designed to manage long-running processes (referred to as "states") and execute scheduled tasks based on system conditions or time. It provides robust state management, graceful process lifecycle control, and dynamic configuration reloading, making it ideal for maintaining the desired operational state of your system.

## ✨ Features

* **State Management:** Define and transition between different operational states, each launching a specified long-running process.
* **Graceful Process Control:** Ensures processes are started and stopped cleanly, with support for `SIGTERM` and `SIGKILL` fallbacks.
* **Dynamic Configuration Reloading:** Update `go-sentinel`'s behavior on the fly by modifying its YAML configuration file (supports `SIGHUP` for state transitions).
* **Task Scheduling:** Execute one-shot or periodic tasks.
* **Conditional Task Execution:** Run tasks only when the system is in a specific defined state.
* **Extensible Task Handlers:** Easily integrate new types of tasks (e.g., mount checks, process restarts).
* **Robust Logging:** Provides detailed logging using `zerolog` for easy debugging and monitoring.

## 🚀 Getting Started

### Prerequisites

* Go (1.24 or higher recommended)

### Installation

1.  **Clone the repository:**
    ```bash
    git clone [https://github.com/Lunal98/go-sentinel.git](https://github.com/Lunal98/go-sentinel.git)
    cd go-sentinel
    ```
2.  **Build the executable:**
    ```bash
    go build -o go-sentinel .
    ```
    This will create a `go-sentinel` executable in your current directory.

### Configuration

`go-sentinel` uses a YAML configuration file (e.g., `config.yaml`). It searches for this file in the following locations, in order:
1. `/etc/go-sentinel/`
2. `$HOME/.go-sentinel/`
3. The current working directory

You specify the path of the config path using the `go-sentinel_CONFIG` environment variable.
Environment variables prefixed with `go-sentinel_` (e.g., `go-sentinel_STATES_0_NAME`) can override configuration values.

### Running go-sentinel

`go-sentinel` is designed to be used as a systemctl service

#### Example systemd file

```
[Unit]
Description=go-sentinel Daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/go-go-sentinel
Restart=on-failure
RestartSec=5
ExecReload=/bin/kill -HUP $MAINPID

# Set the path to the configuration file here.
Environment="go-sentinel_CONFIG=/etc/go-sentinel/config.yml"

[Install]
WantedBy=multi-user.target
```

#### Example `config.yaml`

```yaml
# States define long-running processes that go-sentinel manages.
states:
  - name:
    cmd: '/usr/bin/cvlc --loop /mnt/fileserver/asdasd'
  - name2:
    cmd: '/usr/bin/fbi /mnt/fileserver/wp/*.jpg'
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
        procname: '/usr/bin/clvc'````


### Signals

`go-sentinel` responds to the following OS signals:

  * `SIGINT` (Ctrl+C) / `SIGTERM`: Gracefully shuts down `go-sentinel` and any running processes.
  * `SIGHUP`: Triggers `go-sentinel` to rotate to the next defined state in its configuration. Also triggers a configuration file reload if changes are detected.

## 🛠️ Development

### Project Structure

  * `main.go`: Entry point, handles configuration loading, signal handling, and orchestrates the state and task managers.
  * `config/`: Defines the structure for `go-sentinel`'s YAML configuration.
  * `state/`: Manages the lifecycle of long-running processes ("states"), including starting, stopping, and transitioning between them.
  * `task/`: Implements the task scheduling logic and defines interfaces for custom task handlers.
  * `utils/`: Contains utility functions (e.g., for checking mount status).

### Adding New Task Handlers

To extend `go-sentinel` with new functionality, implement the `TaskHandler` interface:

```go
// task/registry.go
type TaskHandler interface {
    Execute(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error
}
```

Then, register your new handler in `main.go`:

```go
// main.go

// ... in main()
taskHandlers.Register("your_new_action_type", &task.YourNewTaskHandler{})
```
