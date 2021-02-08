package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"
)

func readSubmits(ctx context.Context, nk runtime.NakamaModule, MatchID string, UserID string) (*nakamaCommands.Submits, error) {
	var submits *nakamaCommands.Submits
	storageObjects, err := nk.StorageRead(ctx, []*runtime.StorageRead{&runtime.StorageRead{
		Collection: nakamaCommands.SUBMIT_COLLECTION,
		Key:        MatchID,
		UserID:     UserID,
	}})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if len(storageObjects) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(storageObjects[0].Value), &submits); err != nil {
		log.Error(err)
		return nil, err
	}
	submits.Version = storageObjects[0].Version
	return submits, nil
}

func writeSubmits(ctx context.Context, nk runtime.NakamaModule, submits *nakamaCommands.Submits) (*nakamaCommands.Submits, error) {
	acks, err := nk.StorageWrite(ctx, []*runtime.StorageWrite{
		&runtime.StorageWrite{
			Collection:      nakamaCommands.SUBMIT_COLLECTION,
			Key:             submits.MatchID,
			Value:           string(Marshal(submits)),
			UserID:          submits.UserID,
			PermissionWrite: runtime.STORAGE_PERMISSION_NO_WRITE,
			PermissionRead:  runtime.STORAGE_PERMISSION_PUBLIC_READ,
			Version:         submits.Version,
		},
	})

	if err != nil {
		log.Error(err)
		return nil, err
	}

	if len(acks) != 1 {
		log.Infof("Invocation failed. Return result not expected: ", len(acks))
		return nil, err
	}

	submits.Version = acks[0].Version
	return submits, nil
}

func SubmitCreateRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var submitCreateRequest *nakamaCommands.SubmitCreateRequest
	json.Unmarshal([]byte(payload), &submitCreateRequest)
	submitCreateRequest.Submit.Datetime = time.Now().UTC()
	log.Infof(MarshalIndent(submitCreateRequest))

	submits, err := readSubmits(ctx, nk, submitCreateRequest.MatchID, submitCreateRequest.UserID)
	if err != nil {
		return "", err
	}
	if submits == nil {
		submits = &nakamaCommands.Submits{
			Submits: []*nakamaCommands.Submit{submitCreateRequest.Submit},
			MatchID: submitCreateRequest.MatchID,
			UserID:  submitCreateRequest.UserID,
			Version: "*",
		}
	} else {
		submits.Submits = append(submits.Submits, submitCreateRequest.Submit)
	}

	account, err := nk.AccountGetId(ctx, submitCreateRequest.UserID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	metadata := make(map[string]interface{})
	metadata["submit"] = submitCreateRequest.Submit
	_, err = nk.TournamentRecordWrite(ctx, submitCreateRequest.MatchID, submitCreateRequest.UserID,
		account.CustomId,
		submitCreateRequest.Submit.Score, submitCreateRequest.Submit.Subscore, metadata)
	if err != nil {
		log.Error(err)
		if err.Error() == "sql: no rows in result set" {
			return "", fmt.Errorf("Unable to write a submit score **smaller or equal** than the existing one")
		}
		return "", err
	}

	submits, err = writeSubmits(ctx, nk, submits)
	if err != nil {
		return "", err
	}
	matchState, err := readMatchState(ctx, nk, getDummyMatchState(submits.MatchID, nakamaCommands.MATCH_COLLECTION))
	if err != nil {
		return "", err
	}
	notifyDiscordUsers(
		nakamaCommands.GetUsersFromMatch(matchState),
		fmt.Sprintf(`<@%v> has submited %v`, account.CustomId, nakamaCommands.PrintSubmit(submitCreateRequest.Submit)))
	if err != nil {
		return "", err
	}
	return "", nil
}
