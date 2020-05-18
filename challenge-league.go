package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gofrs/uuid"
	"github.com/heroiclabs/nakama-common/runtime"
)

func TournamentCreate(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	logger.Print("RUNNING IN GO")
	id := uuid.Must(uuid.NewV4())

	sortOrder := "desc"           // one of: "desc", "asc"
	operator := "best"            // one of: "best", "set", "incr"
	resetSchedule := "0 12 * * *" // noon UTC each day
	metadata := map[string]interface{}{}
	title := "Daily Dash"
	description := "Dash past your opponents for high scores and big rewards!"
	category := 1
	startTime := int(time.Now().UTC().Unix()) // start now
	endTime := 0                              // never end, repeat the tournament each day forever
	duration := 3600                          // in seconds
	maxSize := 10000                          // first 10,000 players who join
	maxNumScore := 3                          // each player can have 3 attempts to score
	joinRequired := true                      // must join to compete
	err := nk.TournamentCreate(ctx, id.String(), sortOrder, operator, resetSchedule, metadata, title,
		description, category, startTime, endTime, duration, maxSize, maxNumScore, joinRequired)
	if err != nil {
		logger.Printf("unable to create tournament: %q", err.Error())
		return "", runtime.NewError("failed to create tournament", 3)
	}

	return payload, nil
}

func MatchJoin(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	params := make(map[string]interface{})
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return "", err
	}

	modulename := "pingpong" // Name with which match handler was registered in InitModule, see example above.
	if matchId, err := nk.MatchCreate(ctx, modulename, params); err != nil {
		return "", err
	} else {
		return matchId, nil
	}
}

func MatchCreate(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	params := make(map[string]interface{})
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return "", err
	}

	modulename := "pingpong" // Name with which match handler was registered in InitModule, see example above.
	if matchId, err := nk.MatchCreate(ctx, modulename, params); err != nil {
		return "", err
	} else {
		return matchId, nil
	}
}
