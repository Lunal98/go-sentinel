package config

// Config is the top-level structure for the application's configuration.
type Config struct {
	States []State `mapstructure:"states"`
	Checks []Check `mapstructure:"checks"`
}

// State defines a command to be run as a managed process.
type State struct {
	Name   string                 `mapstructure:"name"`
	Type   string                 `mapstructure:"type"`
	Params map[string]interface{} `mapstructure:"params"`
}

// Check defines a periodic or one-shot action to be performed.
type Check struct {
	Name      string     `mapstructure:"name"`
	Frequency Frequency  `mapstructure:"frequency"`
	Condition *Condition `mapstructure:"condition,omitempty"`
	Action    Action     `mapstructure:"action"`
}

// Frequency defines how often a Check should run.
type Frequency struct {
	Type string `mapstructure:"type"` // "oneshot" or "periodic"
	Time string `mapstructure:"time,omitempty"`
}

// Condition specifies prerequisites for a Check to run.
type Condition struct {
	State string `mapstructure:"state"` // The name of the state that must be active.
}

// Action defines the type of operation for a Check.
type Action struct {
	Type   string                 `mapstructure:"type"`
	Params map[string]interface{} `mapstructure:"params"`
}
