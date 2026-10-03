package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	"option-tab/internal/folder"
)

type (
	MediaLyricsScope struct {
		Provider MediaProvider `json:"provider"`
		TrackID  string        `json:"trackID"`
	}
	MediaLyricsFile struct {
		DocumentID string           `json:"documentID"`
		Scope      MediaLyricsScope `json:"scope"`
		Data       []byte           `json:"-"`
		OffsetMS   int64            `json:"offsetMS"`
		Status     string           `json:"status"`
		Reason     string           `json:"reason"`
	}
	MediaLyricsSource interface {
		ChooseMediaLyrics(context.Context, MediaLyricsScope, func([]byte) error) (MediaLyricsFile, error)
		LoadMediaLyrics(context.Context, MediaLyricsScope) (MediaLyricsFile, error)
		RemoveMediaLyrics(context.Context, MediaLyricsScope) error
		SetMediaLyricsOffset(context.Context, MediaLyricsScope, int64) error
	}
)

type (
	lyricsRecord struct {
		DocumentID    string
		Scope         MediaLyricsScope
		Bookmark      []byte
		Device, Inode uint64
		OffsetMS      int64
	}
	lyricsSelection struct {
		Record lyricsRecord
		Data   []byte
	}
	lyricsTransport interface {
		choose(context.Context) (lyricsSelection, error)
		read(context.Context, lyricsRecord) ([]byte, error)
	}
	mediaLyricsSource struct {
		store     *folder.BookmarkStore
		native    lyricsTransport
		chooser   chan struct{}
		mu        sync.Mutex
		revisions map[string]uint64
	}
)

func newMediaLyricsSource(path string, native lyricsTransport) *mediaLyricsSource {
	return &mediaLyricsSource{store: folder.NewBookmarkStore(path), native: native, chooser: make(chan struct{}, 1), revisions: map[string]uint64{}}
}

func lyricsKey(scope MediaLyricsScope) (string, error) {
	if (scope.Provider != MediaMusic && scope.Provider != MediaSpotify) || scope.TrackID == "" || len(scope.TrackID) > 4096 {
		return "", errors.New("invalid media lyric scope")
	}
	bytes, _ := json.Marshal(scope)
	key := sha256.Sum256(bytes)
	return hex.EncodeToString(key[:]), nil
}

func lyricFailure(scope MediaLyricsScope, err error) (MediaLyricsFile, error) {
	return MediaLyricsFile{Scope: scope, Status: "unavailable", Reason: err.Error()}, err
}

func (s *mediaLyricsSource) ChooseMediaLyrics(ctx context.Context, scope MediaLyricsScope, validate func([]byte) error) (MediaLyricsFile, error) {
	key, err := lyricsKey(scope)
	if err != nil {
		return lyricFailure(scope, err)
	}
	if validate == nil {
		return lyricFailure(scope, errors.New("lyric validation required"))
	}
	if err = ctx.Err(); err != nil {
		return lyricFailure(scope, err)
	}
	select {
	case s.chooser <- struct{}{}:
		defer func() { <-s.chooser }()
	default:
		return lyricFailure(scope, errors.New("lyric chooser already active"))
	}
	s.mu.Lock()
	revision := s.revisions[key]
	s.mu.Unlock()
	selected, err := s.native.choose(ctx)
	if err != nil {
		return lyricFailure(scope, err)
	}
	if err = ctx.Err(); err != nil {
		return lyricFailure(scope, err)
	}
	if len(selected.Data) == 0 || len(selected.Data) > 1<<20 || len(selected.Record.Bookmark) == 0 || selected.Record.Inode == 0 {
		return lyricFailure(scope, errors.New("lyric file is invalid or exceeds limit"))
	}
	if err = validate(append([]byte(nil), selected.Data...)); err != nil {
		return lyricFailure(scope, err)
	}
	record := selected.Record
	record.Scope = scope
	record.OffsetMS = 0
	token := make([]byte, 16)
	if _, err = rand.Read(token); err != nil {
		return lyricFailure(scope, err)
	}
	record.DocumentID = hex.EncodeToString(token)
	encoded, err := json.Marshal(record)
	if err != nil {
		return lyricFailure(scope, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return lyricFailure(scope, err)
	}
	if s.revisions[key] != revision {
		return lyricFailure(scope, errors.New("lyric association retired"))
	}
	if err = s.store.Put(key, encoded); err != nil {
		return lyricFailure(scope, err)
	}
	s.revisions[key]++
	return MediaLyricsFile{DocumentID: record.DocumentID, Scope: scope, Data: append([]byte(nil), selected.Data...), Status: "ready"}, nil
}

func (s *mediaLyricsSource) record(key string, scope MediaLyricsScope) (lyricsRecord, error) {
	data, err := s.store.Get(key)
	if err != nil {
		return lyricsRecord{}, err
	}
	if len(data) == 0 {
		return lyricsRecord{}, nil
	}
	var record lyricsRecord
	if err = json.Unmarshal(data, &record); err != nil {
		return record, err
	}
	if record.Scope != scope || record.DocumentID == "" || len(record.Bookmark) == 0 || record.Inode == 0 || record.OffsetMS < -30000 || record.OffsetMS > 30000 {
		return record, errors.New("lyric association identity invalid")
	}
	return record, nil
}

func (s *mediaLyricsSource) LoadMediaLyrics(ctx context.Context, scope MediaLyricsScope) (MediaLyricsFile, error) {
	key, err := lyricsKey(scope)
	if err != nil {
		return lyricFailure(scope, err)
	}
	if err = ctx.Err(); err != nil {
		return lyricFailure(scope, err)
	}
	s.mu.Lock()
	record, err := s.record(key, scope)
	revision := s.revisions[key]
	s.mu.Unlock()
	if err != nil {
		return lyricFailure(scope, err)
	}
	if record.DocumentID == "" {
		return MediaLyricsFile{Scope: scope, Status: "missing"}, nil
	}
	data, err := s.native.read(ctx, record)
	if err != nil {
		return lyricFailure(scope, err)
	}
	if err = ctx.Err(); err != nil {
		return lyricFailure(scope, err)
	}
	if len(data) == 0 || len(data) > 1<<20 {
		return lyricFailure(scope, errors.New("lyric file is invalid or exceeds limit"))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return lyricFailure(scope, err)
	}
	if s.revisions[key] != revision {
		return lyricFailure(scope, errors.New("lyric association retired"))
	}
	return MediaLyricsFile{DocumentID: record.DocumentID, Scope: scope, Data: append([]byte(nil), data...), OffsetMS: record.OffsetMS, Status: "ready"}, nil
}

func (s *mediaLyricsSource) RemoveMediaLyrics(ctx context.Context, scope MediaLyricsScope) error {
	key, err := lyricsKey(scope)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = s.store.Delete(key); err != nil {
		return err
	}
	s.revisions[key]++
	return nil
}

func (s *mediaLyricsSource) SetMediaLyricsOffset(ctx context.Context, scope MediaLyricsScope, offset int64) error {
	key, err := lyricsKey(scope)
	if err != nil {
		return err
	}
	if offset < -30000 || offset > 30000 {
		return errors.New("lyric offset exceeds limit")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	record, err := s.record(key, scope)
	if err != nil {
		return err
	}
	if record.DocumentID == "" {
		return errors.New("lyric association missing")
	}
	record.OffsetMS = offset
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err = s.store.Put(key, data); err != nil {
		return err
	}
	s.revisions[key]++
	return nil
}
