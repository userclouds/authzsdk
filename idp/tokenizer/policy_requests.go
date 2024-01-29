package tokenizer

import (
	"userclouds.com/idp/policy"
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

// CreateTransformerRequest creates a new GP
type CreateTransformerRequest struct {
	Transformer policy.Transformer `json:"transformer"`
}

//go:generate genvalidate CreateTransformerRequest

// Note: Transformers are immutable for record keeping, so there are no updates

// TestTransformerRequest lets you run an unsaved policy for testing
type TestTransformerRequest struct {
	Transformer policy.Transformer `json:"transformer"`
	Data        string             `json:"data"`
}

//go:generate genvalidate TestTransformerRequest

// TestTransformerResponse is the response to a TestTransformer call
type TestTransformerResponse struct {
	Value string `json:"value"`
}

// TestAccessPolicyRequest lets you run an unsaved policy with a given context for testing
type TestAccessPolicyRequest struct {
	AccessPolicy policy.AccessPolicy        `json:"access_policy"`
	Context      policy.AccessPolicyContext `json:"context"`
}

//go:generate genvalidate TestAccessPolicyRequest

// TestAccessPolicyTemplateRequest lets you run an unsaved policy template with a given context for testing
type TestAccessPolicyTemplateRequest struct {
	AccessPolicyTemplate policy.AccessPolicyTemplate `json:"access_policy_template"`
	Context              policy.AccessPolicyContext  `json:"context"`
	Params               string                      `json:"params"`
}

//go:generate genvalidate TestAccessPolicyTemplateRequest

// TestAccessPolicyResponse is the response to a TestAccessPolicy call
type TestAccessPolicyResponse struct {
	Allowed bool `json:"allowed"`
}
