package parser

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// ParseFlags parses the flags and returns a map of the flags and their values
func ParseFlags(cfg any, t ...reflect.Type) map[string]*string {
	flags := make(map[string]*string)

	if len(t) == 0 {
		v := reflect.ValueOf(cfg).Elem()
		t = []reflect.Type{v.Type()}
	}

	// Register flags
	for i := 0; i < t[0].NumField(); i++ {
		field := t[0].Field(i)
		if flagName := field.Tag.Get("flag"); flagName != "" {
			flags[field.Name] = flag.String(flagName, "", field.Tag.Get("usage"))
		}
	}

	flag.Parse()

	return flags
}

// ParseConfig parses the config and returns all errors that occurred while parsing the config
func ParseConfig(cfg any) []error {
	var errors []error

	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	flags := ParseFlags(cfg, t)

	// Set values and collect errors
	for i := range t.NumField() {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip if field has no tags at all
		if field.Tag == "" {
			continue
		}

		// Check flag value first
		if flagVal, ok := flags[field.Name]; ok && *flagVal != "" {
			fieldValue.SetString(*flagVal)
			continue // Skip env vars if flag is set
		}

		// If no flag value, check environment variables
		for _, env := range strings.Split(field.Tag.Get("env"), ",") {
			if val := os.Getenv(env); val != "" {
				fieldValue.SetString(val)
				break
			}
		}

		// If no value set yet, use default value if present
		if fieldValue.String() == "" {
			if _, exists := field.Tag.Lookup("default"); exists {
				fieldValue.SetString(field.Tag.Get("default"))
				continue
			}
		}

		// collect errors only if no value is set and no default exists
		if fieldValue.String() == "" {
			flagName := field.Tag.Get("flag")
			envVars := field.Tag.Get("env")

			if flagName != "" {
				errors = append(errors, fmt.Errorf("%s is required: set %s in env or use --%s", field.Name, envVars, flagName))
			} else {
				errors = append(errors, fmt.Errorf("%s is required: set one of: %s in env", field.Name, envVars))
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}
