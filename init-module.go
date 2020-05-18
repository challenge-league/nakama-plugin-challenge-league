package main

import (
	"context"
	"database/sql"

	"github.com/heroiclabs/nakama-common/runtime"
)

func InitModule(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) error {
	if err := initializer.RegisterRpc("MatchJoin", MatchJoin); err != nil {
		return err
	}

	if err := initializer.RegisterRpc("TournamentCreate", TournamentCreate); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("MatchCreate", MatchCreate); err != nil {
		return err
	}

	//// Examples
	// Register as match handler, this call should be in InitModule.
	if err := initializer.RegisterMatch("pingpong", func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule) (runtime.Match, error) {
		return &Match{}, nil
	}); err != nil {
		logger.Error("Unable to register: %v", err)
		return err
	}

	if err := initializer.RegisterBeforeRt("ChannelJoin", beforeChannelJoin); err != nil {
		return err
	}
	if err := initializer.RegisterAfterGetAccount(afterGetAccount); err != nil {
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

	if err := initializer.RegisterTournamentEnd(distributeRewards); err != nil {
		return err
	}

	logger.Printf("nakama-plugin-challenge-league.so module loaded")
	return nil
}
