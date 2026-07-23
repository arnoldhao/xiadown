package library

import (
	"strings"
	"time"
)

type ListenLocalMusicMembershipState string

const (
	ListenLocalMusicMembershipIncluded ListenLocalMusicMembershipState = "included"
	ListenLocalMusicMembershipExcluded ListenLocalMusicMembershipState = "excluded"
)

type ListenLocalMusicMembership struct {
	FileID    string
	State     ListenLocalMusicMembershipState
	Reason    string
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListenLocalMusicMembershipParams struct {
	FileID    string
	State     string
	Reason    string
	Revision  int64
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

func NewListenLocalMusicMembership(params ListenLocalMusicMembershipParams) (ListenLocalMusicMembership, error) {
	fileID := strings.TrimSpace(params.FileID)
	state := ListenLocalMusicMembershipState(strings.ToLower(strings.TrimSpace(params.State)))
	reason := strings.ToLower(strings.TrimSpace(params.Reason))
	revision := params.Revision
	if revision == 0 {
		revision = 1
	}
	if fileID == "" || revision < 1 {
		return ListenLocalMusicMembership{}, ErrInvalidListenLocalMusicMembership
	}
	switch state {
	case ListenLocalMusicMembershipIncluded, ListenLocalMusicMembershipExcluded:
	default:
		return ListenLocalMusicMembership{}, ErrInvalidListenLocalMusicMembership
	}
	switch reason {
	case "", "user", "unsupported", "policy":
	default:
		return ListenLocalMusicMembership{}, ErrInvalidListenLocalMusicMembership
	}
	now := time.Now().UTC()
	createdAt := now
	if params.CreatedAt != nil && !params.CreatedAt.IsZero() {
		createdAt = params.CreatedAt.UTC()
	}
	updatedAt := createdAt
	if params.UpdatedAt != nil && !params.UpdatedAt.IsZero() {
		updatedAt = params.UpdatedAt.UTC()
	}
	return ListenLocalMusicMembership{
		FileID: fileID, State: state, Reason: reason, Revision: revision,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func (item ListenLocalMusicMembership) IsUserExcluded() bool {
	return item.State == ListenLocalMusicMembershipExcluded && item.Reason == "user"
}

func (item ListenLocalMusicMembership) IsExcluded() bool {
	return item.State == ListenLocalMusicMembershipExcluded
}
