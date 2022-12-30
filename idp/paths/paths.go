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
	CreateAccessorPath = BaseConfigAccessorPath
	DeleteAccessorPath = singleConfigAccessorPath
	GetAccessorPath    = singleConfigAccessorPath
	ListAccessorsPath  = BaseConfigAccessorPath
	UpdateAccessorPath = singleConfigAccessorPath

	BaseAPIPath = fmt.Sprintf("%s/api", UserStoreBasePath)

	BaseAccessorPath    = fmt.Sprintf("%s/accessors", BaseAPIPath)
	ExecuteAccessorPath = BaseAccessorPath
)

// StripBase makes the URLs functional for handler setup
func StripBase(path string) string {
	return strings.TrimPrefix(path, UserStoreBasePath)
}
