package util

import (
	"os"
	"strings"
)

func ReplaceInCatalinaProps(fileName string) error {
	input, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}

	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		if strings.Contains(line, "common.loader=") {
			lines[i] = strings.Replace(lines[i], "common.loader=", "common.loader=${BPI_TOMCAT_ADDITIONAL_COMMON_JARS},", 1)
		}
	}
	output := strings.Join(lines, "\n")
	err = os.WriteFile(fileName, []byte(output), 0644)
	if err != nil {
		return err
	}
	return nil
}
