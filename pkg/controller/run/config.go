package run

type Config struct {
	Files         []*File
	IgnoreActions []*IgnoreAction `yaml:"ignore_actions"`
}

type File struct {
	Pattern string
}

type IgnoreAction struct {
	Name string
}
