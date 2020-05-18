package main

import (
	"context"
	"database/sql"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/rtapi"
	"github.com/heroiclabs/nakama-common/runtime"
)

func NewMatch(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule) (runtime.Match, error) {
	return &Match{}, nil
}

func beforeChannelJoin(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, envelope *rtapi.Envelope) (*rtapi.Envelope, error) {
	logger.Printf("Intercepted request to join channel '%v'", envelope.GetChannelJoin().Target)
	return envelope, nil
}

func afterGetAccount(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, in *api.Account) error {
	logger.Printf("Intercepted response to get account '%v'", in)
	return nil
}

func eventSessionStart(ctx context.Context, logger runtime.Logger, evt *api.Event) {
	logger.Printf("session start %v %v", ctx, evt)
}

func eventSessionEnd(ctx context.Context, logger runtime.Logger, evt *api.Event) {
	logger.Printf("session end %v %v", ctx, evt)
}

type MatchState struct {
	debug bool
}

type Match struct{}

func (m *Match) MatchInit(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, params map[string]interface{}) (interface{}, int, string) {
	var debug bool
	if d, ok := params["debug"]; ok {
		if dv, ok := d.(bool); ok {
			debug = dv
		}
	}
	state := &MatchState{
		debug: debug,
	}

	if state.debug {
		logger.Printf("match init, starting with debug: %v", state.debug)
	}
	tickRate := 1
	label := "skill=100-150"

	return state, tickRate, label
}

func (m *Match) MatchJoinAttempt(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presence runtime.Presence, metadata map[string]string) (interface{}, bool, string) {
	if state.(*MatchState).debug {
		logger.Printf("match join attempt username %v user_id %v session_id %v node %v with metadata %v", presence.GetUsername(), presence.GetUserId(), presence.GetSessionId(), presence.GetNodeId(), metadata)
	}

	return state, true, ""
}

func (m *Match) MatchJoin(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	if state.(*MatchState).debug {
		for _, presence := range presences {
			logger.Printf("match join username %v user_id %v session_id %v node %v", presence.GetUsername(), presence.GetUserId(), presence.GetSessionId(), presence.GetNodeId())
		}
	}

	return state
}

func (m *Match) MatchLeave(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	if state.(*MatchState).debug {
		for _, presence := range presences {
			logger.Printf("match leave username %v user_id %v session_id %v node %v", presence.GetUsername(), presence.GetUserId(), presence.GetSessionId(), presence.GetNodeId())
		}
	}

	return state
}

func (m *Match) MatchLoop(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, messages []runtime.MatchData) interface{} {
	if state.(*MatchState).debug {
		logger.Printf("match loop match_id %v tick %v", ctx.Value(runtime.RUNTIME_CTX_MATCH_ID), tick)
		logger.Printf("match loop match_id %v message count %v", ctx.Value(runtime.RUNTIME_CTX_MATCH_ID), len(messages))
	}

	//if tick >= 10 {
	//	return nil
	//}
	return state
}

func (m *Match) MatchTerminate(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, graceSeconds int) interface{} {
	if state.(*MatchState).debug {
		logger.Printf("match terminate match_id %v tick %v", ctx.Value(runtime.RUNTIME_CTX_MATCH_ID), tick)
		logger.Printf("match terminate match_id %v grace seconds %v", ctx.Value(runtime.RUNTIME_CTX_MATCH_ID), graceSeconds)
	}

	return state
}

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
		logger.Error("failed to update winner wallets: %v", err)
		return err
	}
	err = nk.NotificationsSend(ctx, notifications)
	if err == nil {
		logger.Error("failed to send winner notifications: %v", err)
		return err
	}
	return nil
}
