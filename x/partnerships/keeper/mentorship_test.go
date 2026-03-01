package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// ---------- Mock KnowledgeKeeper ----------

type mockKnowledgeKeeper struct {
	callCount  int
	lastDomain string
	lastMentor string
	lastMentee string
}

func (m *mockKnowledgeKeeper) ApplyMentorshipDividend(_ context.Context, domain, mentor, mentee string) {
	m.callCount++
	m.lastDomain = domain
	m.lastMentor = mentor
	m.lastMentee = mentee
}

// ---------- Tests ----------

func TestGraduateMentorship_CallsKnowledgeDividend(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	mock := &mockKnowledgeKeeper{}
	k.SetKnowledgeKeeper(mock)

	mentorship := &types.Mentorship{
		Id:             "m-1",
		MentorAddr:     "zerone1mentor",
		MenteeAddr:     "zerone1mentee",
		Domain:         "physics",
		Status:         "active",
		StartBlock:     1,
		DurationBlocks: 100,
	}
	k.SetMentorship(ctx, mentorship)

	ctx = ctx.WithBlockHeight(101)
	k.AutoGraduateMentorships(ctx)

	require.Equal(t, 1, mock.callCount)
	require.Equal(t, "physics", mock.lastDomain)
	require.Equal(t, "zerone1mentor", mock.lastMentor)
	require.Equal(t, "zerone1mentee", mock.lastMentee)

	m, found := k.GetMentorship(ctx, "m-1")
	require.True(t, found)
	require.Equal(t, "graduated", m.Status)
}
