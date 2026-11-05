package mq

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type fakeXnetStream struct {
	info        *jetstream.StreamInfo
	messages    map[uint64]*jetstream.RawStreamMsg
	getErr      error
	deleteErr   map[uint64]error
	getOptsSeen []int
	deleted     []uint64
	purged      bool
	purgeReq    jetstream.StreamPurgeRequest
}

func (s *fakeXnetStream) Info(context.Context, ...jetstream.StreamInfoOpt) (*jetstream.StreamInfo, error) {
	return s.info, nil
}

func (s *fakeXnetStream) GetMsg(_ context.Context, seq uint64, opts ...jetstream.GetMsgOpt) (*jetstream.RawStreamMsg, error) {
	s.getOptsSeen = append(s.getOptsSeen, len(opts))
	if s.getErr != nil {
		return nil, s.getErr
	}
	sequences := make([]int, 0, len(s.messages))
	for candidate := range s.messages {
		sequences = append(sequences, int(candidate))
	}
	sort.Ints(sequences)
	for _, candidate := range sequences {
		if uint64(candidate) >= seq {
			return s.messages[uint64(candidate)], nil
		}
	}
	return nil, jetstream.ErrMsgNotFound
}

func (s *fakeXnetStream) DeleteMsg(_ context.Context, seq uint64) error {
	s.deleted = append(s.deleted, seq)
	if err := s.deleteErr[seq]; err != nil {
		return err
	}
	delete(s.messages, seq)
	return nil
}

func (s *fakeXnetStream) Purge(_ context.Context, opts ...jetstream.StreamPurgeOpt) error {
	for _, opt := range opts {
		if err := opt(&s.purgeReq); err != nil {
			return err
		}
	}
	s.purged = true
	return nil
}

func TestApplyXnetMigrationStreamLimitsPreservesExistingConfig(t *testing.T) {
	existing := jetstream.StreamConfig{
		Name: "lfsMigrationStream", Subjects: []string{LfsMigrationSubject},
		Retention: jetstream.WorkQueuePolicy, Storage: jetstream.FileStorage,
		Replicas: 3, MaxAge: time.Hour, MaxMsgs: -1, MaxBytes: -1,
	}
	updated, err := applyXnetMigrationStreamLimits(existing, 1000, 1<<30)
	require.NoError(t, err)
	require.Equal(t, int64(1000), updated.MaxMsgs)
	require.Equal(t, int64(1<<30), updated.MaxBytes)
	require.Equal(t, jetstream.DiscardNew, updated.Discard)
	require.Equal(t, existing.Name, updated.Name)
	require.Equal(t, existing.Subjects, updated.Subjects)
	require.Equal(t, existing.Retention, updated.Retention)
	require.Equal(t, existing.Storage, updated.Storage)
	require.Equal(t, existing.Replicas, updated.Replicas)
	require.Equal(t, existing.MaxAge, updated.MaxAge)
}

func TestApplyXnetMigrationStreamLimitsRejectsUnownedStream(t *testing.T) {
	_, err := applyXnetMigrationStreamLimits(jetstream.StreamConfig{
		Name: "lfsMigrationStream", Subjects: []string{"another.subject"}, Retention: jetstream.WorkQueuePolicy,
	}, 1000, 1<<30)
	require.ErrorContains(t, err, LfsMigrationSubject)

	_, err = applyXnetMigrationStreamLimits(jetstream.StreamConfig{
		Name: "lfsMigrationStream", Subjects: []string{LfsMigrationSubject}, Retention: jetstream.LimitsPolicy,
	}, 1000, 1<<30)
	require.ErrorContains(t, err, "work queue retention")
}

func TestApplyXnetMigrationStreamLimitsRejectsSharedStream(t *testing.T) {
	_, err := applyXnetMigrationStreamLimits(jetstream.StreamConfig{
		Name:      "lfsMigrationStream",
		Subjects:  []string{LfsMigrationSubject, "another.subject"},
		Retention: jetstream.WorkQueuePolicy,
	}, 1000, 1<<30)
	require.ErrorContains(t, err, "must exclusively own")
}

func TestDeleteXnetMessagesByFilterUsesRawMessagesAndOnlyDeletesMatches(t *testing.T) {
	stream := &fakeXnetStream{
		info: &jetstream.StreamInfo{Config: jetstream.StreamConfig{
			Name: LfsXnetMigrateGroup.StreamName, Subjects: []string{LfsMigrationSubject}, Retention: jetstream.WorkQueuePolicy,
		}, State: jetstream.StreamState{FirstSeq: 2, LastSeq: 8, Msgs: 3}},
		messages: map[uint64]*jetstream.RawStreamMsg{
			2: {Sequence: 2, Subject: LfsMigrationSubject, Data: []byte("other")},
			5: {Sequence: 5, Subject: LfsMigrationSubject, Data: []byte("target")},
			8: {Sequence: 8, Subject: LfsMigrationSubject, Data: []byte("other")},
		},
		deleteErr: map[uint64]error{},
	}

	err := deleteXnetMessagesByFilter(context.Background(), stream, func(data []byte) bool {
		return string(data) == "target"
	})
	require.NoError(t, err)
	require.Equal(t, []uint64{5}, stream.deleted)
	require.NotEmpty(t, stream.getOptsSeen)
	for _, count := range stream.getOptsSeen {
		require.Equal(t, 1, count, "each raw lookup must be restricted to the migration subject")
	}
}

func TestDeleteXnetMessagesByFilterToleratesConcurrentDelete(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	stream := &fakeXnetStream{
		info: &jetstream.StreamInfo{Config: jetstream.StreamConfig{
			Name: LfsXnetMigrateGroup.StreamName, Subjects: []string{LfsMigrationSubject}, Retention: jetstream.WorkQueuePolicy,
		}, State: jetstream.StreamState{FirstSeq: 1, LastSeq: 1, Msgs: 1}},
		messages: map[uint64]*jetstream.RawStreamMsg{
			1: {Sequence: 1, Subject: LfsMigrationSubject, Data: []byte("target")},
		},
		deleteErr: map[uint64]error{1: jetstream.ErrMsgNotFound},
	}

	require.NoError(t, deleteXnetMessagesByFilter(context.Background(), stream, func([]byte) bool { return true }))
	require.True(t, strings.Contains(logs.String(), "deleted_count=0"), logs.String())
}

func TestXnetStreamMutationsRejectUnsafeStreamBeforeMutation(t *testing.T) {
	for _, config := range []jetstream.StreamConfig{
		{Name: LfsXnetMigrateGroup.StreamName, Subjects: []string{LfsMigrationSubject, "shared"}, Retention: jetstream.WorkQueuePolicy},
		{Name: LfsXnetMigrateGroup.StreamName, Subjects: []string{LfsMigrationSubject}, Retention: jetstream.LimitsPolicy},
	} {
		stream := &fakeXnetStream{
			info:      &jetstream.StreamInfo{Config: config, State: jetstream.StreamState{FirstSeq: 1, LastSeq: 1, Msgs: 1}},
			messages:  map[uint64]*jetstream.RawStreamMsg{1: {Sequence: 1, Data: []byte("target")}},
			deleteErr: map[uint64]error{},
		}

		err := deleteXnetMessagesByFilter(context.Background(), stream, func([]byte) bool { return true })
		require.Error(t, err)
		require.Empty(t, stream.deleted)

		err = purgeXnetMigrationStream(context.Background(), stream)
		require.Error(t, err)
		require.False(t, stream.purged)
	}
}

func TestPurgeXnetMigrationStreamValidatesThenPurges(t *testing.T) {
	stream := &fakeXnetStream{
		info: &jetstream.StreamInfo{Config: jetstream.StreamConfig{
			Name: LfsXnetMigrateGroup.StreamName, Subjects: []string{LfsMigrationSubject}, Retention: jetstream.WorkQueuePolicy,
		}},
	}

	require.NoError(t, purgeXnetMigrationStream(context.Background(), stream))
	require.True(t, stream.purged)
	require.Equal(t, LfsMigrationSubject, stream.purgeReq.Subject)
}

func TestDeleteXnetMessagesByFilterReturnsUnexpectedReadAndDeleteErrors(t *testing.T) {
	readErr := errors.New("read failed")
	stream := &fakeXnetStream{
		info: &jetstream.StreamInfo{Config: jetstream.StreamConfig{
			Name: LfsXnetMigrateGroup.StreamName, Subjects: []string{LfsMigrationSubject}, Retention: jetstream.WorkQueuePolicy,
		}, State: jetstream.StreamState{Msgs: 1}},
		messages: map[uint64]*jetstream.RawStreamMsg{}, deleteErr: map[uint64]error{}, getErr: readErr,
	}
	require.ErrorIs(t, deleteXnetMessagesByFilter(context.Background(), stream, func([]byte) bool { return true }), readErr)

	deleteErr := errors.New("delete failed")
	stream.getErr = nil
	stream.info.State.FirstSeq = 1
	stream.info.State.LastSeq = 1
	stream.messages[1] = &jetstream.RawStreamMsg{Sequence: 1, Subject: LfsMigrationSubject, Data: []byte("target")}
	stream.deleteErr[1] = deleteErr
	require.ErrorIs(t, deleteXnetMessagesByFilter(context.Background(), stream, func([]byte) bool { return true }), deleteErr)
}
