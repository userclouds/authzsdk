package authz

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucerr"
)

// MigrationRequest is the request body for the migration methods
type MigrationRequest struct {
	OrganizationID uuid.UUID `json:"organization_id"`
}

// AddOrganizationToObject adds the specified organization id to the user
func (c *Client) AddOrganizationToObject(ctx context.Context, objectID uuid.UUID, organizationID uuid.UUID) (*Object, error) {

	req := &MigrationRequest{
		OrganizationID: organizationID,
	}

	var resp Object
	if err := c.client.Put(ctx, fmt.Sprintf("/authz/migrate/objects/%s", objectID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveObject(resp)
	return &resp, nil
}

// AddOrganizationToEdgeType adds the specified organization id to the edge type
func (c *Client) AddOrganizationToEdgeType(ctx context.Context, edgeTypeID uuid.UUID, organizationID uuid.UUID) (*EdgeType, error) {

	req := &MigrationRequest{
		OrganizationID: organizationID,
	}

	var resp EdgeType
	if err := c.client.Put(ctx, fmt.Sprintf("/authz/migrate/edgetypes/%s", edgeTypeID), req, &resp); err != nil {
		return nil, ucerr.Wrap(err)
	}

	c.saveEdgeType(resp)
	return &resp, nil
}
