package authz

import (
	"github.com/gofrs/uuid"

	"userclouds.com/infra/ucdb"
)

// AuthZ object types & edge types (roles) provisioned for every tenant.
// TODO: merge the string constant with the UUID into a const-ish struct to keep them associated,
// particularly if we add more of these.
// Keep in sync with TSX constants!
// TODO: we should have a better way to sync constants between TS and Go
const (
	ObjectTypeUser  = "_user"
	ObjectTypeGroup = "_group"
)

// UserObjectTypeID is the ID of a built-in object type called "_user"
var UserObjectTypeID = uuid.Must(uuid.FromString("1bf2b775-e521-41d3-8b7e-78e89427e6fe"))

// GroupObjectTypeID is the ID of a built-in object type called "_group"
var GroupObjectTypeID = uuid.Must(uuid.FromString("f5bce640-f866-4464-af1a-9e7474c4a90c"))

// RBACAuthZObjectTypes is an array containing default AuthZ object types
var RBACAuthZObjectTypes = []ObjectType{
	{BaseModel: ucdb.NewBaseWithID(UserObjectTypeID), TypeName: ObjectTypeUser},
	{BaseModel: ucdb.NewBaseWithID(GroupObjectTypeID), TypeName: ObjectTypeGroup},
}
