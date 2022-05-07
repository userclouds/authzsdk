package authz

import "github.com/gofrs/uuid"

// AuthZ object types & edge types (roles) provisioned for every tenant.
// TODO: merge the string constant with the UUID into a const-ish struct to keep them associated,
// particularly if we add more of these.
// Keep in sync with TSX constants!
// TODO: we should have a better way to sync constants between TS and Go
const (
	GroupObjectType = "_group"
	UserObjectType  = "_user"
	AdminRole       = "_admin"
	MemberRole      = "_member"
)

// UserObjectTypeID is the ID of a built-in object type called "_user"
var UserObjectTypeID = uuid.Must(uuid.FromString("1bf2b775-e521-41d3-8b7e-78e89427e6fe"))

// GroupObjectTypeID is the ID of a built-in object type called "_group"
var GroupObjectTypeID = uuid.Must(uuid.FromString("f5bce640-f866-4464-af1a-9e7474c4a90c"))

// AdminRoleTypeID is the ID of a built-in edge type called "_admin"
var AdminRoleTypeID = uuid.Must(uuid.FromString("60b69666-4a8a-4eb3-94dd-621298fb365d"))

// MemberRoleTypeID is the ID of a built-in edge type called "_member"
var MemberRoleTypeID = uuid.Must(uuid.FromString("1eec16ec-6130-4f9e-a51f-21bc19b20d8f"))
