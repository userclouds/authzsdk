package paths

import (
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
)

// Path constants for the userstore
var (
	IDPBasePath = "/authn" // TODO change this

	// TODO: finish converting IDP path handling to use these
	CreateUser             = fmt.Sprintf("%s/users", IDPBasePath)
	GetUserByExternalAlias = fmt.Sprintf("%s/users", IDPBasePath)
	AddAuthnToUser         = fmt.Sprintf("%s/addauthntouser", IDPBasePath)

	UserStoreBasePath = "/userstore"

	BaseConfigPath = fmt.Sprintf("%s/config", UserStoreBasePath)

	BaseConfigColumnsPath  = fmt.Sprintf("%s/columns", BaseConfigPath)
	singleConfigColumnPath = func(id uuid.UUID) string {
		return fmt.Sprintf("%s/%s", BaseConfigColumnsPath, id)
	}
	CreateColumnPath = BaseConfigColumnsPath
	DeleteColumnPath = singleConfigColumnPath
	GetColumnPath    = singleConfigColumnPath
	ListColumnsPath  = BaseConfigColumnsPath
	UpdateColumnPath = singleConfigColumnPath

	BaseConfigAccessorPath   = fmt.Sprintf("%s/accessors", BaseConfigPath)
	singleConfigAccessorPath = func(id uuid.UUID) string {
		return fmt.Sprintf("%s/%s", BaseConfigAccessorPath, id)
	}
	versionedSingleConfigAccessorPath = func(id uuid.UUID, version int) string {
		return fmt.Sprintf("%s/%s?version=%d", BaseConfigAccessorPath, id, version)
	}
	CreateAccessorPath       = BaseConfigAccessorPath
	DeleteAccessorPath       = singleConfigAccessorPath
	GetAccessorPath          = singleConfigAccessorPath
	GetAccessorByVersionPath = versionedSingleConfigAccessorPath
	ListAccessorsPath        = BaseConfigAccessorPath
	UpdateAccessorPath       = singleConfigAccessorPath

	BaseAccessorPath    = fmt.Sprintf("%s/accessors", BaseAPIPath)
	ExecuteAccessorPath = BaseAccessorPath

	BaseConfigMutatorPath   = fmt.Sprintf("%s/mutators", BaseConfigPath)
	singleConfigMutatorPath = func(id uuid.UUID) string {
		return fmt.Sprintf("%s/%s", BaseConfigMutatorPath, id)
	}
	CreateMutatorPath = BaseConfigMutatorPath
	DeleteMutatorPath = singleConfigMutatorPath
	GetMutatorPath    = singleConfigMutatorPath
	ListMutatorsPath  = BaseConfigMutatorPath
	UpdateMutatorPath = singleConfigMutatorPath

	BaseMutatorPath    = fmt.Sprintf("%s/mutators", BaseAPIPath)
	ExecuteMutatorPath = BaseMutatorPath

	BaseAPIPath = fmt.Sprintf("%s/api", UserStoreBasePath)
)

// StripBase makes the URLs functional for handler setup
func StripBase(path string) string {
	return strings.TrimPrefix(path, UserStoreBasePath)
}

// GetReferenceURLForAccessor return URL pointing at a particular access policy object
func GetReferenceURLForAccessor(id uuid.UUID, v int) string {
	return fmt.Sprintf("%s/%s/%d", BaseAccessorPath, id.String(), v)
}

// GetReferenceURLForMutator return URL pointing at a particular generation policy object
func GetReferenceURLForMutator(id uuid.UUID, v int) string {
	return fmt.Sprintf("%s/%s/%d", BaseMutatorPath, id.String(), v)
}
