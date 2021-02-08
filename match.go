package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	nakamaContext "github.com/challenge-league/nakama-go/context"
	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"

	"open-match.dev/open-match/pkg/pb"
)

type Match struct{}

func NewMatch(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule) (runtime.Match, error) {
	return &Match{}, nil
}

func eventSessionStart(ctx context.Context, logger runtime.Logger, evt *api.Event) {
	log.Infof("session start %v %v", ctx, evt)
}

func eventSessionEnd(ctx context.Context, logger runtime.Logger, evt *api.Event) {
	log.Infof("session end %v %v", ctx, evt)
}

func RestoreActiveMatchesAfterRestart(ctx context.Context, nk runtime.NakamaModule) error {
	matchStateList, err := matchStateListGet(ctx, nk, nakamaContext.NakamaSystemUserID, nakamaCommands.MATCH_COLLECTION)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Infof("MatchList %v", matchStateList)

	for _, matchState := range matchStateList {
		if matchState.Active {
			module := nakamaCommands.DEFAULT_NAKAMA_MATCH_MODULE
			params := make(map[string]interface{})
			params["MatchID"] = matchState.MatchID

			if _, err := nk.MatchCreate(
				ctx,
				module,
				params,
			); err != nil {
				log.Error(err)
				return err
			}
			log.Infof("Match %v restored", matchState.MatchID)
		} else {
			log.Infof("Match %v", matchState)
			if err := stopMatch(ctx, nk, matchState); err != nil {
				log.Error(err)
				return err
			}
		}

	}
	return nil
}

func MatchCreateRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var oMMatch *pb.Match
	if err := json.Unmarshal([]byte(payload), &oMMatch); err != nil {
		log.Error(err)
		return "", err
	}

	var matchType string
	err := json.Unmarshal(oMMatch.Extensions[nakamaCommands.MATCH_EXTENSION_MATCH_TYPE].Value, &matchType)
	if err != nil {
		log.Error(err)
		return "", err
	}

	var captains []string
	var teams []*nakamaCommands.Team
	var userIDsReady []string
	tickets := oMMatch.GetTickets()
	for i, t := range tickets {
		teamUser, _ := nakamaCommands.UnmarshalTeamUser(t.Extensions[nakamaCommands.TICKET_EXTENSION_USER].Value)
		teamUser.Captain = true
		teamUser.TicketID = t.Id
		teamUser.Reward = 0

		if teamUser != nil {
			ticketState, err := readTicketState(ctx, nk, t.Id, teamUser.User.Nakama.ID)
			if err != nil {
				log.Error(err)
				return "", err
			}
			if ticketState == nil {
				err := OpenMatchFrontendTicketDelete(t.Id)
				if err != nil {
					log.Error(err)
					return "", err
				}
				return "", fmt.Errorf("No ticket %v found for <@%v>", teamUser.User.Nakama.CustomID)
			}

			/*
				teamAccount, err := AccountByCustomIDGetRPC(ctx, logger, db, nk, teamUser.User.Nakama.CustomID)
				if err != nil {
					log.Error(err)
					return "", err
				}

				if teamUser.User.Discord.ChannelID == "" {
					teamUser.User.Discord.ChannelID = teamAccount
				}
			*/

			captains = append(captains, teamUser.User.Nakama.CustomID)
			teams = append(teams, &nakamaCommands.Team{
				Name:      "",
				TeamUsers: []*nakamaCommands.TeamUser{teamUser},
				ID:        i,
			})

			if ticketState.UserReady {
				userIDsReady = append(userIDsReady, teamUser.User.Nakama.ID)
			}

			log.Infof("Ticket state: %+v", ticketState)
			ticketState.MatchID = oMMatch.MatchId
			err = writeTicketState(ctx, nk, ticketState, teamUser.User.Nakama.ID)
			if err != nil {
				log.Error(err)
				return "", err
			}

		}
	}

	currentTime := time.Now()
	duration := int(tickets[0].SearchFields.DoubleArgs[nakamaCommands.SEARCH_MIN_DURATION])

	rand.Seed(time.Now().Unix())
	matchState := &nakamaCommands.MatchState{
		Debug:             true,
		Active:            true,
		Started:           false,
		MatchID:           oMMatch.MatchId,
		MatchProfile:      oMMatch.MatchProfile,
		MatchType:         matchType,
		CaptainTurnUserID: captains[rand.Intn(len(captains))],
		CaptainUserIDs:    captains,
		Status:            nakamaCommands.MATCH_STATUS_CREATED,
		ReadyUserIDs:      userIDsReady,
		Results:           []*nakamaCommands.MatchResult{},
		Teams:             teams,
		PoolUserIDs:       []string{},
		PoolUserCustomIDs: []string{},
		StorageUserID:     nakamaContext.NakamaSystemUserID,
		StorageCollection: nakamaCommands.MATCH_COLLECTION,
		Version:           "*",
		CancelUserIDs:     []string{},
		DateTimeStart:     currentTime,
		DateTimeEnd:       currentTime,
		Duration:          time.Duration(duration) * time.Hour, //time.Second * time.Duration(100),
	}
	log.Info(duration)
	_, err = writeMatchState(ctx, nk, matchState)
	if err != nil {
		log.Error(err)
		return "", err
	}

	for _, team := range teams {
		for _, teamUser := range team.TeamUsers {
			if err := createOrUpdateLastUserData(ctx, nk, &nakamaCommands.UserData{
				MatchID: matchState.MatchID,
			}, teamUser.User.Nakama.ID); err != nil {
				log.Error(err)
				return "", err
			}
		}
	}

	module := nakamaCommands.DEFAULT_NAKAMA_MATCH_MODULE
	params := make(map[string]interface{})
	params["MatchID"] = oMMatch.MatchId

	if result, err := nk.MatchCreate(
		ctx,
		module,
		params,
	); err != nil {
		log.Error(err)
		return "", err
	} else {
		return result, nil
	}
}

func MatchCancelRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var request *nakamaCommands.MatchCancelRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Error(err)
		return "", err
	}

	account, err := nk.AccountGetId(ctx, request.UserID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	matchState, err := readMatchState(ctx, nk, getDummyMatchState(request.MatchID, nakamaCommands.MATCH_COLLECTION))
	if err != nil {
		log.Error(err)
		return "", err
	}

	if matchState.Started {
		return fmt.Sprintf("Match **%v** is started, **unable to cancel** it!", matchState.MatchID), nil
	}

	if !nakamaCommands.IsUserIDInMatch(account.User.Id, matchState) {
		return fmt.Sprintf("User <@%v> not found in match **%v**", account.User.Id, matchState.MatchID), nil
	}

	matchState.Status = nakamaCommands.MATCH_STATUS_CANCELED
	matchState.Active = false

	if _, err := writeMatchState(ctx, nk, matchState); err != nil {
		log.Error(err)
		return "", err
	}
	if err := deleteTicketsFromMatchState(ctx, nk, matchState); err != nil {
		log.Error(err)
		return "", err
	}
	if !nakamaCommands.IsStringInSlice(request.UserID, matchState.CancelUserIDs) {
		matchState.CancelUserIDs = append(matchState.CancelUserIDs, request.UserID)

		if err := notifyDiscordUsers(
			nakamaCommands.GetUsersFromMatch(matchState),
			fmt.Sprintf("<@%v> is **not** ready for a Match **%v**, match has been canceled", account.CustomId, request.MatchID)); err != nil {
			log.Error(err)
			return "", err
		}
	}

	return "", nil
}

func MatchReadyRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var request *nakamaCommands.MatchReadyRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Error(err)
		return "", err
	}

	account, err := nk.AccountGetId(ctx, request.UserID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	matchState, err := readMatchState(ctx, nk, getDummyMatchState(request.MatchID, nakamaCommands.MATCH_COLLECTION))
	if err != nil {
		log.Error(err)
		return "", err
	}

	if !nakamaCommands.IsUserIDInMatch(account.User.Id, matchState) {
		return "", fmt.Errorf("User <@%v> not found in match **%v**", account.User.Id, matchState.MatchID)
	}

	if !nakamaCommands.IsStringInSlice(request.UserID, matchState.ReadyUserIDs) {
		matchState.ReadyUserIDs = append(matchState.ReadyUserIDs, request.UserID)

		_, err = writeMatchState(ctx, nk, matchState)
		if err != nil {
			log.Error(err)
			return "", err
		}
		if err := notifyDiscordUsers(
			nakamaCommands.GetUsersFromMatch(matchState),
			fmt.Sprintf("<@%v> is ready for a Match **%v**", account.CustomId, request.MatchID)); err != nil {
			log.Error(err)
			return "", err
		}
	} else {
		return fmt.Sprintf("User <@%v> is ready", account.CustomId), nil
	}
	return "", nil
}

func (m *Match) MatchInit(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, params map[string]interface{}) (interface{}, int, string) {
	var MatchID string
	if d, ok := params["MatchID"]; ok {
		if dv, ok := d.(string); ok {
			MatchID = dv
		}
	}
	log.Info(MatchID)
	tickRate := 1
	label := fmt.Sprintf("matchID: %v", MatchID)
	state, err := readMatchState(ctx, nk, getDummyMatchState(MatchID, nakamaCommands.MATCH_COLLECTION))
	if err != nil {
		return nil, tickRate, label
	}

	if err := notifyDiscordNewMatch(state); err != nil {
		//if err := notifyNewMatchEmbed(state); err != nil {
		log.Errorf("Error %+v", err)
	}

	if state.Debug {
		log.Infof("match init, starting with debug: %v", state.Debug)
	}
	return state, tickRate, label
}

func (m *Match) MatchJoinAttempt(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presence runtime.Presence, metadata map[string]string) (interface{}, bool, string) {
	if state.(*nakamaCommands.MatchState).Debug {
		log.Infof("match join attempt username %v user_id %v session_id %v node %v with metadata %v", presence.GetUsername(), presence.GetUserId(), presence.GetSessionId(), presence.GetNodeId(), metadata)
	}

	return state, true, ""
}

func (m *Match) MatchJoin(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	if state.(*nakamaCommands.MatchState).Debug {
		for _, presence := range presences {
			log.Infof("match join username %v user_id %v session_id %v node %v", presence.GetUsername(), presence.GetUserId(), presence.GetSessionId(), presence.GetNodeId())
		}
	}

	return state
}

func (m *Match) MatchLeave(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, presences []runtime.Presence) interface{} {
	if state.(*nakamaCommands.MatchState).Debug {
		for _, presence := range presences {
			log.Infof("match leave username %v user_id %v session_id %v node %v", presence.GetUsername(), presence.GetUserId(), presence.GetSessionId(), presence.GetNodeId())
		}
	}

	return state
}

func stopMatch(ctx context.Context, nk runtime.NakamaModule, s *nakamaCommands.MatchState) error {
	log.Infof("Match state1: %+v", s)
	if err := archiveMatchState(ctx, nk, s); err != nil {
		log.Error(err)
		return err
	}
	log.Infof("Match state2: %+v", s)
	if err := deleteTicketsFromMatchState(ctx, nk, s); err != nil {
		log.Error(err)
		return err
	}
	if err := deleteDiscordChannelsFromMatchState(s); err != nil {
		log.Error(err)
		return err
	}
	if err := deleteMatchState(ctx, nk, s); err != nil {
		log.Errorf("Error: %+v, returning previous state", err)
		return err
	}
	return nil
}

func (m *Match) MatchLoop(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, messages []runtime.MatchData) interface{} {
	s := state.(*nakamaCommands.MatchState)
	var err error
	if s != nil {
		if s, err = readMatchState(ctx, nk, getDummyMatchState(s.MatchID, nakamaCommands.MATCH_COLLECTION)); err != nil {
			log.Errorf("Error: %+v, returning previous state", err)
			return state
		}
	} else {
		log.Error("Match state is nil")
		return nil
	}
	if s.Debug {
		log.Infof("match loop match_id %v tick %v match.Status %v", s.MatchID, tick, s.Status)
	}
	if len(s.CancelUserIDs) > 0 && !s.Started {
		if err := notifyDiscordUsers(
			nakamaCommands.GetUsersFromMatch(s),
			fmt.Sprintf("Match **%v** was canceled", s.MatchID)); err != nil {
			log.Error(err)
		}
		if err := stopMatch(ctx, nk, s); err != nil {
			log.Error(err)
		}
		return nil
	}
	if s.Active != true {
		log.Infof("Match %v is not active", s.MatchID)
		if err := stopMatch(ctx, nk, s); err != nil {
			log.Error(err)
		}
		return nil
	}

	maxUsersCount := nakamaCommands.CAPTAINS_DRAFT_MODES_MAP[s.MatchProfile].TeamCount * nakamaCommands.CAPTAINS_DRAFT_MODES_MAP[s.MatchProfile].UsersInTeam
	readyUsersCount := len(s.ReadyUserIDs)
	if readyUsersCount < maxUsersCount {
		log.Infof("match_id: %v Not all users ready, awaiting for them", s.MatchID)
		log.Infof("ReadyUsersCount: %v, maxUsersCount: %v", readyUsersCount, maxUsersCount)
		if s.Status != nakamaCommands.MATCH_STATUS_AWAITNG_USERS_READY {
			s.Status = nakamaCommands.MATCH_STATUS_AWAITNG_USERS_READY
			s = writeMatchStateInLoop(ctx, nk, s)
		}
		return s
	}

	if s.DateTimeStart.Unix() == s.DateTimeEnd.Unix() {
		if !s.Started {
			if s.MatchType == nakamaCommands.MATCH_TYPE_CAPTAINS_DRAFT {
				log.Infof("match_id: %v Waiting for the draft completion", s.MatchID)
				mode := nakamaCommands.CAPTAINS_DRAFT_MODES_MAP[s.MatchProfile]
				log.Infof("mode: %v ", mode)
				teamUsersCount := len(nakamaCommands.GetTeamUsersFromTeams(s.Teams))
				log.Infof("TeamUsers count: %v, maxUsersCount: %v", teamUsersCount, maxUsersCount)
				if teamUsersCount < maxUsersCount {
					return s
				}
			}

			s, err = createNakamaTournament(ctx, logger, db, nk, s)
			if err != nil {
				log.Error(err)
			}

			if err := createDiscordChannels(s); err != nil {
				log.Error(err)
			}

			s.Started = true
			s.Status = nakamaCommands.MATCH_STATUS_IN_PROGRESS
			msg, err := notifyDiscordChannel(os.Getenv("DISCORD_ANNOUNCEMENTS_MATCH_MAKER_CHANNEL_ID"), nakamaCommands.PrintMatchState(s))
			s.DiscordNewMatchMessage = nakamaCommands.DiscordMessage{ID: msg.ID, ChannelID: msg.ChannelID, GuildID: msg.GuildID}
			if err != nil {
				log.Error(err)
			}
			s = writeMatchStateInLoop(ctx, nk, s)

			if err := deleteTicketsByPoolUserIDs(ctx, nk, s); err != nil {
				log.Error(err)
			}
		}
	}

	if s.DateTimeEnd.Unix() < time.Now().UTC().Unix() {
		winnerTeam, err := getWinnerTeam(ctx, nk, s)
		if err != nil {
			log.Error(err)
		}
		s.Status = nakamaCommands.MATCH_STATUS_ENDED_AFTER_TIME_EXPIRED
		msg := "> The Match **%v** time is over.\n"
		if err := distributeRewardsWithMessage(ctx, nk, winnerTeam, s, msg); err != nil {
			log.Error(err)
		}
		if err := stopMatch(ctx, nk, s); err != nil {
			log.Error(err)
		}
		return nil
	}

	if consensusEstablished, winnerTeam := isResultConsensusEstablished(s); consensusEstablished {
		log.Infof("Consensus established")
		s.Status = nakamaCommands.MATCH_STATUS_COMPLETED_AHEAD_OF_SCHEDULE
		msg := "> The Match **%v** was completed ahead of schedule\n"
		if err := distributeRewardsWithMessage(ctx, nk, winnerTeam, s, msg); err != nil {
			log.Error(err)
		}
		if err := stopMatch(ctx, nk, s); err != nil {
			log.Error(err)
		}
		return nil
	}
	return s
}

func (m *Match) MatchTerminate(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, dispatcher runtime.MatchDispatcher, tick int64, state interface{}, graceSeconds int) interface{} {
	log.Infof("Match will end in " + strconv.Itoa(graceSeconds) + " seconds.")
	if state.(*nakamaCommands.MatchState).Debug {
		log.Infof("match terminate match_id %v tick %v", ctx.Value(runtime.RUNTIME_CTX_MATCH_ID), tick)
		log.Infof("match terminate match_id %v grace seconds %v", ctx.Value(runtime.RUNTIME_CTX_MATCH_ID), graceSeconds)
	}

	return state
}
