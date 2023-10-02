package universe

import (
	"fmt"
	"os"

	"userclouds.com/infra/ucerr"
)

// Universe represents a universe (or environment) that UC code runs in
type Universe string

// Environment keys for config settings
// We use these instead of command line args because it works better with `go test`
const (
	EnvKeyUniverse = "UC_UNIVERSE"
)

// Supported universes.
// NOTE: keep in sync with `enum Universe` in TSX codebase
const (
	Dev     Universe = "dev"     // local dev laptops
	Test    Universe = "test"    // automated tests on localhost
	CI      Universe = "ci"      // AWS continuous integration env
	Debug   Universe = "debug"   // AWS EB universe to debug off master
	Staging Universe = "staging" // cloud hosted staging universe (similar to prod)
	Prod    Universe = "prod"    // user-facing prod deployment
)

//go:generate genstringconstenum Universe

// Current checks the current application environment.
func Current() Universe {
	u := Universe(os.Getenv(EnvKeyUniverse))
	if err := u.Validate(); err != nil {
		panic(fmt.Sprintf("invalid universe from environment: %v", u))
	}
	return u
}

// IsProd returns true if universe is prod
func (u Universe) IsProd() bool {
	return u == Prod
}

// IsProdOrStaging returns true if universe is prod or staging
func (u Universe) IsProdOrStaging() bool {
	return u == Prod || u == Staging
}

// IsDev returns true if universe is dev
func (u Universe) IsDev() bool {
	return u == Dev
}

// IsCloud returns true if universe is one of the cloud envs (prod, staging debug)
func (u Universe) IsCloud() bool {
	return u.IsProdOrStaging() || u == Debug
}

// IsTestOrCI returns true if universe is CI or tests
func (u Universe) IsTestOrCI() bool {
	return u == CI || u == Test
}

// AllUniverses returns a list of all known universes
// Useful for config testing etc
func AllUniverses() []Universe {
	return allUniverseValues
}

// Validate implements Validateable
func (u Universe) Validate() error {
	for _, v := range allUniverseValues {
		if u == v {
			return nil
		}
	}
	return ucerr.Errorf("unknown Universe value %v", u)
}
