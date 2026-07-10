package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// These are integration tests, not pure unit tests — the store package is a
// thin SQL wrapper, so there's no meaningful logic to test without a real
// database. They connect to the local Postgres started by
// `docker compose up postgres` (or `task up`) and skip, rather than fail,
// if it isn't reachable, so `go test ./...` still works in environments
// without Docker running.
func testStore(t *testing.T) *PostgresStore {
	t.Helper()

	ctx := context.Background()
	pool, err := Connect(ctx, "postgres://postgres:postgres@localhost:5432/yauli?sslmode=disable")
	if err != nil {
		t.Skipf("skipping: could not connect to local postgres (is `docker compose up postgres` running?): %v", err)
	}
	t.Cleanup(pool.Close)

	return NewPostgresStore(pool)
}

// testEmail returns a unique email per call so tests can run repeatedly
// (and in parallel) without colliding on the `users.email` unique
// constraint.
func testEmail(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test-%s@example.com", uuid.NewString())
}

func TestUpsertUserByEmail_Idempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	email := testEmail(t)
	t.Cleanup(func() { s.pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email) })

	first, err := s.UpsertUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if first.Email != email {
		t.Fatalf("expected email %q, got %q", email, first.Email)
	}

	second, err := s.UpsertUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("upsert again: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected the same user id on a repeat upsert, got %v vs %v", second.ID, first.ID)
	}
}

func TestGetFamilyMembership_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	email := testEmail(t)
	t.Cleanup(func() { s.pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email) })

	user, err := s.UpsertUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	membership, err := s.GetFamilyMembership(ctx, user.ID)
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if membership.Found {
		t.Fatalf("expected no membership for a fresh user, got %+v", membership)
	}
}

func TestCreateFamilyWithOwner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	email := testEmail(t)

	user, err := s.UpsertUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	familyID, err := s.CreateFamilyWithOwner(ctx, user.ID, "test family")
	if err != nil {
		t.Fatalf("create family: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		s.pool.Exec(bg, `DELETE FROM family_members WHERE family_id = $1`, familyID)
		s.pool.Exec(bg, `DELETE FROM families WHERE id = $1`, familyID)
		s.pool.Exec(bg, `DELETE FROM users WHERE id = $1`, user.ID)
	})

	membership, err := s.GetFamilyMembership(ctx, user.ID)
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if !membership.Found {
		t.Fatalf("expected a membership to exist after CreateFamilyWithOwner")
	}
	if membership.FamilyID != familyID {
		t.Fatalf("expected family id %v, got %v", familyID, membership.FamilyID)
	}
	if membership.Role != MembershipRoleOwner {
		t.Fatalf("expected role %q, got %q", MembershipRoleOwner, membership.Role)
	}
	if membership.Status != MembershipStatusActive {
		t.Fatalf("expected status %q, got %q", MembershipStatusActive, membership.Status)
	}
}

func TestActivateInvitedMembership(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	owner, err := s.UpsertUserByEmail(ctx, testEmail(t))
	if err != nil {
		t.Fatalf("upsert owner: %v", err)
	}
	familyID, err := s.CreateFamilyWithOwner(ctx, owner.ID, "test family")
	if err != nil {
		t.Fatalf("create family: %v", err)
	}

	invitee, err := s.UpsertUserByEmail(ctx, testEmail(t))
	if err != nil {
		t.Fatalf("upsert invitee: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		s.pool.Exec(bg, `DELETE FROM family_members WHERE family_id = $1`, familyID)
		s.pool.Exec(bg, `DELETE FROM families WHERE id = $1`, familyID)
		s.pool.Exec(bg, `DELETE FROM users WHERE id = ANY($1)`, []uuid.UUID{owner.ID, invitee.ID})
	})

	// Simulate an invite (PR13 will do this via a real invite endpoint):
	// a pending family_members row for a user who hasn't logged in yet.
	if _, err := s.pool.Exec(ctx, `INSERT INTO family_members (family_id, user_id, role, status) VALUES ($1, $2, $3, $4)`,
		familyID, invitee.ID, MembershipRoleMember, MembershipStatusInvited); err != nil {
		t.Fatalf("insert invited row: %v", err)
	}

	if err := s.ActivateInvitedMembership(ctx, invitee.ID, familyID); err != nil {
		t.Fatalf("activate: %v", err)
	}

	membership, err := s.GetFamilyMembership(ctx, invitee.ID)
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if membership.Status != MembershipStatusActive {
		t.Fatalf("expected status %q after activation, got %q", MembershipStatusActive, membership.Status)
	}
	if membership.Role != MembershipRoleMember {
		t.Fatalf("expected role %q, got %q", MembershipRoleMember, membership.Role)
	}
}

func TestActivateInvitedMembership_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// No invited row exists for these arbitrary, never-inserted ids.
	err := s.ActivateInvitedMembership(ctx, uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
