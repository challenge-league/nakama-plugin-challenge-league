package main

import (
	"context"
	"database/sql"
	"encoding/json"

	log "github.com/micro/go-micro/v2/logger"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
)

func Unmarshal(payload string) (map[string]interface{}, error) {
	params := make(map[string]interface{})
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return nil, err
	}
	return params, nil
}

func Marshal(v interface{}) []byte {
	resultJSON, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	return resultJSON
}

func MarshalIndent(v interface{}) string {
	resultJSON, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	return string(resultJSON)
}

func InitModule(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) error {
	NewDiscordSessionSingleton()

	//UpdateKaggleCompetitions()

	if err := initializer.RegisterRpc("TournamentCreate", TournamentCreateRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("TournamentDelete", TournamentDeleteRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("MatchCreate", MatchCreateRPC); err != nil {
		return err
	}
	/*
		if err := initializer.RegisterRpc("MatchGet", MatchGetRPC); err != nil {
			return err
		}
	*/
	if err := initializer.RegisterRpc("MatchReady", MatchReadyRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("MatchCancel", MatchCancelRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("MatchResult", MatchResultRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("MatchStateGet", MatchStateGetRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("MatchStateListGet", MatchStateListGetRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("LeaderboardCreate", LeaderboardCreateRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("LeaderboardDelete", LeaderboardDeleteRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("LeaderboardRecordWrite", LeaderboardRecordWriteRPC); err != nil {
		return err
	}
	if err := initializer.RegisterBeforeAuthenticateCustom(beforeAuthenticateCustom); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("AccountUpdateID", AccountUpdateIDRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("AccountByUsernameGet", AccountByUsernameGetRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("AccountByCustomIDGet", AccountByCustomIDGetRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("LastUserDataCreate", LastUserDataCreateRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("PoolJoin", PoolJoinRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("PoolPick", PoolPickRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("StorageWrite", StorageWriteRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("SubmitCreate", SubmitCreateRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("TicketStateCreate", TicketStateCreateRPC); err != nil {
		return err
	}

	// open-match RPC
	if err := initializer.RegisterRpc("OpenMatchFrontendTicketCreate", OpenMatchFrontendTicketCreateRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("OpenMatchFrontendTicketGet", OpenMatchFrontendTicketGetRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("OpenMatchFrontendTicketDelete", OpenMatchFrontendTicketDeleteRPC); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("OpenMatchFrontendTicketWatchAssignments", OpenMatchFrontendTicketWatchAssignmentsRPC); err != nil {
		return err
	}

	// Match
	if err := initializer.RegisterMatch(nakamaCommands.DEFAULT_NAKAMA_MATCH_MODULE, func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule) (runtime.Match, error) {
		return &Match{}, nil
	}); err != nil {
		log.Errorf("Unable to register: %v", err)
		return err
	}
	if err := initializer.RegisterMatch("match", NewMatch); err != nil {
		return err
	}
	if err := initializer.RegisterEventSessionStart(eventSessionStart); err != nil {
		return err
	}
	if err := initializer.RegisterEventSessionEnd(eventSessionEnd); err != nil {
		return err
	}
	/*
		if err := initializer.RegisterTournamentEnd(distributeRewards); err != nil {
			return err
		}
	*/

	if err := CreateFakeUsers(ctx, nk); err != nil {
		return err
	}
	if err := RestoreActiveMatchesAfterRestart(ctx, nk); err != nil {
		return err
	}
	if err := CreateLeaderboardsIfNotExist(ctx, nk); err != nil {
		return err
	}
	log.Infof("nakama-plugin-challenge-league.so module loaded")
	return nil
}
