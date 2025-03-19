package config

import (
	"os"
)

func checkForFlag(flagName string) bool {
	for _, arg := range os.Args[1:] {
		if arg == "-"+flagName || arg == "--"+flagName || arg == "-"+flagName[:1] || arg == "--"+flagName[:1] {
			return true
		}
	}

	return false
}
