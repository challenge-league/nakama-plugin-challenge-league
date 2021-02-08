package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
	log "github.com/micro/go-micro/v2/logger"
)

const (
	//CONSENSUS_RATIO = 0.51
	CONSENSUS_RATIO = 0.001
)

func updateMatchResults(result *nakamaCommands.MatchResult, matchState *nakamaCommands.MatchState) []*nakamaCommands.MatchResult {
	var newResults []*nakamaCommands.MatchResult
	for _, v := range matchState.Results {
		if v.UserID != result.UserID {
			newResults = append(newResults, v)
		}
	}
	return append(newResults, result)
}

func isMatchResultExist(result *nakamaCommands.MatchResult, matchState *nakamaCommands.MatchState) bool {
	for _, v := range matchState.Results {
		if v.UserID == result.UserID &&
			v.DiscordID == result.DiscordID &&
			v.Win == result.Win &&
			v.Draw == result.Draw &&
			v.TeamNumber == result.TeamNumber &&
			v.ProofLink == result.ProofLink {
			return true
		}
	}

	return false
}

func isResultConsensusEstablished(s *nakamaCommands.MatchState) (isConsensusEstablished bool, team *nakamaCommands.Team) {
	if float64(len(s.Results))/float64(len(s.Teams)*len(s.Teams[0].TeamUsers)) >= float64(CONSENSUS_RATIO) {
		teamResults := make(map[int]int)
		for i := 0; i < len(s.Teams); i++ {
			teamResults[i] = 0
		}
		drawCount := 0
		for _, v := range s.Results {
			if v.Draw {
				drawCount++
			}

			if v.Win {
				teamResults[v.TeamNumber] = teamResults[v.TeamNumber] + 1
			} else {
				teamResults[v.TeamNumber] = teamResults[v.TeamNumber] - 1
			}
		}

		if drawCount > 0 {
			if float64(len(s.Results))/float64(drawCount) >= float64(CONSENSUS_RATIO) {
				return true, nil
			}
		}

		var ss []*nakamaCommands.TeamResult
		for k, v := range teamResults {
			ss = append(ss, &nakamaCommands.TeamResult{k, v})
		}

		sort.Slice(ss, func(i, j int) bool {
			return ss[i].Votes > ss[j].Votes
		})

		if ss[0].Votes > ss[1].Votes {
			return true, s.Teams[ss[0].TeamNumber]
		}
	}
	return false, nil
}

func MatchResultRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var request *nakamaCommands.MatchResultRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Error(err)
		return "", err
	}

	account, err := nk.AccountGetId(ctx, request.MatchResult.UserID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	matchState, err := readMatchState(ctx, nk, getDummyMatchState(request.MatchID, nakamaCommands.MATCH_COLLECTION))
	if err != nil {
		log.Error(err)
		return "", err
	}

	if !matchState.Started {
		return fmt.Sprintf("Match **%v** is not started, unable to add the result. Please indicate that you are **ready** for the match or **cancel** it.", matchState.MatchID), nil
	}

	if !nakamaCommands.IsUserIDInMatch(account.User.Id, matchState) {
		return "", fmt.Errorf("User <@%v> not found in match **%v**", account.User.Id, matchState.MatchID)
	}

	if !request.MatchResult.Draw && request.MatchResult.TeamNumber == -1 {
		request.MatchResult.TeamNumber = nakamaCommands.GetTeamNumberFromUserAndMatch(account.User.Id, matchState)
	}

	if !isMatchResultExist(request.MatchResult, matchState) {
		request.MatchResult.DateTime = time.Now().UTC()
		matchState.Results = updateMatchResults(request.MatchResult, matchState)

		_, err = writeMatchState(ctx, nk, matchState)
		if err != nil {
			log.Error(err)
			return "", err
		}

		if request.MatchResult.Draw {
			msg := "<@%v> reported result **Draw** for a Match **%v**"
			if request.MatchResult.ProofLink != "" {
				msg = msg + fmt.Sprintf(" with the proof link: %v", request.MatchResult.ProofLink)
			}
			if err := notifyDiscordUsers(nakamaCommands.GetUsersFromMatch(matchState),
				fmt.Sprintf(
					msg,
					account.CustomId,
					request.MatchID,
				)); err != nil {
				log.Error(err)
				return "", err
			}
		} else {
			msg := "<@%v> reported **Team %v** **%v** result for a Match **%v**"
			result := "Win"
			if !request.MatchResult.Win {
				result = "Lose"
			}
			if request.MatchResult.ProofLink != "" {
				msg = msg + fmt.Sprintf(" with the proof link: %v", request.MatchResult.ProofLink)
			}
			if err := notifyDiscordUsers(nakamaCommands.GetUsersFromMatch(matchState),
				fmt.Sprintf(
					msg,
					account.CustomId,
					request.MatchResult.TeamNumber,
					result,
					request.MatchID,
				)); err != nil {
				log.Error(err)
				return "", err
			}
		}
	} else {
		return "The result already exists", nil
	}
	return "", nil
}
