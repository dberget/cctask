package claude

// ProcessOutputMsg is sent when a background claude process produces output.
type ProcessOutputMsg struct {
	ID      string
	Output  string
	LogFile string
}

// ProcessDoneMsg is sent when a background claude process completes.
type ProcessDoneMsg struct {
	ID      string
	Output  string
	LogFile string
	Err     error
}
