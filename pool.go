package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"
)

func CreateFakeUsers(ctx context.Context, nk runtime.NakamaModule) error {
	for i := 0; i < 10; i++ {
		username := "testuser#" + strconv.Itoa(i)
		if string1, string2, val, err := nk.AuthenticateCustom(ctx, username, username, true); err != nil {
			log.Infof("%+s %+s %+s %+s", string1, string2, val, err)
			log.Error(err)
		}
	}
	return nil
}

func PoolJoinRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var request *nakamaCommands.MatchPoolJoinRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Error(err)
		return "", err
	}

	msg, err := poolJoin(ctx, nk, request.MatchID, request.UserID)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return msg, nil
}

func poolJoin(ctx context.Context, nk runtime.NakamaModule, matchID string, userID string) (string, error) {
	matchState, err := readMatchState(ctx, nk, getDummyMatchState(matchID, nakamaCommands.MATCH_COLLECTION))
	if err != nil {
		log.Error(err)
		return "", err
	}

	account, err := nk.AccountGetId(ctx, userID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	if nakamaCommands.IsStringInSlice(userID, matchState.PoolUserIDs) {
		return fmt.Sprintf("User <@%v> already joined match pool %v", account.CustomId, matchID), nil
	}

	matchState.PoolUserIDs = append(matchState.PoolUserIDs, userID)
	matchState.PoolUserCustomIDs = append(matchState.PoolUserCustomIDs, account.CustomId)
	_, err = writeMatchState(ctx, nk, matchState)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return fmt.Sprintf("User <@%v> joined match pool %v", account.CustomId, matchID), nil
}

func PoolPickRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var request *nakamaCommands.MatchPoolPickRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Error(err)
		return "", err
	}

	msg, err := poolPick(ctx, nk, request.MatchID, request.CaptainUserID, request.UserID)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return msg, nil
}

func poolPick(ctx context.Context, nk runtime.NakamaModule, matchID string, captainUserID string, userID string) (string, error) {
	matchState, err := readMatchState(ctx, nk, getDummyMatchState(matchID, nakamaCommands.MATCH_COLLECTION))
	if err != nil {
		log.Error(err)
		return "", err
	}

	captainAccount, err := nk.AccountGetId(ctx, captainUserID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	if !nakamaCommands.IsStringInSlice(captainAccount.CustomId, matchState.CaptainUserIDs) {
		return fmt.Sprintf("User <@%v> is not a captain in match %v", captainAccount.CustomId, matchID), nil
	}

	if captainAccount.CustomId != matchState.CaptainTurnUserID {
		return fmt.Sprintf("Current captain draft turn is for captain <@%v>, match %v", matchState.CaptainTurnUserID, matchID), nil
	}

	account, err := nk.AccountGetId(ctx, userID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	if !nakamaCommands.IsStringInSlice(userID, matchState.PoolUserIDs) {
		return fmt.Sprintf("User <@%v> is not joined match pool %v", account.CustomId, matchID), nil
	}

	var newPoolUserIDs []string
	for _, v := range matchState.PoolUserIDs {
		if v != userID {
			newPoolUserIDs = append(newPoolUserIDs, v)
		}
	}
	matchState.PoolUserIDs = newPoolUserIDs

	var newPoolUserCustomIDs []string
	for _, v := range matchState.PoolUserCustomIDs {
		if v != account.CustomId {
			newPoolUserIDs = append(newPoolUserIDs, v)
		}
	}
	matchState.PoolUserCustomIDs = newPoolUserCustomIDs

	ticketState, err := readLastUserIDTicketState(ctx, nk, userID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	teamUser, _ := nakamaCommands.UnmarshalTeamUser(ticketState.Ticket.Extensions[nakamaCommands.TICKET_EXTENSION_USER].Value)
	teamUser.Captain = false
	teamUser.TicketID = ticketState.Ticket.Id
	teamUser.Reward = 0

	teamNumber := nakamaCommands.GetTeamNumberFromUserAndMatch(captainUserID, matchState)
	log.Infof("%+v\n", teamNumber)
	matchState.Teams[teamNumber].TeamUsers = append(matchState.Teams[teamNumber].TeamUsers, teamUser)
	matchState.ReadyUserIDs = append(matchState.ReadyUserIDs, teamUser.User.Nakama.ID)

	nextCaptainTurnUserID := getNextCaptainTurnUserID(matchState)
	log.Infof("Next CaptainTurnUserID %v", nextCaptainTurnUserID)

	nextCaptainTurnUserIDMsg := ""
	if nextCaptainTurnUserID != matchState.CaptainTurnUserID && nextCaptainTurnUserID != "" {
		nextCaptainTurnUserIDMsg = fmt.Sprintf("\nCaptain <@%v>'s pick turn!", nextCaptainTurnUserID)
		matchState.CaptainTurnUserID = nextCaptainTurnUserID
	}

	_, err = writeMatchState(ctx, nk, matchState)
	if err != nil {
		log.Error(err)
		return "", err
	}

	if err := notifyDiscordUsers(
		nakamaCommands.GetUsersFromMatch(matchState),
		fmt.Sprintf("User <@%v> was picked by captain <@%v> in  match %v. %v", account.CustomId, captainAccount.CustomId, matchID, nextCaptainTurnUserIDMsg)); err != nil {
		log.Error(err)
	}
	return "", nil
}

func getNextCaptainTurnUserID(matchState *nakamaCommands.MatchState) string {
	usersPerCaptainTurn := nakamaCommands.CAPTAINS_DRAFT_MODES_MAP[matchState.MatchProfile].UsersPerCaptainTurn
	log.Infof("usersPerCaptainTurn %v", usersPerCaptainTurn)

	captainsCount := nakamaCommands.CAPTAINS_DRAFT_MODES_MAP[matchState.MatchProfile].TeamCount
	log.Infof("captainsCount %v", captainsCount)

	usersInTeam := nakamaCommands.CAPTAINS_DRAFT_MODES_MAP[matchState.MatchProfile].UsersInTeam
	log.Infof("usersInTeam %v", usersInTeam)

	currentUsersInTeamsCount := len(nakamaCommands.GetTeamUsersFromTeams(matchState.Teams))
	log.Infof("currentUsersInTeamsCount %v", currentUsersInTeamsCount)

	if currentUsersInTeamsCount == usersInTeam*captainsCount {
		return ""
	}

	var currentCaptainIndex int
	for i, captainUserID := range matchState.CaptainUserIDs {
		if matchState.CaptainTurnUserID == captainUserID {
			currentCaptainIndex = i
			log.Infof("currentCaptainIndex %v", currentCaptainIndex)
			break
		}
	}

	captainUserIDsIterator := nakamaCommands.NewSliceIterator(currentCaptainIndex, matchState.CaptainUserIDs)

	expectedUsersCount := captainsCount
	for i, usersOnCaptainTurn := range usersPerCaptainTurn {
		expectedUsersCount += usersOnCaptainTurn
		log.Infof("captain turn: %v, expectedUsersCount %v", i, expectedUsersCount)
		if expectedUsersCount >= currentUsersInTeamsCount {
			if expectedUsersCount-currentUsersInTeamsCount >= 1 {
				return matchState.CaptainTurnUserID
			}
			return captainUserIDsIterator.Next()
		}
	}
	return captainUserIDsIterator.Next()
}
