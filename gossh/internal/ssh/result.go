package ssh

type Result struct {
	Host     string
	Command  string
	Output   string
	Success  bool
	ExitCode int
	Error    string
}

func (r *Result) HasError() bool {
	return !r.Success || r.Error != ""
}
