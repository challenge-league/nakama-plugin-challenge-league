package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"
)

func createNakamaTournament(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) (*nakamaCommands.MatchState, error) {
	sortOrder := "desc"
	operator := "best"
	resetSchedule := "" // "0,12,*,*,*"
	title := ""
	desc := ""
	category := 1
	infiniteDuration := 9007199254740992 // infinite loop equal to max int value for nakama tournament to avoid tournament stop and no lb records because of internal nakama logic
	maxSize := 10000
	maxNumScore := 9999
	joinRequired := false
	debug := true

	if matchState.MaxNumScore != 0 {
		maxNumScore = matchState.MaxNumScore
	}

	startTime := time.Now().UTC()
	endTime := startTime.Add(time.Second * time.Duration(infiniteDuration))

	payload := Marshal(&nakamaCommands.TournamentCreateRequest{
		ID:            matchState.MatchID,
		SortOrder:     sortOrder,
		Operator:      operator,
		ResetSchedule: strings.ReplaceAll(resetSchedule, ",", " "),
		Metadata:      map[string]interface{}{},
		Title:         title,
		Description:   desc,
		Category:      category,
		StartTime:     int(startTime.Unix()),
		EndTime:       int(endTime.Unix()),
		Duration:      infiniteDuration,
		MaxSize:       maxSize,
		MaxNumScore:   maxNumScore,
		JoinRequired:  joinRequired,
		Debug:         debug,
	})
	log.Infof("%+v\n", string(payload))

	result, err := TournamentCreateRPC(ctx, logger, db, nk, string(payload))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	log.Infof("%+v", MarshalIndent(result))

	matchState.DateTimeStart = startTime
	matchState.DateTimeEnd = startTime.Add(matchState.Duration)

	matchState, err = writeMatchState(ctx, nk, matchState)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	users := nakamaCommands.GetUsersFromMatch(matchState)

	for _, user := range users {
		// Create initial submits on the leaderboard
		metadata := make(map[string]interface{})
		metadata["initialSubmit"] = true
		_, err = nk.TournamentRecordWrite(ctx, matchState.MatchID, user.Nakama.ID, user.Nakama.CustomID, 0, 0, metadata)
		if err != nil {
			log.Error(err)
			return nil, err
		}
	}

	return matchState, nil
}

func TournamentCreateRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	log.Info(payload)
	var params map[string]interface{}
	json.Unmarshal([]byte(payload), &params)

	if err := nk.TournamentCreate(
		ctx,
		params["ID"].(string),
		params["SortOrder"].(string),
		params["Operator"].(string),
		params["ResetSchedule"].(string),
		params["Metadata"].(map[string]interface{}),
		params["Title"].(string),
		params["Description"].(string),
		int(params["Category"].(float64)),
		int(params["StartTime"].(float64)),
		int(params["EndTime"].(float64)),
		int(params["Duration"].(float64)),
		int(params["MaxSize"].(float64)),
		int(params["MaxNumScore"].(float64)),
		params["JoinRequired"].(bool),
	); err != nil {
		log.Infof("unable to create tournament: %q", err.Error())
		return "", err
	}

	return payload, nil
}
