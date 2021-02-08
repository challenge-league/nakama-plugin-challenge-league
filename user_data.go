package main

import (
	"context"
	"database/sql"
	"encoding/json"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"
)

func createOrUpdateLastUserData(ctx context.Context, nk runtime.NakamaModule, userData *nakamaCommands.UserData, userID string) error {
	currentUserData, err := readLastUserData(ctx, nk, userID)
	if err != nil {
		log.Error(err)
		return err
	}

	currentUserData = nakamaCommands.PatchStructByNewStruct(currentUserData, userData).(*nakamaCommands.UserData)

	if err := writeLastUserData(ctx, nk, currentUserData, userID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func deleteUserData(ctx context.Context, nk runtime.NakamaModule, key string, userID string) error {
	userData, err := readUserData(ctx, nk, key, userID)
	if err != nil {
		log.Error(err)
		return err
	}
	if userData == nil {
		log.Errorf("User data with key %v for user %v not found, delete operation canceled", key, userID)
		return nil
	}

	if err := nk.StorageDelete(ctx, []*runtime.StorageDelete{
		&runtime.StorageDelete{
			Collection: nakamaCommands.USER_DATA_COLLECTION,
			Key:        key,
			UserID:     userID,
		},
	}); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func readLastUserData(ctx context.Context, nk runtime.NakamaModule, userID string) (*nakamaCommands.UserData, error) {
	return readUserData(ctx, nk, nakamaCommands.USER_LAST_DATA_KEY, userID)
}

func readUserData(ctx context.Context, nk runtime.NakamaModule, key string, userID string) (*nakamaCommands.UserData, error) {
	var userData *nakamaCommands.UserData
	storageObjects, err := nk.StorageRead(ctx, []*runtime.StorageRead{&runtime.StorageRead{
		Collection: nakamaCommands.USER_DATA_COLLECTION,
		Key:        key,
		UserID:     userID,
	}})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if len(storageObjects) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(storageObjects[0].Value), &userData); err != nil {
		log.Error(err)
		return nil, err
	}
	userData.Version = storageObjects[0].Version
	return userData, nil
}

func LastUserDataCreateRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	log.Info(payload)
	var userDataCreateRequest *nakamaCommands.LastUserDataCreateRequest
	json.Unmarshal([]byte(payload), &userDataCreateRequest)

	if err := writeLastUserData(ctx, nk, userDataCreateRequest.UserData, userDataCreateRequest.UserID); err != nil {
		log.Infof("unable to create user data: %q", err.Error())
		return "", err
	}

	return "", nil
}

func writeLastUserData(ctx context.Context, nk runtime.NakamaModule, userData *nakamaCommands.UserData, userID string) error {
	return writeUserData(ctx, nk, userData, nakamaCommands.USER_LAST_DATA_KEY, userID)
}

func writeUserData(ctx context.Context, nk runtime.NakamaModule, userData *nakamaCommands.UserData, key string, userID string) error {
	acks, err := nk.StorageWrite(ctx, []*runtime.StorageWrite{
		&runtime.StorageWrite{
			Collection:      nakamaCommands.USER_DATA_COLLECTION,
			Key:             key,
			Value:           string(Marshal(userData)),
			UserID:          userID,
			PermissionWrite: runtime.STORAGE_PERMISSION_OWNER_WRITE,
			PermissionRead:  runtime.STORAGE_PERMISSION_PUBLIC_READ,
			Version:         userData.Version,
		},
	})

	if err != nil {
		log.Error(err)
		return err
	}

	if len(acks) != 1 {
		log.Infof("Invocation failed. Return result not expected: ", len(acks))
		return err
	}

	return nil
}
