package util

import (
	"os"
	"strings"
)

func ContainsWarFiles(dir string) (bool, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".war") {
			return true, nil
		}
	}
	return false, nil
}
