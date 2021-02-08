package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"
)

func readLastUserIDTicketState(ctx context.Context, nk runtime.NakamaModule, userID string) (*nakamaCommands.TicketState, error) {
	objects, _, err := nk.StorageList(ctx, userID, nakamaCommands.TICKET_COLLECTION, 100, "")
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("No tickets found for userID %v", userID)
	}

	sort.Slice(objects[:], func(i, j int) bool {
		return objects[i].CreateTime.Seconds < objects[j].CreateTime.Seconds
	})

	return readTicketState(ctx, nk, objects[0].Key, userID)
}

func deleteTicketsFromMatchState(ctx context.Context, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) error {
	if err := deleteTicketsByTeamUsers(ctx, nk, matchState); err != nil {
		log.Error(err)
		return err
	}

	if err := deleteTicketsByPoolUserIDs(ctx, nk, matchState); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func deleteTicketsByTeamUsers(ctx context.Context, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) error {
	teamUsers := nakamaCommands.GetTeamUsersFromMatch(matchState)
	var userIDs []string
	var ticketIDs []string
	for _, teamUser := range teamUsers {
		userIDs = append(userIDs, teamUser.User.Nakama.ID)
		ticketIDs = append(ticketIDs, teamUser.TicketID)
	}
	return deleteTicketByUserIDAndTicketID(ctx, nk, userIDs, ticketIDs)
}

func deleteTicketsByPoolUserIDs(ctx context.Context, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) error {
	var userIDs []string
	var ticketIDs []string
	for _, userID := range matchState.PoolUserIDs {
		ticketState, err := readLastUserIDTicketState(ctx, nk, userID)
		if err != nil {
			log.Error(err)
			return err
		}

		userIDs = append(userIDs, userID)
		ticketIDs = append(ticketIDs, ticketState.Ticket.Id)
	}
	return deleteTicketByUserIDAndTicketID(ctx, nk, userIDs, ticketIDs)
}

func deleteTicketByUserIDAndTicketID(ctx context.Context, nk runtime.NakamaModule, userIDs []string, ticketIDs []string) error {
	for i, _ := range userIDs {
		log.Infof("i %v", i)
		err := deleteTicketState(ctx, nk, ticketIDs[i], userIDs[i])
		if err != nil {
			log.Error(err)
			return err
		}

		currentUserData, err := readLastUserData(ctx, nk, userIDs[i])
		if err != nil {
			log.Error(err)
			return err
		}

		currentUserData.TicketID = ""

		if err := writeLastUserData(ctx, nk, currentUserData, userIDs[i]); err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

func deleteTicketState(ctx context.Context, nk runtime.NakamaModule, ticketID string, userID string) error {
	ticketState, err := readTicketState(ctx, nk, ticketID, userID)
	if err != nil {
		log.Error(err)
		return err
	}
	if ticketState == nil {
		log.Errorf("Ticket %v for user %v not found, delete operation canceled", ticketID, userID)
		return nil
	}

	if err := nk.StorageDelete(ctx, []*runtime.StorageDelete{
		&runtime.StorageDelete{
			Collection: nakamaCommands.TICKET_COLLECTION,
			Key:        ticketID,
			UserID:     userID,
		},
	}); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func readTicketState(ctx context.Context, nk runtime.NakamaModule, ticketID string, userID string) (*nakamaCommands.TicketState, error) {
	var ticketState *nakamaCommands.TicketState
	storageObjects, err := nk.StorageRead(ctx, []*runtime.StorageRead{&runtime.StorageRead{
		Collection: nakamaCommands.TICKET_COLLECTION,
		Key:        ticketID,
		UserID:     userID,
	}})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if len(storageObjects) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(storageObjects[0].Value), &ticketState); err != nil {
		log.Error(err)
		return nil, err
	}
	ticketState.Version = storageObjects[0].Version
	return ticketState, nil
}

func TicketStateCreateRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	log.Info(payload)
	var ticketStateCreateRequest *nakamaCommands.TicketStateCreateRequest
	json.Unmarshal([]byte(payload), &ticketStateCreateRequest)

	if err := writeTicketState(ctx, nk, ticketStateCreateRequest.TicketState, ticketStateCreateRequest.UserID); err != nil {
		log.Infof("unable to create ticket state: %q", err.Error())
		return "", err
	}

	return "", nil
}

func writeTicketState(ctx context.Context, nk runtime.NakamaModule, ticketState *nakamaCommands.TicketState, userID string) error {
	acks, err := nk.StorageWrite(ctx, []*runtime.StorageWrite{
		&runtime.StorageWrite{
			Collection:      nakamaCommands.TICKET_COLLECTION,
			Key:             ticketState.Ticket.Id,
			Value:           string(Marshal(ticketState)),
			UserID:          userID,
			PermissionWrite: runtime.STORAGE_PERMISSION_OWNER_WRITE,
			PermissionRead:  runtime.STORAGE_PERMISSION_PUBLIC_READ,
			Version:         ticketState.Version,
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
