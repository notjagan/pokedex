package sprite

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
)

type Sprite string

func (s *Sprite) Filepath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %w", err)
	}

	cwd := path.Dir(exe)
	path := path.Join(cwd, string(*s))

	return filepath.FromSlash(path), nil
}
