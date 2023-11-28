package region

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const regionEnvVar = "UC_REGION"

// Region represents a UserClouds region (eg. datacenter, ish)
type Region string

// Regions simply lists the regions UC current runs in
// not sure these will live here forever but a starting place
var Regions = []Region{"aws-us-west-2", "aws-us-east-1"}

// InitLogger lets us pass in the logger to avoid import cycles
// TODO (sgarrity 10/23): remove this when we remove old REGION env var
func InitLogger(lf loggerFn) {
	logger = lf
}

type loggerFn func(context.Context, string, ...interface{})

var logger loggerFn

// Current returns the current region, or empty string
// TODO: error check against known list?
func Current() Region {
	r := os.Getenv(regionEnvVar)
	if r != "" {
		return Region(r)
	}

	// TODO (sgarrity 10/23): remove this once we're sure we don't hit it anymore
	r = os.Getenv("REGION")
	if logger != nil {
		// this is super janky, but transportLogServer.go calls `region.Current()` during init
		// and this callback then deadlocks. Rather than rewrite our logging to fix this
		// (we should just queue messages during init, or write them only to already-inited
		// transports), since this is just a temporary warning, we'll just sleep for a bit to let the
		// init finish and then log the warning.
		go func() {
			time.Sleep(5 * time.Second)
			logger(context.Background(), "using old REGION env var: %v", r)
		}()
	}

	return Region(r)
}

// FromAWSRegion returns a region from a aws region string. e.g. us-east-1, us-west-2
func FromAWSRegion(awsRegion string) Region {
	return Region(fmt.Sprintf("aws-%s", awsRegion))
}

// GetAWSRegion returns the AWS name of the region and blank if region is not in AWS
func GetAWSRegion(r Region) string {
	if strings.HasPrefix(string(r), "aws-") {
		return strings.TrimPrefix(string(r), "aws-")
	}
	// TODO maybe makes to error
	return ""
}

// IsValid returns true if the region is a valid region
func IsValid(region Region) bool {
	for _, r := range Regions {
		if r == region {
			return true
		}
	}
	return false
}
