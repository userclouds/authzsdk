package tokenizer

import (
	"userclouds.com/idp/policy"
	"userclouds.com/infra/ucerr"
)

// CreateAccessPolicyRequest creates a new AP
type CreateAccessPolicyRequest struct {
	AccessPolicy policy.AccessPolicy `json:"access_policy"`
}

//go:generate genvalidate CreateAccessPolicyRequest

// UpdateAccessPolicyRequest updates an AP by creating a new version
type UpdateAccessPolicyRequest struct {
	AccessPolicy policy.AccessPolicy `json:"access_policy"`
}

//go:generate genvalidate UpdateAccessPolicyRequest

// DeleteAccessPolicyRequest is to delete an AP
// The ID is passed in the URL but we need a specific version, and our handler
// infrastructure doesn't currently support arbitrary non-UUID path components
type DeleteAccessPolicyRequest struct {
	Version int `json:"version"`
}

// CreateAccessPolicyTemplateRequest creates a new AP Template
type CreateAccessPolicyTemplateRequest struct {
	AccessPolicyTemplate policy.AccessPolicyTemplate `json:"access_policy_template"`
}

//go:generate genvalidate CreateAccessPolicyTemplateRequest

// UpdateAccessPolicyTemplateRequest updates an AP Template
type UpdateAccessPolicyTemplateRequest struct {
	AccessPolicyTemplate policy.AccessPolicyTemplate `json:"access_policy_template"`
}

//go:generate genvalidate UpdateAccessPolicyTemplateRequest

// DeleteAccessPolicyTemplateRequest is to delete an AP Template
// The ID is passed in the URL but we need a specific version, and our handler
// infrastructure doesn't currently support arbitrary non-UUID path components
type DeleteAccessPolicyTemplateRequest struct {
	Version int `json:"version"`
}

// CreateTransformerRequest creates a new GP
type CreateTransformerRequest struct {
	Transformer policy.Transformer `json:"transformer"`
}

//go:generate genvalidate CreateTransformerRequest

// Note: Transformers are immutable for recordkeeping, so there are no updates

// TestTransformerRequest lets you run an unsaved policy for testing
type TestTransformerRequest struct {
	Transformer policy.Transformer `json:"transformer"`
	Data        string             `json:"data"`
}

// Validate implements Validateable
func (t TestTransformerRequest) Validate() error {
	if t.Transformer.Function == "" {
		return ucerr.New("Transformer.Function can't be empty")
	}
	if t.Data == "" {
		return ucerr.New("Data can't be empty")
	}
	return nil
}

// TestTransformerResponse is the response to a TestTransformer call
type TestTransformerResponse struct {
	Value string `json:"value"`
}

// TestAccessPolicyRequest lets you run an unsaved policy with a given context for testing
type TestAccessPolicyRequest struct {
	AccessPolicy policy.AccessPolicy        `json:"access_policy"`
	Context      policy.AccessPolicyContext `json:"context"`
}

// TestAccessPolicyResponse is the response to a TestAccessPolicy call
type TestAccessPolicyResponse struct {
	Allowed bool `json:"allowed"`
}
