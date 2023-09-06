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
	CreateUser     = fmt.Sprintf("%s/users", IDPBasePath)
	AddAuthnToUser = fmt.Sprintf("%s/addauthntouser", IDPBasePath)

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
		return fmt.Sprintf("%s/%s?accessor_version=%d", BaseConfigAccessorPath, id, version)
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
	versionedSingleConfigMutatorPath = func(id uuid.UUID, version int) string {
		return fmt.Sprintf("%s/%s?mutator_version=%d", BaseConfigMutatorPath, id, version)
	}
	CreateMutatorPath       = BaseConfigMutatorPath
	DeleteMutatorPath       = singleConfigMutatorPath
	GetMutatorPath          = singleConfigMutatorPath
	GetMutatorByVersionPath = versionedSingleConfigMutatorPath
	ListMutatorsPath        = BaseConfigMutatorPath
	UpdateMutatorPath       = singleConfigMutatorPath

	BaseMutatorPath    = fmt.Sprintf("%s/mutators", BaseAPIPath)
	ExecuteMutatorPath = BaseMutatorPath

	BaseConfigPurposePath   = fmt.Sprintf("%s/purposes", BaseConfigPath)
	singleConfigPurposePath = func(id uuid.UUID) string {
		return fmt.Sprintf("%s/%s", BaseConfigPurposePath, id)
	}

	CreatePurposePath = BaseConfigPurposePath
	ListPurposesPath  = BaseConfigPurposePath
	GetPurposePath    = singleConfigPurposePath
	DeletePurposePath = singleConfigPurposePath
	UpdatePurposePath = singleConfigPurposePath

	GetConsentedPurposesForUserPath = fmt.Sprintf("%s/consentedpurposes", UserStoreBasePath)

	BaseAPIPath = fmt.Sprintf("%s/api", UserStoreBasePath)

	TokenizerBasePath = "/tokenizer"

	BaseTokenPath        = fmt.Sprintf("%s/tokens", TokenizerBasePath)
	CreateToken          = BaseTokenPath
	DeleteToken          = BaseTokenPath
	ResolveToken         = fmt.Sprintf("%s/actions/resolve", BaseTokenPath)
	InspectToken         = fmt.Sprintf("%s/actions/inspect", BaseTokenPath)
	LookupToken          = fmt.Sprintf("%s/actions/lookup", BaseTokenPath)
	LookupOrCreateTokens = fmt.Sprintf("%s/actions/lookuporcreate", BaseTokenPath)

	BasePolicyPath = fmt.Sprintf("%s/policies", TokenizerBasePath)

	BaseAccessPolicyPath = fmt.Sprintf("%s/access", BasePolicyPath)
	ListAccessPolicies   = BaseAccessPolicyPath
	GetAccessPolicy      = func(id uuid.UUID) string {
		return fmt.Sprintf("%s/%s", BaseAccessPolicyPath, id)
	}
	GetAccessPolicyByName = func(name string) string {
		return fmt.Sprintf("%s?name=%s", BaseAccessPolicyPath, name)
	}
	GetAccessPolicyByVersion = func(id uuid.UUID, version int) string {
		return fmt.Sprintf("%s/%s?policy_version=%d", BaseAccessPolicyPath, id, version)
	}
	GetAccessPolicyByNameAndVersion = func(name string, version int) string {
		return fmt.Sprintf("%s?name=%s&policy_version=%d", BaseAccessPolicyPath, name, version)
	}
	CreateAccessPolicy = BaseAccessPolicyPath
	UpdateAccessPolicy = func(id uuid.UUID) string { return fmt.Sprintf("%s/%s", BaseAccessPolicyPath, id) }
	DeleteAccessPolicy = func(id uuid.UUID, version int) string {
		return fmt.Sprintf("%s/%s?policy_version=%d", BaseAccessPolicyPath, id, version)
	}
	TestAccessPolicy    = fmt.Sprintf("%s/actions/test", BaseAccessPolicyPath)
	ExecuteAccessPolicy = fmt.Sprintf("%s/actions/execute", BaseAccessPolicyPath)

	BaseAccessPolicyTemplatePath = fmt.Sprintf("%s/accesstemplate", BasePolicyPath)
	ListAccessPolicyTemplates    = BaseAccessPolicyTemplatePath
	GetAccessPolicyTemplate      = func(id uuid.UUID) string {
		return fmt.Sprintf("%s/%s", BaseAccessPolicyTemplatePath, id)
	}
	GetAccessPolicyTemplateByName = func(name string) string {
		return fmt.Sprintf("%s?name=%s", BaseAccessPolicyTemplatePath, name)
	}
	GetAccessPolicyTemplateByVersion = func(id uuid.UUID, version int) string {
		return fmt.Sprintf("%s/%s?template_version=%d", BaseAccessPolicyTemplatePath, id, version)
	}
	GetAccessPolicyTemplateByNameAndVersion = func(name string, version int) string {
		return fmt.Sprintf("%s?name=%s&template_version=%d", BaseAccessPolicyTemplatePath, name, version)
	}
	CreateAccessPolicyTemplate = BaseAccessPolicyTemplatePath
	UpdateAccessPolicyTemplate = func(id uuid.UUID) string { return fmt.Sprintf("%s/%s", BaseAccessPolicyTemplatePath, id) }
	DeleteAccessPolicyTemplate = func(id uuid.UUID, version int) string {
		return fmt.Sprintf("%s/%s?template_version=%d", BaseAccessPolicyTemplatePath, id, version)
	}

	BaseTransformerPath = fmt.Sprintf("%s/transformation", BasePolicyPath)
	ListTransformers    = BaseTransformerPath
	GetTransformer      = func(id uuid.UUID) string {
		return fmt.Sprintf("%s/%s", BaseTransformerPath, id)
	}
	GetTransformerByName = func(name string) string {
		return fmt.Sprintf("%s?name=%s", BaseTransformerPath, name)
	}
	CreateTransformer  = BaseTransformerPath
	DeleteTransformer  = func(id uuid.UUID) string { return fmt.Sprintf("%s/%s", BaseTransformerPath, id) }
	TestTransformer    = fmt.Sprintf("%s/actions/test", BaseTransformerPath)
	ExecuteTransformer = fmt.Sprintf("%s/actions/execute", BaseTransformerPath)
)

// StripUserstoreBase makes the URLs functional for handler setup
func StripUserstoreBase(path string) string {
	return strings.TrimPrefix(path, UserStoreBasePath)
}

// StripTokenizerBase makes the URLs functional for handler setup
func StripTokenizerBase(path string) string {
	return strings.TrimPrefix(path, TokenizerBasePath)
}

// GetReferenceURLForAccessor return URL pointing at a particular access policy object
func GetReferenceURLForAccessor(id uuid.UUID, v int) string {
	return fmt.Sprintf("%s/%s/%d", BaseAccessorPath, id.String(), v)
}

// GetReferenceURLForMutator return URL pointing at a particular transformer object
func GetReferenceURLForMutator(id uuid.UUID, v int) string {
	return fmt.Sprintf("%s/%s/%d", BaseMutatorPath, id.String(), v)
}

// GetReferenceURLForAccessPolicy return URL pointing at a particular access policy object
func GetReferenceURLForAccessPolicy(id uuid.UUID, v int) string {
	return fmt.Sprintf("%s/%s/%d", BaseAccessPolicyPath, id.String(), v)
}

// GetReferenceURLForTransformer return URL pointing at a particular transformer object
func GetReferenceURLForTransformer(id uuid.UUID, v int) string {
	return fmt.Sprintf("%s/%s/%d", BaseTransformerPath, id.String(), v)
}

// GetColumnRetentionDurationURL returns a retention duration URL for the specified column ID and duration type
func GetColumnRetentionDurationURL(columnID uuid.UUID, isPreDelete bool) string {
	path := fmt.Sprintf("%s/%v", BaseConfigColumnsPath, columnID)
	if isPreDelete {
		path += "/predeleteretentiondurations"
	} else {
		path += "/postdeleteretentiondurations"
	}
	return path
}
