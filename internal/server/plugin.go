package server

// PluginRoute describes an HTTP route a plugin handles.
type PluginRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// PluginInfo is metadata returned by a plugin's "info" command.
type PluginInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Routes      []PluginRoute `json:"routes"`
	BinaryPath  string        `json:"-"` // resolved path to compiled binary
}

// PluginRequest is the JSON sent to a plugin's stdin on "handle".
type PluginRequest struct {
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

// PluginResponse is the JSON a plugin writes to stdout.
type PluginResponse struct {
	Tasks []PluginTask `json:"tasks"`
}

// PluginTask is a task to create, as returned by a plugin.
type PluginTask struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Group       string   `json:"group"`
}
