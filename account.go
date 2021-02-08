package main

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"context"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	log "github.com/micro/go-micro/v2/logger"

	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
	"github.com/jackc/pgx/pgtype"
	"go.uber.org/zap"
)

func AccountByUsernameGetRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var accountGetRequest *nakamaCommands.AccountGetRequest
	json.Unmarshal([]byte(payload), &accountGetRequest)

	users, err := nk.UsersGetUsername(ctx, []string{accountGetRequest.Identifier})
	if err != nil {
		log.Error(err)
		return "", err
	}
	if len(users) == 0 {
		log.Error(err)
		return "", fmt.Errorf("No users found with identifier %v", accountGetRequest.Identifier)
	}
	account, err := nk.AccountGetId(ctx, users[0].Id)
	if err != nil {
		log.Error(err)
		return "", err
	}

	return string(Marshal(account)), nil
}

func AccountByCustomIDGetRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var accountGetRequest *nakamaCommands.AccountGetRequest
	json.Unmarshal([]byte(payload), &accountGetRequest)

	var displayName sql.NullString
	var username sql.NullString
	var avatarURL sql.NullString
	var langTag sql.NullString
	var location sql.NullString
	var timezone sql.NullString
	var metadata sql.NullString
	var wallet sql.NullString
	var email sql.NullString
	var facebook sql.NullString
	var facebookInstantGame sql.NullString
	var google sql.NullString
	var gamecenter sql.NullString
	var steam sql.NullString
	var userID uuid.UUID
	var edgeCount int
	var createTime pgtype.Timestamptz
	var updateTime pgtype.Timestamptz
	var verifyTime pgtype.Timestamptz
	var disableTime pgtype.Timestamptz
	var deviceIDs pgtype.VarcharArray

	query := `
SELECT u.username, u.display_name, u.avatar_url, u.lang_tag, u.location, u.timezone, u.metadata, u.wallet,
	u.email, u.facebook_id, u.facebook_instant_game_id, u.google_id, u.gamecenter_id, u.steam_id, u.id, u.edge_count,
	u.create_time, u.update_time, u.verify_time, u.disable_time, array(select ud.id from user_device ud where u.id = ud.user_id)
FROM users u
WHERE u.custom_id = $1`

	if err := db.QueryRowContext(ctx, query, accountGetRequest.Identifier).Scan(&username, &displayName, &avatarURL, &langTag, &location, &timezone, &metadata, &wallet, &email, &facebook, &facebookInstantGame, &google, &gamecenter, &steam, &userID, &edgeCount, &createTime, &updateTime, &verifyTime, &disableTime, &deviceIDs); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("Account not found, customID: %v", accountGetRequest.Identifier)
		}
		logger.Error("Error retrieving user account.", zap.Error(err))
		return "", err
	}

	devices := make([]*api.AccountDevice, 0, len(deviceIDs.Elements))
	for _, deviceID := range deviceIDs.Elements {
		devices = append(devices, &api.AccountDevice{Id: deviceID.String})
	}

	var verifyTimestamp *timestamp.Timestamp
	if verifyTime.Status == pgtype.Present && verifyTime.Time.Unix() != 0 {
		verifyTimestamp = &timestamp.Timestamp{Seconds: verifyTime.Time.Unix()}
	}
	var disableTimestamp *timestamp.Timestamp
	if disableTime.Status == pgtype.Present && disableTime.Time.Unix() != 0 {
		disableTimestamp = &timestamp.Timestamp{Seconds: disableTime.Time.Unix()}
	}

	return string(Marshal(&api.Account{
		User: &api.User{
			Id:                    userID.String(),
			Username:              username.String,
			DisplayName:           displayName.String,
			AvatarUrl:             avatarURL.String,
			LangTag:               langTag.String,
			Location:              location.String,
			Timezone:              timezone.String,
			Metadata:              metadata.String,
			FacebookId:            facebook.String,
			FacebookInstantGameId: facebookInstantGame.String,
			GoogleId:              google.String,
			GamecenterId:          gamecenter.String,
			SteamId:               steam.String,
			EdgeCount:             int32(edgeCount),
			CreateTime:            &timestamp.Timestamp{Seconds: createTime.Time.Unix()},
			UpdateTime:            &timestamp.Timestamp{Seconds: updateTime.Time.Unix()},
			Online:                false,
		},
		Wallet:      wallet.String,
		Email:       email.String,
		Devices:     devices,
		CustomId:    accountGetRequest.Identifier,
		VerifyTime:  verifyTimestamp,
		DisableTime: disableTimestamp,
	})), nil
}
