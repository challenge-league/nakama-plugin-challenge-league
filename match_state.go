package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	nakamaContext "github.com/challenge-league/nakama-go/context"
	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"
)

func writeMatchStateInLoop(ctx context.Context, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) *nakamaCommands.MatchState {
	matchState, err := writeMatchState(ctx, nk, matchState)
	if err != nil {
		log.Errorf("Error %+v", err)
		return nil
	}
	return matchState
}

func archiveMatchState(ctx context.Context, nk runtime.NakamaModule, s *nakamaCommands.MatchState) error {
	matchState := *s
	matchState.StorageCollection = nakamaCommands.MATCH_ARCHIVE_COLLECTION
	matchState.Active = false
	matchState.ActualDateTimeEnd = time.Now()
	matchState.ActualDuration = matchState.ActualDateTimeEnd.Sub(matchState.DateTimeStart)
	matchState.Version = "*"

	if _, err := writeMatchState(ctx, nk, &matchState); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func getDummyMatchState(matchID string, collection string) *nakamaCommands.MatchState {
	return &nakamaCommands.MatchState{
		StorageCollection: collection,
		MatchID:           matchID,
		StorageUserID:     nakamaContext.NakamaSystemUserID,
	}
}

func MatchStateGetRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var request *nakamaCommands.MatchStateGetRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Error(err)
		return "", err
	}

	matchState, err := readMatchState(ctx, nk, getDummyMatchState(request.ID, request.StorageCollection))
	if err != nil {
		log.Error(err)
		return "", err
	}
	return MarshalIndent(matchState), nil
}

func matchStateListGet(ctx context.Context, nk runtime.NakamaModule, userID string, collection string) ([]*nakamaCommands.MatchState, error) {
	storageCollection := nakamaCommands.MATCH_COLLECTION
	if collection != "" {
		storageCollection = collection
	}

	storageObjects, _, err := nk.StorageList(ctx,
		userID,
		storageCollection,
		nakamaCommands.MAX_LIST_LIMIT,
		"",
	)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if collection == "" && len(storageObjects) == 0 {
		storageObjects, _, err = nk.StorageList(ctx,
			userID,
			nakamaCommands.MATCH_ARCHIVE_COLLECTION,
			nakamaCommands.MAX_LIST_LIMIT,
			"",
		)
	}
	if len(storageObjects) == 0 {
		log.Info("No match found")
		return nil, nil
	}
	var matchStateList []*nakamaCommands.MatchState
	for _, object := range storageObjects {
		var state *nakamaCommands.MatchState
		if err := json.Unmarshal([]byte(object.Value), &state); err != nil {
			log.Error(err)
			return nil, err
		}
		state.Version = object.Version
		matchStateList = append(matchStateList, state)
	}
	return matchStateList, nil
}

func MatchStateListGetRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var request *nakamaCommands.MatchStateListGetRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Error(err)
		return "", err
	}
	matchStateList, err := matchStateListGet(ctx, nk, request.UserID, request.StorageCollection)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return string(Marshal(matchStateList)), nil
}

func deleteMatchState(ctx context.Context, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) error {
	err := nk.StorageDelete(ctx, []*runtime.StorageDelete{
		&runtime.StorageDelete{
			Collection: matchState.StorageCollection,
			Key:        matchState.MatchID,
			UserID:     matchState.StorageUserID,
		},
	})

	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func readMatchState(ctx context.Context, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) (*nakamaCommands.MatchState, error) {
	log.Infof("%+v", matchState.StorageCollection)
	storageCollection := nakamaCommands.MATCH_COLLECTION
	if matchState.StorageCollection != "" {
		storageCollection = matchState.StorageCollection
	}
	storageObjects, err := nk.StorageRead(ctx, []*runtime.StorageRead{&runtime.StorageRead{
		Collection: storageCollection,
		Key:        matchState.MatchID,
		UserID:     matchState.StorageUserID,
	}})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if matchState.StorageCollection == "" && len(storageObjects) == 0 {
		storageObjects, err = nk.StorageRead(ctx, []*runtime.StorageRead{&runtime.StorageRead{
			Collection: nakamaCommands.MATCH_ARCHIVE_COLLECTION,
			Key:        matchState.MatchID,
			UserID:     matchState.StorageUserID,
		}})
	}
	if len(storageObjects) == 0 {
		return nil, runtime.NewError(fmt.Sprintf("No match found with ID: %v", matchState.MatchID), 404)
	}
	var state *nakamaCommands.MatchState
	if err := json.Unmarshal([]byte(storageObjects[0].Value), &state); err != nil {
		log.Error(err)
		return nil, err
	}
	state.Version = storageObjects[0].Version
	return state, nil
}

func writeMatchState(ctx context.Context, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) (*nakamaCommands.MatchState, error) {
	log.Infof("Writing match state: %+v", matchState)
	acks, err := nk.StorageWrite(ctx, []*runtime.StorageWrite{
		&runtime.StorageWrite{
			Collection:      matchState.StorageCollection,
			Key:             matchState.MatchID,
			Value:           string(Marshal(matchState)),
			UserID:          matchState.StorageUserID,
			PermissionWrite: runtime.STORAGE_PERMISSION_NO_WRITE,
			PermissionRead:  runtime.STORAGE_PERMISSION_PUBLIC_READ,
			Version:         matchState.Version,
		},
	})

	if err != nil {
		log.Error(err)
		return nil, err
	}

	if len(acks) != 1 {
		log.Errorf("Invocation failed. Return result not expected: ", len(acks))
		return nil, err
	}
	matchState.Version = acks[0].Version

	return matchState, err
}
