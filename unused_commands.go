package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"

	common_api "github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func StorageWriteRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	log.Print(payload)
	//var params map[string]interface{}
	//json.Unmarshal([]byte(payload), &params)
	var storageWrite []*runtime.StorageWrite
	if err := json.Unmarshal([]byte(payload), &storageWrite); err != nil {
		log.Printf("unable to create tournament: %q", err.Error())
		return "", err
	}
	log.Printf("%+v", string(MarshalIndent(storageWrite)))

	result, err := nk.StorageWrite(ctx, storageWrite)

	if err != nil {
		log.Print(err)
		return "", err
	}

	if len(result) != 1 {
		log.Printf("Invocation failed. Return result not expected: ", len(result))
		return "", err
	}

	return MarshalIndent(result), nil
}

func TournamentDeleteRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return "", err
	}

	if err := nk.TournamentDelete(ctx, params["ID"].(string)); err != nil {
		return "", err
	}
	return "", nil
}

func LeaderboardCreateRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return "", err
	}

	if err := nk.LeaderboardCreate(
		ctx,
		params["ID"].(string),
		params["Authoritative"].(bool),
		params["SortOrder"].(string),
		params["Operator"].(string),
		params["ResetSchedule"].(string),
		params["Metadata"].(map[string]interface{}),
	); err != nil {
		return "", err
	}
	return "", nil
}

func LeaderboardDeleteRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return "", err
	}

	if err := nk.LeaderboardDelete(ctx, params["ID"].(string)); err != nil {
		return "", err
	}
	return "", nil

}

func LeaderboardRecordWriteRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return "", err
	}

	//score, _ := strconv.ParseInt(params["Score"].(string), 10, 64)
	//subscore, _ := strconv.ParseInt(params["Subscore"].(string), 10, 64)
	//int64(params["Subscore"].(float64)),
	if result, err := nk.LeaderboardRecordWrite(
		ctx,
		params["ID"].(string),
		params["OwnerID"].(string),
		params["Username"].(string),
		int64(params["Score"].(float64)),
		int64(params["Subscore"].(float64)),
		map[string]interface{}{},
	); err != nil {
		return "", err
	} else {
		return MarshalIndent(result), err
	}
}

func LeaderboardRecordDelete(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return "", err
	}

	if err := nk.LeaderboardRecordDelete(
		ctx,
		params["ID"].(string),
		params["OwnerID"].(string),
	); err != nil {
		return "", err
	} else {
		return "", nil
	}
}

func LeaderboardRecordList(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (*common_api.LeaderboardRecordList, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return nil, err
	}

	if records, ownerRecords, nextCursorStr, prevCursorStr, err := nk.LeaderboardRecordsList(
		ctx,
		params["ID"].(string),
		params["OwnerIDs"].([]string),
		params["Limit"].(int),
		params["Cursor"].(string),
		params["Expiry"].(int64),
	); err != nil {
		return nil, err
	} else {
		return &common_api.LeaderboardRecordList{Records: records, OwnerRecords: ownerRecords, NextCursor: nextCursorStr, PrevCursor: prevCursorStr}, nil
	}
}
