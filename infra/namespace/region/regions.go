package region

import (
	"fmt"
	"os"
	"strings"
)

// Region represents a UserClouds region (eg. datacenter, ish)
type Region string

// Regions simply lists the regions UC current runs in
// not sure these will live here forever but a starting place
var Regions = []Region{"aws-us-west-2", "aws-us-east-1"}

// Current returns the current region, or empty string
// TODO: error check against known list?
func Current() Region {
	return Region(os.Getenv("REGION"))
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
