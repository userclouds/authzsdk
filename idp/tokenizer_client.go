package idp

import (
	"context"
	"net/url"
	"strconv"

	"github.com/gofrs/uuid"

	"userclouds.com/idp/paths"
	"userclouds.com/idp/policy"
	"userclouds.com/idp/tokenizer"
	"userclouds.com/idp/userstore"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/pagination"
	"userclouds.com/infra/ucerr"
)

// TokenizerClient defines a tokenizer client
type TokenizerClient struct {
	client  *jsonclient.Client
	options options
}

// NewTokenizerClient creates a new tokenizer client
func NewTokenizerClient(url string, opts ...Option) *TokenizerClient {
	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	return &TokenizerClient{client: jsonclient.New(url, options.jsonclientOptions...), options: options}
}

// CreateToken creates a token
func (c *TokenizerClient) CreateToken(ctx context.Context, data string, transformerRID, accessPolicyRID userstore.ResourceID) (string, error) {
	req := tokenizer.CreateTokenRequest{
		Data:            data,
		TransformerRID:  transformerRID,
		AccessPolicyRID: accessPolicyRID,
	}
	if err := req.Validate(); err != nil {
		return "", ucerr.Wrap(err)
	}

	var res tokenizer.CreateTokenResponse
	if err := c.client.Post(ctx, paths.CreateToken, req, &res); err != nil {
		return "", ucerr.Wrap(err)
	}

	return res.Token, nil
}

// ResolveToken resolves a token
func (c *TokenizerClient) ResolveToken(ctx context.Context, token string, resolutionContext policy.ClientContext, purposes []userstore.ResourceID) (string, error) {
	res, err := c.ResolveTokens(ctx, []string{token}, resolutionContext, purposes)
	if err != nil {
		return "", ucerr.Wrap(err)
	}

	return res[0], nil
}

// ResolveTokens resolves tokens
func (c *TokenizerClient) ResolveTokens(ctx context.Context, tokens []string, resolutionContext policy.ClientContext, purposes []userstore.ResourceID) ([]string, error) {
	req := tokenizer.ResolveTokensRequest{
		Tokens:   tokens,
		Context:  resolutionContext,
		Purposes: purposes,
	}
	if err := req.Validate(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var res []tokenizer.ResolveTokenResponse
	if err := c.client.Post(ctx, paths.ResolveToken, req, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	if len(tokens) != len(res) {
		return nil, ucerr.New("Server returned partial response")
	}

	v := make([]string, len(res))
	for i := range res {
		v[i] = res[i].Data
	}
	return v, nil
}

// InspectToken helps with debugging
func (c *TokenizerClient) InspectToken(ctx context.Context, token string) (*tokenizer.InspectTokenResponse, error) {
	req := tokenizer.InspectTokenRequest{
		Token: token,
	}
	if err := req.Validate(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var res tokenizer.InspectTokenResponse
	if err := c.client.Post(ctx, paths.InspectToken, req, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// LookupTokens checks to see if one or more tokens exists already for given data
func (c *TokenizerClient) LookupTokens(ctx context.Context, data string, transformerRID, accessPolicyRID userstore.ResourceID) ([]string, error) {
	req := tokenizer.LookupTokensRequest{
		Data:            data,
		TransformerRID:  transformerRID,
		AccessPolicyRID: accessPolicyRID,
	}
	if err := req.Validate(); err != nil {
		return nil, ucerr.Wrap(err)
	}

	var res tokenizer.LookupTokensResponse
	if err := c.client.Post(ctx, paths.LookupToken, req, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return res.Tokens, nil
}

// DeleteToken deletes a token
func (c *TokenizerClient) DeleteToken(ctx context.Context, token string) error {

	requestURL := url.URL{
		Path: paths.DeleteToken,
	}

	requestURL.RawQuery = url.Values{
		"token": []string{token},
	}.Encode()

	if err := c.client.Delete(ctx, requestURL.String(), nil); err != nil {
		return ucerr.Wrap(err)
	}

	return nil
}

// TestAccessPolicy tests an access policy without saving it
func (c *TokenizerClient) TestAccessPolicy(ctx context.Context, accessPolicy policy.AccessPolicy, context policy.AccessPolicyContext) (bool, error) {
	req := tokenizer.TestAccessPolicyRequest{
		AccessPolicy: accessPolicy,
		Context:      context,
	}

	var res tokenizer.TestAccessPolicyResponse
	if err := c.client.Post(ctx, paths.TestAccessPolicy, req, &res); err != nil {
		return false, ucerr.Wrap(err)
	}

	return res.Allowed, nil
}

// TestTransformer tests an access policy without saving it
func (c *TokenizerClient) TestTransformer(ctx context.Context, data string, transformer policy.Transformer) (string, error) {
	req := tokenizer.TestTransformerRequest{
		Transformer: transformer,
		Data:        data,
	}
	if err := req.Validate(); err != nil {
		return "", ucerr.Wrap(err)
	}

	var res tokenizer.TestTransformerResponse
	if err := c.client.Post(ctx, paths.TestTransformer, req, &res); err != nil {
		return "", ucerr.Wrap(err)
	}

	return res.Value, nil
}

// ListAccessPoliciesResponse is the paginated response from listing object types.
type ListAccessPoliciesResponse struct {
	Data []policy.AccessPolicy `json:"data"`
	pagination.ResponseFields
}

// ListAccessPolicies lists access policies
func (c *TokenizerClient) ListAccessPolicies(ctx context.Context, versioned bool, opts ...Option) (*ListAccessPoliciesResponse, error) {

	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	var res ListAccessPoliciesResponse

	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()
	query.Add("versioned", strconv.FormatBool(versioned))

	url := url.URL{
		Path:     paths.ListAccessPolicies,
		RawQuery: query.Encode()}
	path := url.String()

	if err := c.client.Get(ctx, path, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// GetAccessPolicy gets a single access policy by ID
func (c *TokenizerClient) GetAccessPolicy(ctx context.Context, accessPolicyRID userstore.ResourceID) (*policy.AccessPolicy, error) {
	var res policy.AccessPolicy
	if accessPolicyRID.ID != uuid.Nil {
		if err := c.client.Get(ctx, paths.GetAccessPolicy(accessPolicyRID.ID), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
		if accessPolicyRID.Name != "" && res.Name != accessPolicyRID.Name {
			return nil, ucerr.Errorf("Access policy name mismatch: %s != %s", res.Name, accessPolicyRID.Name)
		}
	} else {
		if err := c.client.Get(ctx, paths.GetAccessPolicyByName(accessPolicyRID.Name), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &res, nil
}

// GetAccessPolicyByVersion gets a single access policy by ID and version
func (c *TokenizerClient) GetAccessPolicyByVersion(ctx context.Context, accessPolicyRID userstore.ResourceID, version int) (*policy.AccessPolicy, error) {
	var res policy.AccessPolicy
	if accessPolicyRID.ID != uuid.Nil {
		if err := c.client.Get(ctx, paths.GetAccessPolicyByVersion(accessPolicyRID.ID, version), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
		if accessPolicyRID.Name != "" && res.Name != accessPolicyRID.Name {
			return nil, ucerr.Errorf("Access policy name mismatch: %s != %s", res.Name, accessPolicyRID.Name)
		}
	} else {
		if err := c.client.Get(ctx, paths.GetAccessPolicyByNameAndVersion(accessPolicyRID.Name, version), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &res, nil
}

// CreateAccessPolicy creates an access policy
func (c *TokenizerClient) CreateAccessPolicy(ctx context.Context, ap policy.AccessPolicy, opts ...Option) (*policy.AccessPolicy, error) {

	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := tokenizer.CreateAccessPolicyRequest{
		AccessPolicy: ap,
	}

	var res policy.AccessPolicy
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateAccessPolicy, req, &res)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			res = ap
			res.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateAccessPolicy, req, &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &res, nil
}

// UpdateAccessPolicy updates an access policy
func (c *TokenizerClient) UpdateAccessPolicy(ctx context.Context, ap policy.AccessPolicy) (*policy.AccessPolicy, error) {
	req := tokenizer.UpdateAccessPolicyRequest{
		AccessPolicy: ap,
	}

	var resp policy.AccessPolicy
	if err := c.client.Put(ctx, paths.UpdateAccessPolicy(ap.ID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// DeleteAccessPolicy deletes an access policy
func (c *TokenizerClient) DeleteAccessPolicy(ctx context.Context, id uuid.UUID, version int) error {
	req := tokenizer.DeleteAccessPolicyRequest{
		Version: version,
	}

	if err := c.client.Delete(ctx, paths.DeleteAccessPolicy(id), req); err != nil {
		return ucerr.Wrap(err)
	}

	return nil
}

// ListAccessPolicyTemplatesResponse is the paginated response from listing object types.
type ListAccessPolicyTemplatesResponse struct {
	Data []policy.AccessPolicyTemplate `json:"data"`
	pagination.ResponseFields
}

// ListAccessPolicyTemplates lists access policies
func (c *TokenizerClient) ListAccessPolicyTemplates(ctx context.Context, versioned bool, opts ...Option) (*ListAccessPolicyTemplatesResponse, error) {
	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	var res ListAccessPolicyTemplatesResponse

	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	query := pager.Query()
	query.Add("versioned", strconv.FormatBool(versioned))

	url := url.URL{
		Path:     paths.ListAccessPolicyTemplates,
		RawQuery: query.Encode()}
	path := url.String()

	if err := c.client.Get(ctx, path, &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// GetAccessPolicyTemplate gets a single access policy by ID
func (c *TokenizerClient) GetAccessPolicyTemplate(ctx context.Context, accessPolicyTemplateRID userstore.ResourceID) (*policy.AccessPolicyTemplate, error) {
	var res policy.AccessPolicyTemplate
	if accessPolicyTemplateRID.ID != uuid.Nil {
		if err := c.client.Get(ctx, paths.GetAccessPolicyTemplate(accessPolicyTemplateRID.ID), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
		if accessPolicyTemplateRID.Name != "" && res.Name != accessPolicyTemplateRID.Name {
			return nil, ucerr.Errorf("Access policy template name mismatch: %s != %s", res.Name, accessPolicyTemplateRID.Name)
		}
	} else {
		if err := c.client.Get(ctx, paths.GetAccessPolicyTemplateByName(accessPolicyTemplateRID.Name), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &res, nil
}

// GetAccessPolicyTemplateByVersion gets a single access policy by ID and version
func (c *TokenizerClient) GetAccessPolicyTemplateByVersion(ctx context.Context, accessPolicyTemplateRID userstore.ResourceID, version int) (*policy.AccessPolicyTemplate, error) {
	var res policy.AccessPolicyTemplate
	if accessPolicyTemplateRID.ID != uuid.Nil {
		if err := c.client.Get(ctx, paths.GetAccessPolicyTemplateByVersion(accessPolicyTemplateRID.ID, version), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
		if accessPolicyTemplateRID.Name != "" && res.Name != accessPolicyTemplateRID.Name {
			return nil, ucerr.Errorf("Access policy template name mismatch: %s != %s", res.Name, accessPolicyTemplateRID.Name)
		}
	} else {
		if err := c.client.Get(ctx, paths.GetAccessPolicyByNameAndVersion(accessPolicyTemplateRID.Name, version), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &res, nil
}

// CreateAccessPolicyTemplate creates an access policy
func (c *TokenizerClient) CreateAccessPolicyTemplate(ctx context.Context, apt policy.AccessPolicyTemplate, opts ...Option) (*policy.AccessPolicyTemplate, error) {

	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := tokenizer.CreateAccessPolicyTemplateRequest{
		AccessPolicyTemplate: apt,
	}

	var res policy.AccessPolicyTemplate
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateAccessPolicyTemplate, req, &res)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			res = apt
			res.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateAccessPolicyTemplate, req, &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &res, nil
}

// UpdateAccessPolicyTemplate updates an access policy
func (c *TokenizerClient) UpdateAccessPolicyTemplate(ctx context.Context, apt policy.AccessPolicyTemplate) (*policy.AccessPolicyTemplate, error) {
	req := tokenizer.UpdateAccessPolicyTemplateRequest{
		AccessPolicyTemplate: apt,
	}

	var resp policy.AccessPolicyTemplate
	if err := c.client.Put(ctx, paths.UpdateAccessPolicyTemplate(apt.ID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &resp, nil
}

// DeleteAccessPolicyTemplate deletes an access policy
func (c *TokenizerClient) DeleteAccessPolicyTemplate(ctx context.Context, id uuid.UUID, version int) error {
	if err := c.client.Delete(ctx, paths.DeleteAccessPolicyTemplate(id), nil); err != nil {
		return ucerr.Wrap(err)
	}

	return nil
}

// ListTransformersResponse is the paginated response from listing transformers
type ListTransformersResponse struct {
	Data []policy.Transformer `json:"data"`
	pagination.ResponseFields
}

// ListTransformers lists transformers
func (c *TokenizerClient) ListTransformers(ctx context.Context, opts ...Option) (*ListTransformersResponse, error) {
	options := c.options
	for _, opt := range opts {
		opt.apply(&options)
	}

	var res ListTransformersResponse

	pager, err := pagination.ApplyOptions(options.paginationOptions...)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	url := url.URL{
		Path:     paths.ListTransformers,
		RawQuery: pager.Query().Encode()}

	if err := c.client.Get(ctx, url.String(), &res); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return &res, nil
}

// CreateTransformer creates a transformer
func (c *TokenizerClient) CreateTransformer(ctx context.Context, tp policy.Transformer, opts ...Option) (*policy.Transformer, error) {

	var options options
	for _, opt := range opts {
		opt.apply(&options)
	}

	req := tokenizer.CreateTransformerRequest{
		Transformer: tp,
	}

	var resp policy.Transformer
	if options.ifNotExists {
		exists, existingID, err := c.client.CreateIfNotExists(ctx, paths.CreateTransformer, req, &resp)
		if err != nil {
			return nil, ucerr.Wrap(err)
		}
		if exists {
			resp = tp
			resp.ID = existingID
		}
	} else {
		if err := c.client.Post(ctx, paths.CreateTransformer, req, &resp); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &resp, nil
}

// GetTransformer gets a single transformer by ID
func (c *TokenizerClient) GetTransformer(ctx context.Context, transformerRID userstore.ResourceID) (*policy.Transformer, error) {
	var res policy.Transformer
	if transformerRID.ID != uuid.Nil {
		if err := c.client.Get(ctx, paths.GetTransformer(transformerRID.ID), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
		if transformerRID.Name != "" && res.Name != transformerRID.Name {
			return nil, ucerr.Errorf("Transformer name mismatch: %s != %s", res.Name, transformerRID.Name)
		}
	} else {
		if err := c.client.Get(ctx, paths.GetTransformerByName(transformerRID.Name), &res); err != nil {
			return nil, ucerr.Wrap(err)
		}
	}

	return &res, nil
}

// DeleteTransformer deletes a transformer
func (c *TokenizerClient) DeleteTransformer(ctx context.Context, id uuid.UUID) error {
	if err := c.client.Delete(ctx, paths.DeleteTransformer(id), nil); err != nil {
		return ucerr.Wrap(err)
	}

	return nil
}
