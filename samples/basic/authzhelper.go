package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gofrs/uuid"

	"userclouds.com/authz"
	"userclouds.com/idp"
	"userclouds.com/idp/userstore"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/ucerr"
)

// NB: most of the methods in this file should end up in the public SDK in some form, as they're generally useful
// for idempotent creation of objects, types, etc.

// isBenign returns true if there is no error or if the error is safely ignorable (e.g. resource already created)
// during initial setup & provisioning.
func isBenign(err error) bool {
	if err == nil {
		return true
	}
	var clientErr jsonclient.Error
	if errors.As(err, &clientErr) {
		// Resource already exists
		if clientErr.StatusCode == http.StatusConflict {
			return true
		}
	}
	return false
}

func provisionObjectType(ctx context.Context, authZClient *authz.Client, typeName string) (uuid.UUID, error) {
	id, err := authZClient.FindObjectTypeID(ctx, typeName)
	if err != nil {
		id = uuid.Must(uuid.NewV4())
	}
	if _, err := authZClient.CreateObjectType(ctx, id, typeName); !isBenign(err) {
		return uuid.Nil, ucerr.Wrap(err)
	}
	return id, nil
}

func provisionEdgeType(ctx context.Context, authZClient *authz.Client, sourceObjectID, targetObjectID uuid.UUID, typeName string, attributes authz.Attributes) (uuid.UUID, error) {
	id, err := authZClient.FindEdgeTypeID(ctx, typeName)
	if err != nil {
		id = uuid.Must(uuid.NewV4())
	}
	if _, err := authZClient.CreateEdgeType(ctx, id, sourceObjectID, targetObjectID, typeName, attributes); !isBenign(err) {
		return uuid.Nil, ucerr.Wrap(err)
	}
	return id, nil
}

func provisionObject(ctx context.Context, authZClient *authz.Client, typeID uuid.UUID, alias string) (uuid.UUID, error) {
	obj, err := authZClient.CreateObject(ctx, uuid.Must(uuid.NewV4()), typeID, alias)
	if !isBenign(err) {
		return uuid.Nil, ucerr.Wrap(err)
	}
	return obj.ID, nil
}

func provisionUser(ctx context.Context, idpClient *idp.Client, name string) (uuid.UUID, error) {
	// Create a new user
	profile := userstore.Record{}
	profile["name"] = name

	id, err := idpClient.CreateUser(ctx, profile)
	return id, ucerr.Wrap(err)
}

// mustID panics if a UUID-producing operation returns an error, otherwise it returns the UUID
func mustID(id uuid.UUID, err error) uuid.UUID {
	if err != nil {
		log.Fatalf("mustID error: %v", err)
	}
	if id == uuid.Nil {
		log.Fatal("mustID error: unexpected nil uuid")
	}
	return id
}

// mustEdge panics if an edge-producing operation returns an error, otherwise it returns the Edge
func mustEdge(edge *authz.Edge, err error) *authz.Edge {
	if err != nil {
		log.Fatalf("mustEdge error: %v", err)
	}
	if edge.ID == uuid.Nil {
		log.Fatal("mustEdge error: unexpected nil uuid")
	}
	return edge
}
