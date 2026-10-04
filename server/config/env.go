package config

import (
	"log"
	"os"
)

var requiredEnvs = [...]string{"JURY_ADMIN_PASSWORD"}

// Checks to see if all required environmental variables are defined
func CheckEnv() {
	for _, v := range requiredEnvs {
		if !hasEnv(v) {
			log.Fatalf("ERROR: %s environmental variable not defined\n", v)
		}
	}
}

// hasEnv returns true if the environmental variable is defined and not empty
func hasEnv(key string) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return false
	}
	return val != ""
}

// GetEnv returns the value of the environmental variable or panics if it does not exist
func GetEnv(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalf("ERROR: %s environmental variable not defined\n", key)
		return ""
	}
	return val
}

// GetOptEnv returns the value of the environmental variable or the default value if it does not exist
func GetOptEnv(key string, defaultVal string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}
	return val
}
