package main

import (
	"context"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
	"github.com/prometheus/common/log"
)

func CreateLeaderboardsIfNotExist(ctx context.Context, nk runtime.NakamaModule) error {
	if err := nk.LeaderboardCreate(
		ctx,
		nakamaCommands.MAIN_LEADERBOARD,
		true,
		"desc",
		"incr",
		"",
		make(map[string]interface{}),
	); err != nil {
		log.Error(err)
	}
	return nil
}
