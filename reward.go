package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
)

func getWinnerTeam(ctx context.Context, nk runtime.NakamaModule, matchState *nakamaCommands.MatchState) (*nakamaCommands.Team, error) {
	log.Infof("%+v", matchState)
	userIDs := []string{}
	for _, team := range matchState.Teams {
		for _, teamUser := range team.TeamUsers {
			userIDs = append(userIDs, teamUser.User.Nakama.ID)
		}
	}
	records, _, _, _, err := nk.LeaderboardRecordsList(ctx, matchState.MatchID, userIDs, nakamaCommands.MAX_LIST_LIMIT, "", 0)
	log.Infof("%+v", records)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	metadata := make(map[string]interface{})
	if err := json.Unmarshal([]byte(records[0].Metadata), &metadata); err != nil {
		log.Error(err)
		return nil, err
	}
	if v, ok := metadata["initialSubmit"]; ok && v.(bool) {
		return nil, nil
	}
	teamNumber := nakamaCommands.GetTeamNumberFromUserAndMatch(records[0].OwnerId, matchState)
	return matchState.Teams[teamNumber], nil
}

func distributeRewardsWithMessage(ctx context.Context, nk runtime.NakamaModule, winnerTeam *nakamaCommands.Team, matchState *nakamaCommands.MatchState, msg string) error {
	if winnerTeam != nil {
		if err := distributeRewards(ctx, nk, winnerTeam, matchState); err != nil {
			log.Error(err)
		}
		msg = fmt.Sprintf(msg+"The **winner** team is \n", matchState.MatchID) + nakamaCommands.PrintTeam(winnerTeam)
	} else {
		msg = fmt.Sprintf(msg+"The result of the match is a **Draw**\n", matchState.MatchID)
	}
	if err := notifyDiscordUsers(nakamaCommands.GetUsersFromMatch(matchState), msg); err != nil {
		log.Error(err)
	}
	if _, err := notifyDiscordChannel(os.Getenv("DISCORD_ANNOUNCEMENTS_RESULTS_CHANNEL_ID"), nakamaCommands.PrintMatchState(matchState)); err != nil {
		log.Error(err)
	}
	_, err := writeMatchState(ctx, nk, matchState)
	if err != nil {
		log.Error(err)
	}
	return nil
}

func distributeRewards(ctx context.Context, nk runtime.NakamaModule, winnerTeam *nakamaCommands.Team, matchState *nakamaCommands.MatchState) error {
	var walletUpdates []*runtime.WalletUpdate
	for _, v := range winnerTeam.TeamUsers {
		//changeset := map[string]int64{"coins": int64(100)}
		changeset := map[string]interface{}{"coins": float64(100)}
		metadata := map[string]interface{}{"MatchID": matchState.MatchID}
		walletUpdates = append(walletUpdates, &runtime.WalletUpdate{
			UserID:    v.User.Nakama.ID,
			Changeset: changeset,
			Metadata:  metadata,
		})
	}
	//_, err := nk.WalletsUpdate(ctx, walletUpdates, true)
	err := nk.WalletsUpdate(ctx, walletUpdates, true)
	if err != nil {
		log.Errorf("failed to update winner wallets: %v", err)
		return err
	}

	for _, team := range matchState.Teams {
		for _, teamUser := range team.TeamUsers {
			metadata := make(map[string]interface{})
			metadata["winner"] = "false"
			score := int64(0)
			subscore := int64(0)
			if team.ID == winnerTeam.ID {
				metadata["winner"] = "true"
				score = int64(100)
				subscore = int64(0)
				teamUser.Reward = float64(100)
			}
			if _, err := nk.LeaderboardRecordWrite(
				ctx,
				nakamaCommands.MAIN_LEADERBOARD,
				teamUser.User.Nakama.ID,
				teamUser.User.Nakama.CustomID,
				score,
				subscore,
				metadata,
			); err != nil {
				log.Errorf("failed to update winner wallets: %v", err)
				return err

			}
		}
	}

	return nil
}

/*
func distributeRewards(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, tournament *api.Tournament, end int64, reset int64) error {
	wallets := make([]*runtime.WalletUpdate, 0, 10)
	notifications := make([]*runtime.NotificationSend, 0, 10)
	content := map[string]interface{}{}
	changeset := map[string]interface{}{"coins": 100}
	records, _, _, _, err := nk.LeaderboardRecordsList(ctx, tournament.Id, []string{}, 10, "", reset)
	for _, record := range records {
		wallets = append(wallets, &runtime.WalletUpdate{record.OwnerId, changeset, content})
		notifications = append(notifications, &runtime.NotificationSend{record.OwnerId, "Winner", content, 1, "", true})
	}
	err = nk.WalletsUpdate(ctx, wallets, false)
	if err == nil {
		log.Errorf("failed to update winner wallets: %v", err)
		return err
	}
	err = nk.NotificationsSend(ctx, notifications)
	if err == nil {
		log.Errorf("failed to send winner notifications: %v", err)
		return err
	}
	return nil
}
*/
