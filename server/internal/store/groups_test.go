package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/store"
)

func TestGroupCRUD(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	g, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{
		Slug: "platform", Name: "Platform Team", Description: "owns infra",
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	got, err := sharedDB.GetGroupBySlug(ctx, "platform")
	if err != nil || got.ID != g.ID {
		t.Fatalf("GetGroupBySlug: %v (%+v)", err, got)
	}
	if got.Description != "owns infra" {
		t.Errorf("description = %q, want 'owns infra'", got.Description)
	}

	updated, err := sharedDB.UpdateGroup(ctx, g.ID, store.UpdateGroupParams{
		Name: "Platform", Description: "",
	})
	if err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	if updated.Name != "Platform" || updated.Description != "" {
		t.Errorf("after update = %+v, want name=Platform, empty description", updated)
	}

	list, err := sharedDB.ListGroups(ctx, store.ListGroupsParams{})
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListGroups len = %d, want 1", len(list))
	}

	if err := sharedDB.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if _, err := sharedDB.GetGroupBySlug(ctx, "platform"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
}

func TestCreateGroup_DuplicateSlugConflict(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if _, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "dup", Name: "A"}); err != nil {
		t.Fatalf("first CreateGroup: %v", err)
	}
	if _, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "dup", Name: "B"}); !errors.Is(err, store.ErrConflict) {
		t.Errorf("duplicate slug err = %v, want ErrConflict", err)
	}
}

func TestEnsureGroupBySlug_Idempotent(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a, err := sharedDB.EnsureGroupBySlug(ctx, "reviewers", "Reviewers")
	if err != nil {
		t.Fatalf("first EnsureGroupBySlug: %v", err)
	}
	b, err := sharedDB.EnsureGroupBySlug(ctx, "reviewers", "Different Name")
	if err != nil {
		t.Fatalf("second EnsureGroupBySlug: %v", err)
	}
	if a.ID != b.ID {
		t.Errorf("EnsureGroupBySlug returned different ids %q vs %q", a.ID, b.ID)
	}
	// Name is only applied on creation; the existing row keeps its name.
	if b.Name != "Reviewers" {
		t.Errorf("name = %q, want unchanged 'Reviewers'", b.Name)
	}
}

func TestGroupMembers(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	g, err := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "team", Name: "Team"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	u1, _ := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "a@x.test"})
	u2, _ := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "b@x.test"})

	if err := sharedDB.AddGroupMember(ctx, g.ID, u1.ID); err != nil {
		t.Fatalf("AddGroupMember u1: %v", err)
	}
	if err := sharedDB.AddGroupMember(ctx, g.ID, u2.ID); err != nil {
		t.Fatalf("AddGroupMember u2: %v", err)
	}
	// Idempotent — adding the same member twice is not an error.
	if err := sharedDB.AddGroupMember(ctx, g.ID, u1.ID); err != nil {
		t.Fatalf("AddGroupMember duplicate: %v", err)
	}

	members, err := sharedDB.ListGroupMembers(ctx, g.ID)
	if err != nil {
		t.Fatalf("ListGroupMembers: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("members = %d, want 2", len(members))
	}

	if err := sharedDB.RemoveGroupMember(ctx, g.ID, u1.ID); err != nil {
		t.Fatalf("RemoveGroupMember: %v", err)
	}
	if err := sharedDB.RemoveGroupMember(ctx, g.ID, u1.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("removing a non-member err = %v, want ErrNotFound", err)
	}
}

func TestAddGroupMember_UnknownPrincipal(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	g, _ := sharedDB.CreateGroup(ctx, store.CreateGroupParams{Slug: "team", Name: "Team"})
	if err := sharedDB.AddGroupMember(ctx, g.ID, "no-such-user"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("adding unknown user err = %v, want ErrNotFound", err)
	}
}
