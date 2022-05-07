package main

import (
	"context"
	"log"
	"os"

	"github.com/gofrs/uuid"
	"github.com/joho/godotenv"

	"userclouds.com/authz"
	"userclouds.com/idp"
	"userclouds.com/infra/jsonclient"
	"userclouds.com/infra/ucerr"
)

// Object type IDs
var FileTypeID, TeamTypeID uuid.UUID

// Edge type IDs (i.e. roles)
var TeamMemberID uuid.UUID
var FileEditorTypeID, FileViewerTypeID uuid.UUID
var FileTeamEditorTypeID, FileTeamViewerTypeID uuid.UUID

var AliceUserID, BobUserID uuid.UUID

func oneTimeProvision(ctx context.Context, idpClient *idp.Client, authZClient *authz.Client) error {
	// NB: these Create commands are only here because this is a self-contained sample app; normally
	// you would do one-time provisioning via the Console or via the AuthZ API ahead of time, not in every single app.
	//
	// You can either generate some UUIDs ahead of time and compile them in to your app as constants for each tenant,
	// or query them at runtime by name,
	FileTypeID = mustID(provisionObjectType(ctx, authZClient, "file"))
	TeamTypeID = mustID(provisionObjectType(ctx, authZClient, "team"))

	TeamMemberID = mustID(provisionEdgeType(ctx, authZClient, TeamTypeID, authz.UserObjectTypeID, "team_member"))
	FileEditorTypeID = mustID(provisionEdgeType(ctx, authZClient, FileTypeID, authz.UserObjectTypeID, "file_editor"))
	FileViewerTypeID = mustID(provisionEdgeType(ctx, authZClient, FileTypeID, authz.UserObjectTypeID, "file_viewer"))
	FileTeamEditorTypeID = mustID(provisionEdgeType(ctx, authZClient, FileTypeID, TeamTypeID, "file_team_editor"))
	FileTeamViewerTypeID = mustID(provisionEdgeType(ctx, authZClient, FileTypeID, TeamTypeID, "file_team_viewer"))

	// Create a few test users (normally users would sign up via the UI, so this is slightly contrived)
	AliceUserID = mustID(provisionUser(ctx, idpClient, "alice", "password_alice_123!", idp.UserProfile{
		Email:    "alice@example.com",
		Name:     "Alice Aardvark",
		Nickname: "Allie",
	}))

	BobUserID = mustID(provisionUser(ctx, idpClient, "bob", "password_bob_123!", idp.UserProfile{
		Email:    "bob@example.com",
		Name:     "Bob Birdie",
		Nickname: "Bobby",
	}))

	return nil
}

// cleanPreviousRunData is a convenience method to clean up 'file' and 'team' objects from previous runs
// so that it's easier to inspect the sample's output in the UserClouds Console.
// NB: deleting objects will also delete all edges to/from those objects.
func cleanPreviousRunData(ctx context.Context, authZClient *authz.Client) error {
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

func initClients() (*idp.Client, *authz.Client) {
	ctx := context.Background()

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("error loading .env file: %v\n(did you forget to copy `.env.example` to `.env` and substitute values?)", err)
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

	if err := cleanPreviousRunData(ctx, authZClient); err != nil {
		log.Printf("failed to clean previous run data, ignoring and moving on")
	}
	return idpClient, authZClient
}

func main() {
	ctx := context.Background()
	idpClient, authZClient := initClients()
	fm := NewFileManager(authZClient)

	// Alice creates the root directory and has full permissions
	rootDir := mustFile(fm.NewRoot(ctx, AliceUserID))
	// Alice can create '/dir1' and '/dir2'
	dir1 := mustFile(fm.NewDir(ctx, "dir1", rootDir, AliceUserID))
	dir2 := mustFile(fm.NewDir(ctx, "dir2", rootDir, AliceUserID))

	// Bob cannot create files in '/dir1'
	_, err := fm.NewFile(ctx, "file1", dir1, BobUserID)
	if err == nil {
		log.Fatalf("expected Bob to fail to create file under dir1")
	}

	// Alice can create files in '/dir1'
	file1 := mustFile(fm.NewFile(ctx, "file1", dir1, AliceUserID))

	// Bob cannot read '/dir1/file1'
	if _, err = fm.ReadFile(ctx, file1, BobUserID); err == nil {
		log.Fatalf("expected Bob to fail to read file1")
	}

	// Grant Bob viewer permissions in 'dir1'
	_ = mustID(authZClient.CreateEdge(ctx, uuid.Must(uuid.NewV4()), dir1.id, BobUserID, FileViewerTypeID))

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
	aliceReportsTeamID := mustID(provisionObject(ctx, authZClient, TeamTypeID, "Alice's Direct Reports"))
	_ = mustID(authZClient.CreateEdge(ctx, uuid.Must(uuid.NewV4()), aliceReportsTeamID, BobUserID, TeamMemberID))
	_ = mustID(authZClient.CreateEdge(ctx, uuid.Must(uuid.NewV4()), dir2.id, aliceReportsTeamID, FileTeamEditorTypeID))

	// Now Bob can create subdirectories under '/dir2'
	if _, err := fm.NewDir(ctx, "dir3", dir2, BobUserID); err != nil {
		log.Fatalf("expected Bob to succeed creating dir3 under /dir2: %v", err)
	}

	// But still not under '/dir1'
	_, err = fm.NewDir(ctx, "dir3", dir1, BobUserID)
	if err == nil {
		log.Fatalf("expected Bob to fail to create dir3 under /dir1")
	}

	renderFileTree(ctx, idpClient, authZClient, rootDir, 0)

	log.Printf("succssfully ran sample")
}
