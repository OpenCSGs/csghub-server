package workhub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// TestRebacTupleJobClientForwardsTransaction verifies one batch uses the exact transaction.
func TestRebacTupleJobClientForwardsTransaction(t *testing.T) {
	fake := &fakeJobClient{}
	tx := &sql.Tx{}
	event := types.NewRebacTuple("test", types.TupleOperation{Relationships: []rebac.Relationship{{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)}}})
	require.NoError(t, NewRebacTupleJobClient(fake).InsertRebacTupleJobsTx(context.Background(), tx, []types.RebacTuple{event, event}))
	require.Same(t, tx, fake.tx)
	require.Equal(t, 1, fake.batchCalls)
	require.Len(t, fake.batch, 2)
	args := fake.batch[0].Args.(RebacTupleArgs)
	require.Equal(t, "user:u", args.SubjectKey)
	require.Equal(t, event, args.RebacTuple)
	require.Equal(t, RebacTupleQueue, fake.opts.Queue)
	require.Equal(t, 12, fake.opts.MaxAttempts)
	payload, err := json.Marshal(args)
	require.NoError(t, err)
	var decoded RebacTupleArgs
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, args, decoded)
}

// TestRebacTupleJobClientRequiresTransaction rejects non-atomic enqueue attempts.
func TestRebacTupleJobClientRequiresTransaction(t *testing.T) {
	fake := &fakeJobClient{}
	err := NewRebacTupleJobClient(fake).InsertRebacTupleJobsTx(context.Background(), nil, []types.RebacTuple{{}})
	require.ErrorContains(t, err, "business transaction")
	require.False(t, fake.called)
	require.NoError(t, NewRebacTupleJobClient(nil).InsertRebacTupleJobsTx(context.Background(), nil, nil))
}

// TestRebacTupleJobClientRelationshipLimit checks chunk boundaries and delete/write order.
func TestRebacTupleJobClientRelationshipLimit(t *testing.T) {
	for _, counts := range [][2]int{{0, 49}, {50, 0}, {0, 51}, {25, 25}, {25, 26}, {1000, 0}} {
		var operations []types.TupleOperation
		for i, count := range counts {
			if count == 0 {
				continue
			}
			var relationships []rebac.Relationship
			for j := 0; j < count; j++ {
				relationships = append(relationships, rebac.Relationship{Subject: rebac.UserSubject("u"), Relation: rebac.RelationReader, Object: rebac.RepositoryObject(int64(j + 1))})
			}
			operations = append(operations, types.TupleOperation{Delete: i == 0, Relationships: relationships})
		}
		input := types.NewRebacTuple("test", operations...)
		fake := &fakeJobClient{}
		require.NoError(t, NewRebacTupleJobClient(fake).InsertRebacTupleJobsTx(context.Background(), &sql.Tx{}, []types.RebacTuple{input}))
		require.Equal(t, 1, fake.batchCalls)
		total := counts[0] + counts[1]
		require.Len(t, fake.batch, (total+49)/50)
		var got []types.TupleOperation
		for _, param := range fake.batch {
			args := param.Args.(RebacTupleArgs)
			require.NoError(t, ValidateRebacTupleJob(args))
			for _, operation := range args.Operations {
				for _, relationship := range operation.Relationships {
					got = append(got, types.TupleOperation{Delete: operation.Delete, Relationships: []rebac.Relationship{relationship}})
				}
			}
		}
		var want []types.TupleOperation
		for _, operation := range input.Operations {
			for _, relationship := range operation.Relationships {
				want = append(want, types.TupleOperation{Delete: operation.Delete, Relationships: []rebac.Relationship{relationship}})
			}
		}
		require.Equal(t, want, got)
	}
}

// TestRebacTupleSubjectGrouping keeps usersets together and distinct types separate.
func TestRebacTupleSubjectGrouping(t *testing.T) {
	input := types.NewRebacTuple("group", types.TupleOperation{Relationships: []rebac.Relationship{
		{Subject: rebac.UserSubject("same"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)},
		{Subject: rebac.OrganizationMembers("same"), Relation: rebac.RelationReader, Object: rebac.RepositoryObject(1)},
		{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, "same"), Relation: rebac.RelationOrganization, Object: rebac.RepositoryObject(1)},
	}})
	jobs := splitRebacTuple(input)
	require.Len(t, jobs, 2)
	require.Equal(t, "user:same", jobs[0].SubjectKey)
	require.Equal(t, "organization:same", jobs[1].SubjectKey)
	require.Len(t, jobs[1].Operations[0].Relationships, 2)
	for _, job := range jobs {
		require.NoError(t, ValidateRebacTupleJob(job))
	}
	jobs[0].SubjectKey = "user:other"
	require.ErrorContains(t, ValidateRebacTupleJob(jobs[0]), "sequence subject")
	opts := (RebacTupleArgs{}).SequenceOpts()
	require.True(t, opts.ByArgs)
	require.False(t, opts.ExcludeKind)
	require.False(t, opts.ByQueue)
	require.True(t, *opts.ContinueOnCancelled)
	require.True(t, *opts.ContinueOnDiscarded)
}

// TestRebacTupleWildcardGrouping isolates objects while retaining visibility operation order.
func TestRebacTupleWildcardGrouping(t *testing.T) {
	first := rebac.Relationship{Subject: rebac.PublicUserWildcard(), Relation: rebac.RelationPublic, Object: rebac.KnowledgeBaseObject(1)}
	second := first
	second.Object = rebac.KnowledgeBaseObject(2)
	input := types.NewRebacTuple("wildcard", types.TupleOperation{Relationships: []rebac.Relationship{first, second}},
		types.TupleOperation{Delete: true, Relationships: []rebac.Relationship{first, second}},
		types.TupleOperation{Relationships: []rebac.Relationship{first}})
	fake := &fakeJobClient{}
	require.NoError(t, NewRebacTupleJobClient(fake).InsertRebacTupleJobsTx(context.Background(), &sql.Tx{}, []types.RebacTuple{input}))
	require.Equal(t, 1, fake.batchCalls)
	require.Len(t, fake.batch, 2)
	firstJob := fake.batch[0].Args.(RebacTupleArgs)
	secondJob := fake.batch[1].Args.(RebacTupleArgs)
	require.Equal(t, "user:*|"+first.Object.String(), firstJob.SubjectKey)
	require.NotEqual(t, firstJob.SubjectKey, secondJob.SubjectKey)
	require.Equal(t, []types.TupleOperation{
		{Relationships: []rebac.Relationship{first}},
		{Delete: true, Relationships: []rebac.Relationship{first}},
		{Relationships: []rebac.Relationship{first}},
	}, firstJob.Operations)
	require.Equal(t, []types.TupleOperation{
		{Relationships: []rebac.Relationship{second}},
		{Delete: true, Relationships: []rebac.Relationship{second}},
	}, secondJob.Operations)
	require.NoError(t, ValidateRebacTupleJob(firstJob))
	require.NoError(t, ValidateRebacTupleJob(secondJob))
	firstJob.Operations[0].Relationships = append(firstJob.Operations[0].Relationships, second)
	require.ErrorContains(t, ValidateRebacTupleJob(firstJob), "sequence subject")
	second.Object = rebac.RepositoryObject(1)
	require.NotEqual(t, RebacSequenceKey(first), RebacSequenceKey(second), "object types must remain isolated")
	first.Subject, second.Subject = rebac.UserSubject("u"), rebac.UserSubject("u")
	require.Equal(t, RebacSequenceKey(first), RebacSequenceKey(second), "concrete subjects must retain one sequence across objects")
}

// TestRebacTupleJobClientModelID accepts future IDs and validates the entire batch first.
func TestRebacTupleJobClientModelID(t *testing.T) {
	for _, modelID := range []string{"", types.OpenFgaAuthorizationModelIDVer1_1, "future-model-1.3"} {
		fake := &fakeJobClient{}
		event := types.NewRebacTuple("test", types.TupleOperation{Relationships: []rebac.Relationship{{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)}}})
		valid := event
		event.ModelID = modelID
		err := NewRebacTupleJobClient(fake).InsertRebacTupleJobsTx(context.Background(), &sql.Tx{}, []types.RebacTuple{valid, event})
		if modelID == "" {
			require.ErrorContains(t, err, "model ID is required")
			require.False(t, fake.called)
		} else {
			require.NoError(t, err)
		}
	}
	fake := &fakeJobClient{err: errors.New("river unavailable")}
	require.ErrorContains(t, NewRebacTupleJobClient(fake).InsertRebacTupleJobsTx(context.Background(), &sql.Tx{}, []types.RebacTuple{types.NewRebacTuple("test", types.TupleOperation{Relationships: []rebac.Relationship{{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)}}})}), "river unavailable")
}

// TestNewRebacTupleJobClientWithDBAllowsUninitializedDatabase preserves mock construction.
func TestNewRebacTupleJobClientWithDBAllowsUninitializedDatabase(t *testing.T) {
	for _, db := range []*database.DB{nil, {}} {
		client, err := NewRebacTupleJobClientWithDB(db)
		require.NoError(t, err)
		require.Nil(t, client)
	}
}
