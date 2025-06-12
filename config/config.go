package config

// Config is the top-level structure for the application's configuration.
type Config struct {
	States []State `mapstructure:"states"`
	Tasks  []Task  `mapstructure:"tasks"`
}

// State defines a command to be run as a managed process.
type State struct {
	Name   string                 `mapstructure:"name"`
	Type   string                 `mapstructure:"type"`
	Params map[string]interface{} `mapstructure:"params"`
}

// Task defines a periodic or one-shot action to be performed.
type Task struct {
	Name      string     `mapstructure:"name"`
	Frequency Frequency  `mapstructure:"frequency"`
	Condition *Condition `mapstructure:"condition,omitempty"`
	Action    Action     `mapstructure:"action"`
}

// Frequency defines how often a task should run.
type Frequency struct {
	Type string `mapstructure:"type"` // "oneshot" or "periodic"
	Time string `mapstructure:"time,omitempty"`
}

// Condition specifies prerequisites for a task to run.
type Condition struct {
	State string `mapstructure:"state"` // The name of the state that must be active.
}

// Action defines the type of operation for a task.
type Action struct {
	Type   string                 `mapstructure:"type"`
	Params map[string]interface{} `mapstructure:"params"`
}
