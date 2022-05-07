package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gofrs/uuid"
	"userclouds.com/authz"
	"userclouds.com/idp"
	"userclouds.com/infra/ucerr"
)

// FileManager is a simple class that can create file systems with permissions.
type FileManager struct {
	authZClient *authz.Client
}

func NewFileManager(authZClient *authz.Client) *FileManager {
	return &FileManager{
		authZClient: authZClient,
	}
}

// File represents a file/directory that is part of a tree. Only directories can have child files.
// The ID of the file is used as a key in the permission system to manage ACLs.
type File struct {
	id   uuid.UUID
	name string

	isDir bool

	parent   *File
	children []*File
}

func (f File) FullPath() string {
	if f.parent == nil {
		return fmt.Sprintf("file://%s", f.id.String())
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

	parent.children = append(parent.children, f)

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

// mustFile panics if a file-producing operation returns an error, otherwise it returns the file
func mustFile(f *File, err error) *File {
	if err != nil {
		log.Fatalf("mustFile error: %v", err)
	}
	if f == nil {
		log.Fatal("mustFile error: unexpected nil file")
	}
	return f
}

const leftColWidth = 50

func summarizePermissions(ctx context.Context, idpClient *idp.Client, authZClient *authz.Client, f *File) string {
	edges, err := authZClient.ListEdges(ctx, f.id)
	if err != nil {
		return "<error fetching edges>"
	}
	permsList := ""
	for _, e := range edges {
		et, err := authZClient.GetEdgeType(ctx, e.EdgeTypeID)
		if err != nil {
			return "<error fetching edge type>"
		}
		var otherID uuid.UUID
		if e.SourceObjectID == f.id {
			otherID = e.TargetObjectID
		} else {
			otherID = e.SourceObjectID
		}
		obj, err := authZClient.GetObject(ctx, otherID)
		if err != nil {
			return "<error fetching object>"
		}
		displayName := obj.Alias
		if obj.TypeID == authz.UserObjectTypeID {
			user, err := idpClient.GetUser(ctx, obj.ID)
			if err != nil {
				return "<error fetching user>"
			}
			displayName = user.Name
		}
		perm := fmt.Sprintf("%s (%s)", displayName, et.TypeName)
		if len(permsList) == 0 {
			permsList = perm
		} else {
			permsList = fmt.Sprintf("%s, %s", permsList, perm)
		}
	}
	return permsList
}

func renderFileTree(ctx context.Context, idpClient *idp.Client, authZClient *authz.Client, f *File, indentLevel int) {
	outStr := ""
	if f.parent == nil {
		outStr += "/"
	} else {
		for i := 0; i < indentLevel-1; i++ {
			outStr += "      "
		}
		outStr += "^---> "
		outStr += f.name
	}

	for i := len(outStr); i < leftColWidth; i++ {
		outStr += " "
	}
	outStr += fmt.Sprintf("| %s", summarizePermissions(ctx, idpClient, authZClient, f))
	fmt.Println(outStr)

	for _, v := range f.children {
		renderFileTree(ctx, idpClient, authZClient, v, indentLevel+1)
	}
}
