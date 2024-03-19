package region

import (
	"fmt"
	"os"
	"strings"

	"userclouds.com/infra/namespace/universe"
	"userclouds.com/infra/ucerr"
)

const regionEnvVar = "UC_REGION"

// MachineRegion represents a region for our systems or located
type MachineRegion string

// machineRegions is a list of regions (real or fake) UC runs in for each universe
var machineRegions = map[universe.Universe][]MachineRegion{
	universe.Prod:      {"aws-us-west-2", "aws-us-east-1", "aws-eu-west-1"},
	universe.Staging:   {"aws-us-west-2", "aws-us-east-1", "aws-eu-west-1"},
	universe.Debug:     {"aws-us-west-2", "aws-us-east-1", "aws-eu-west-1"},
	universe.Dev:       {"themoon", "mars"},
	universe.Container: {""},
	universe.CI:        {""},
	universe.Test:      {""},
}

// MachineRegionsForUniverse returns the list of regions for a given universe
func MachineRegionsForUniverse(u universe.Universe) []MachineRegion {
	return machineRegions[u]
}

// Current returns the current region, or empty string
// TODO: error check against known list?
func Current() MachineRegion {
	r := os.Getenv(regionEnvVar)
	return MachineRegion(r)
}

// FromAWSRegion returns a region from a aws region string. e.g. us-east-1, us-west-2
func FromAWSRegion(awsRegion string) MachineRegion {
	return MachineRegion(fmt.Sprintf("aws-%s", awsRegion))
}

// GetAWSRegion returns the AWS name of the region and blank if region is not in AWS
func GetAWSRegion(r MachineRegion) string {
	if strings.HasPrefix(string(r), "aws-") {
		return strings.TrimPrefix(string(r), "aws-")
	}
	// TODO maybe makes to error
	return ""
}

// IsValid returns true if the region is a valid region for a given universe
func IsValid(region MachineRegion, u universe.Universe) bool {
	for _, r := range machineRegions[u] {
		if r == region {
			return true
		}
	}
	return false
}

// Validate implements Validateable
func (r MachineRegion) Validate() error {
	if IsValid(r, universe.Current()) {
		return nil
	}

	return ucerr.Friendlyf(nil, "invalid machine region: %s", r)
}

// DataRegion represents a region for where user data should be hosted
type DataRegion string

// dataRegions is a list of regions that user data can be hosted in
var dataRegions = map[universe.Universe][]DataRegion{
	universe.Prod:      {"aws-us-west-2", "aws-us-east-1", "aws-eu-west-1"},
	universe.Staging:   {"aws-us-west-2"},
	universe.Debug:     {""},
	universe.Dev:       {""},
	universe.Container: {""},
	universe.CI:        {"aws-us-west-2", "aws-us-east-1", "aws-eu-west-1"},
	universe.Test:      {"aws-us-west-2", "aws-us-east-1", "aws-eu-west-1"},
}

// DataRegionsForUniverse returns the list of regions for a given universe
func DataRegionsForUniverse(u universe.Universe) []DataRegion {
	return dataRegions[u]
}

// Validate implements Validateable
func (r DataRegion) Validate() error {
	for _, reg := range dataRegions[universe.Current()] {
		if string(r) == string(reg) {
			return nil
		}
	}

	// We allow empty data regions since each db will use its primary region by default
	if r == "" {
		return nil
	}

	return ucerr.Friendlyf(nil, "invalid data region: %s", r)
}
