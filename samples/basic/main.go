package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gofrs/uuid"
	"github.com/joho/godotenv"

	"userclouds.com/authz"
	"userclouds.com/idp"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/ucerr"
)

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

// Object type IDs
var FileTypeID, TeamTypeID uuid.UUID

// Edge type IDs (i.e. roles)
var TeamMemberID uuid.UUID
var FileEditorTypeID, FileViewerTypeID uuid.UUID
var FileTeamEditorTypeID, FileTeamViewerTypeID uuid.UUID

var AliceUserID, BobUserID uuid.UUID

func provisionObjectType(ctx context.Context, authZClient *authz.Client, typeName string) (uuid.UUID, error) {
	if _, err := authZClient.CreateObjectType(ctx, uuid.Must(uuid.NewV4()), typeName); !isBenign(err) {
		return uuid.Nil, ucerr.Wrap(err)
	}
	id, err := authZClient.FindObjectTypeID(ctx, typeName)
	if err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}
	return id, err
}

func provisionEdgeType(ctx context.Context, authZClient *authz.Client, sourceObjectID, targetObjectID uuid.UUID, typeName string) (uuid.UUID, error) {
	if _, err := authZClient.CreateEdgeType(ctx, uuid.Must(uuid.NewV4()), sourceObjectID, targetObjectID, typeName); !isBenign(err) {
		return uuid.Nil, ucerr.Wrap(err)
	}
	id, err := authZClient.FindEdgeTypeID(ctx, typeName)
	if err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}
	return id, err
}

func provisionObject(ctx context.Context, authZClient *authz.Client, typeID uuid.UUID, alias string) (uuid.UUID, error) {
	obj, err := authZClient.CreateObject(ctx, uuid.Must(uuid.NewV4()), typeID, alias)
	if !isBenign(err) {
		return uuid.Nil, ucerr.Wrap(err)
	}
	return obj.ID, nil
}

func provisionUser(ctx context.Context, idpClient *idp.Client, username, password string, profile idp.UserProfile) (uuid.UUID, error) {
	var err error
	if _, err = idpClient.CreateUserWithPassword(ctx, username, password, profile); !isBenign(err) {
		return uuid.Nil, ucerr.Wrap(err)
	}
	users, err := idpClient.ListUsersForEmail(ctx, profile.Email, idp.AuthnTypePassword)
	if err != nil {
		return uuid.Nil, ucerr.Wrap(err)
	}
	if len(users) != 1 {
		return uuid.Nil, ucerr.Errorf("expected 1 user with email '%s', got %d", profile.Email, len(users))
	}
	return users[0].ID, nil
}

func enumerateTeams(ctx context.Context, authZClient *authz.Client, userID uuid.UUID) ([]uuid.UUID, error) {
	edges, err := authZClient.ListEdges(ctx, userID)
	if err != nil {
		return nil, ucerr.Wrap(err)
	}

	teams := []uuid.UUID{}
	for _, v := range edges {
		if v.EdgeTypeID == TeamMemberID {
			teams = append(teams, v.SourceObjectID)
		}
	}
	return teams, nil
}

func ensureID(id uuid.UUID, err error) uuid.UUID {
	if err != nil {
		log.Fatalf("ensureID error: %v", err)
	}
	if id == uuid.Nil {
		log.Fatal("ensureID error: unexpected nil uuid")
	}
	return id
}

func oneTimeProvision(ctx context.Context, idpClient *idp.Client, authZClient *authz.Client) error {
	// NB: these Create commands are only here because this is a self-contained sample app; normally
	// you would do one-time provisioning via the Console or via API ahead of time, not in every single app instance!
	//
	// You can either generate some UUIDs ahead of time and compile them in as constants for each tenant, or query them at runtime.
	FileTypeID = ensureID(provisionObjectType(ctx, authZClient, "file"))
	TeamTypeID = ensureID(provisionObjectType(ctx, authZClient, "team"))

	TeamMemberID = ensureID(provisionEdgeType(ctx, authZClient, TeamTypeID, authz.UserObjectTypeID, "team_member"))
	FileEditorTypeID = ensureID(provisionEdgeType(ctx, authZClient, FileTypeID, authz.UserObjectTypeID, "file_editor"))
	FileViewerTypeID = ensureID(provisionEdgeType(ctx, authZClient, FileTypeID, authz.UserObjectTypeID, "file_viewer"))
	FileTeamEditorTypeID = ensureID(provisionEdgeType(ctx, authZClient, FileTypeID, TeamTypeID, "file_team_editor"))
	FileTeamViewerTypeID = ensureID(provisionEdgeType(ctx, authZClient, FileTypeID, TeamTypeID, "file_team_viewer"))

	// Create a few test users (normally users would sign up via the UI, so this is contrived too)
	AliceUserID = ensureID(provisionUser(ctx, idpClient, "alice", "password_alice_123!", idp.UserProfile{
		Email:    "alice@example.com",
		Name:     "Alice Aardvark",
		Nickname: "Allie",
	}))

	BobUserID = ensureID(provisionUser(ctx, idpClient, "bob", "password_bob_123!", idp.UserProfile{
		Email:    "bob@example.com",
		Name:     "Bob Birdie",
		Nickname: "Bobby",
	}))

	// Clean up all files, folders, etc
	return nil
}

func tryCleanPreviousRuns(ctx context.Context, authZClient *authz.Client) error {
	objs, err := authZClient.ListObjects(ctx)
	if err != nil {
		return ucerr.Wrap(err)
	}
	for _, v := range objs {
		if v.TypeID == FileTypeID || v.TypeID == TeamTypeID {
			if err := authZClient.DeleteObject(ctx, v.ID); err != nil {
				log.Printf("warning, failed to delete %+v", v)
			}
		}
	}
	return nil
}

type FileManager struct {
	authZClient *authz.Client
}

func NewFileManager(authZClient *authz.Client) *FileManager {
	return &FileManager{
		authZClient: authZClient,
	}
}

type File struct {
	id   uuid.UUID
	name string

	isDir bool

	parent   *File
	children []*File
}

func (f File) FullPath() string {
	if f.parent == nil {
		return fmt.Sprintf("%s://", f.id.String())
	} else {
		return fmt.Sprintf("%s/%s", f.parent.FullPath(), f.name)
	}
}

func (f File) FindFile(name string) *File {
	if !f.isDir {
		return nil
	}

	for _, v := range f.children {
		if v.name == name {
			return v
		}
	}
	return nil
}

func (fm *FileManager) HasWriteAccess(ctx context.Context, f *File, userID uuid.UUID) error {
	cur := f
	// NB: with inheritance support, the team enumeration would be implicitly handled by the propagation rules
	teams, err := enumerateTeams(ctx, fm.authZClient, userID)
	if err != nil {
		return ucerr.Wrap(err)
	}

	for cur != nil {
		// NB: currently we only support RBAC without Attributes; instead of `FindEdge`, we should use `CheckAttribute` which enumerates all edges
		// between two objects to see if any edge has the desired attribute. This allows for multiple roles to have overlapping attributes, and for
		// the creation of new roles with new combinations of attributes without touching all permission checking vode.
		// NB: with permission inheritance, there would be additional edges between parent<>children files, and the AuthZ API will run the graph
		// BFS on the backend and propagate permissions/attributes based on the edge definitions. Furthermore, team membership would also inherit permissions.
		// For now, the inheritance is handled client-side.
		if _, err := fm.authZClient.FindEdge(ctx, cur.id, userID, FileEditorTypeID); err == nil {
			return nil
		}
		// NB: with inheritance support, these extra calls won't be needed
		for _, teamID := range teams {
			if _, err := fm.authZClient.FindEdge(ctx, cur.id, teamID, FileTeamEditorTypeID); err == nil {
				return nil
			}
		}
		cur = cur.parent
	}
	return ucerr.Errorf("user %v does not have write permissions on file %+v", userID, f)
}

func (fm *FileManager) HasReadAccess(ctx context.Context, f *File, userID uuid.UUID) error {
	cur := f
	// NB: with inheritance support, the team enumeration would be implicitly handled by the propagation rules
	teams, err := enumerateTeams(ctx, fm.authZClient, userID)
	if err != nil {
		return ucerr.Wrap(err)
	}
	for cur != nil {
		// NB: with Attribute support this will be simpler; we can just call CheckAttribute instead of testing multiple edge types
		if _, err := fm.authZClient.FindEdge(ctx, cur.id, userID, FileEditorTypeID); err == nil {
			return nil
		}
		if _, err := fm.authZClient.FindEdge(ctx, cur.id, userID, FileViewerTypeID); err == nil {
			return nil
		}

		// NB: with inheritance support, these extra calls won't be needed
		for _, teamID := range teams {
			if _, err := fm.authZClient.FindEdge(ctx, cur.id, teamID, FileTeamEditorTypeID); err == nil {
				return nil
			}
			if _, err := fm.authZClient.FindEdge(ctx, cur.id, teamID, FileTeamViewerTypeID); err == nil {
				return nil
			}
		}

		cur = cur.parent
	}
	return ucerr.Errorf("user %v does not have read permissions on file %+v", userID, f)
}

func (fm *FileManager) NewRoot(ctx context.Context, creatorUserID uuid.UUID) (*File, error) {
	f := &File{
		id:       uuid.Must(uuid.NewV4()),
		name:     "",
		isDir:    true,
		parent:   nil,
		children: []*File{},
	}

	// If the first operation fails, nothing is created and the operation fails.
	// If the first succeeds but the second fails, we'll have an orphan authz object to clean-up that is harmless and could be reaped later.
	// NB: We will eventually support Transactions for this, which avoids orphans.
	if _, err := fm.authZClient.CreateObject(ctx, f.id, FileTypeID, f.FullPath()); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// Give the creator of the file Editor permission by default (you could get fancy and have "owner"/"creator"/"admin" access on top too)
	if _, err := fm.authZClient.CreateEdge(ctx, uuid.Must(uuid.NewV4()), f.id, creatorUserID, FileEditorTypeID); err != nil {
		return nil, ucerr.Wrap(err)
	}

	return f, nil
}

func (fm *FileManager) newFileHelper(ctx context.Context, name string, isDir bool, parent *File, creatorUserID uuid.UUID) (*File, error) {
	if !parent.isDir {
		return nil, ucerr.Errorf("cannot create files or directories under a file: %+v", parent)
	}

	if parent.FindFile(name) != nil {
		return nil, ucerr.Errorf("file with name '%s' already exists in %+v", name, parent)
	}

	if err := fm.HasWriteAccess(ctx, parent, creatorUserID); err != nil {
		return nil, ucerr.Wrap(err)
	}

	f := &File{
		id:       uuid.Must(uuid.NewV4()),
		name:     name,
		isDir:    isDir,
		parent:   parent,
		children: []*File{},
	}

	if _, err := fm.authZClient.CreateObject(ctx, f.id, FileTypeID, f.FullPath()); err != nil {
		return nil, ucerr.Wrap(err)
	}

	// NB: since the creator has write access to the parent, we don't need to explicitly grant it on the child

	return f, nil
}

func (fm *FileManager) NewFile(ctx context.Context, name string, parent *File, creatorUserID uuid.UUID) (*File, error) {
	f, err := fm.newFileHelper(ctx, name, false, parent, creatorUserID)
	return f, ucerr.Wrap(err)
}

func (fm *FileManager) NewDir(ctx context.Context, name string, parent *File, creatorUserID uuid.UUID) (*File, error) {
	f, err := fm.newFileHelper(ctx, name, true, parent, creatorUserID)
	return f, ucerr.Wrap(err)
}

func (fm *FileManager) ReadFile(ctx context.Context, f *File, readerUserID uuid.UUID) (string, error) {
	if err := fm.HasReadAccess(ctx, f, readerUserID); err != nil {
		return "", ucerr.Wrap(err)
	}

	return fmt.Sprintf("contents of file %s", f.FullPath()), nil
}

func ensureFile(f *File, err error) *File {
	if err != nil {
		log.Fatalf("ensureFile error: %v", err)
	}
	if f == nil {
		log.Fatal("ensureFile error: unexpected nil file")
	}
	return f
}

func main() {
	ctx := context.Background()

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("error loading .env file: %v", err)
	}

	tenantURL := os.Getenv("TENANT_URL")
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")

	tokenSource := jsonclient.ClientCredentialsTokenSource(tenantURL+"/oidc/token", clientID, clientSecret, nil)

	idpClient, err := idp.NewClient(tenantURL, tokenSource)
	if err != nil {
		log.Fatalf("error initializing idp client: %v", err)
	}

	authZClient, err := authz.NewClient(tenantURL, tokenSource)
	if err != nil {
		log.Fatalf("error initializing authz client: %v", err)
	}

	if err := oneTimeProvision(ctx, idpClient, authZClient); err != nil {
		log.Fatalf("error provisioning basic authz types: %v", err)
	}

	if err := tryCleanPreviousRuns(ctx, authZClient); err != nil {
		log.Printf("failed to clean previous run data, ignoring and moving on")
	}

	fm := NewFileManager(authZClient)

	// Alice creates the root directory and has full permissions
	rootDir := ensureFile(fm.NewRoot(ctx, AliceUserID))
	// Alice can create '/dir1' and '/dir2'
	dir1 := ensureFile(fm.NewDir(ctx, "dir1", rootDir, AliceUserID))
	dir2 := ensureFile(fm.NewDir(ctx, "dir2", rootDir, AliceUserID))

	// Bob cannot create files in '/dir1'
	_, err = fm.NewFile(ctx, "file1", dir1, BobUserID)
	if err == nil {
		log.Fatalf("expected Bob to fail to create file under dir1")
	}

	// Alice can create files in '/dir1'
	file1 := ensureFile(fm.NewFile(ctx, "file1", dir1, AliceUserID))

	// Bob cannot read '/dir1/file1'
	if _, err = fm.ReadFile(ctx, file1, BobUserID); err == nil {
		log.Fatalf("expected Bob to fail to read file1")
	}

	// Grant Bob viewer permissions in 'dir1'
	_ = ensureID(authZClient.CreateEdge(ctx, uuid.Must(uuid.NewV4()), dir1.id, BobUserID, FileViewerTypeID))

	// Now Bob can read '/dir1/file1'
	if _, err := fm.ReadFile(ctx, file1, BobUserID); err != nil {
		log.Fatalf("expected Bob to succeed reading dir1/file1: %v", err)
	}

	// Bob cannot (yet) create subdirectories under '/dir2'
	_, err = fm.NewDir(ctx, "dir3", dir2, BobUserID)
	if err == nil {
		log.Fatalf("expected Bob to fail to create dir3 under /dir2")
	}

	// Create a team, add Bob to it, give that team write permissions to '/dir2'
	aliceReportsTeamID := ensureID(provisionObject(ctx, authZClient, TeamTypeID, "Alice's Direct Reports"))
	_ = ensureID(authZClient.CreateEdge(ctx, uuid.Must(uuid.NewV4()), aliceReportsTeamID, BobUserID, TeamMemberID))
	_ = ensureID(authZClient.CreateEdge(ctx, uuid.Must(uuid.NewV4()), dir2.id, aliceReportsTeamID, FileTeamEditorTypeID))

	// Now Bob can create subdirectories under '/dir2'
	if _, err := fm.NewDir(ctx, "dir3", dir2, BobUserID); err != nil {
		log.Fatalf("expected Bob to succeed creating dir3 under /dir2: %v", err)
	}

	// But still not under '/dir1'
	_, err = fm.NewDir(ctx, "dir3", dir1, BobUserID)
	if err == nil {
		log.Fatalf("expected Bob to fail to create dir3 under /dir1")
	}

	log.Printf("succssfully ran sample")
}
