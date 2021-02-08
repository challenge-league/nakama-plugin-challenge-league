package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	common_api "github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/api/option"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"github.com/gofrs/uuid"
)

var (
	ErrBadContext    = runtime.NewError("bad context", 3)
	ErrJsonMarshal   = runtime.NewError("cannot marshal response", 13)
	ErrJsonUnmarshal = runtime.NewError("cannot unmarshal request", 13)
)

type SessionContext struct {
	UserID    string
	SessionID string
}

func getFirebaseUserByEmail(client *auth.Client, email string) (*auth.UserRecord, error) {
	ctx := context.Background()
	user, err := client.GetUserByEmail(ctx, email)
	if user == nil || auth.IsUserNotFound(err) {
		return nil, err
	}
	log.Printf("user %+v", user)
	return user, nil
}

func getFirebaseUserByUID(client *auth.Client, uid string) (*auth.UserRecord, error) {
	ctx := context.Background()
	user, err := client.GetUser(ctx, uid)
	if user == nil || auth.IsUserNotFound(err) {
		return nil, err
	}
	log.Printf("user %+v", user)
	return user, nil
}

func isFirebaseUserVerified(user *auth.UserRecord) error {
	if !user.EmailVerified {
		return errors.New("User email not verified, please sign in with email on https://dataleague.org")
	}

	/* TODO: FIX
	if user.PhoneNumber == "" {
		return errors.New("User phone number is not verified, please complete second factor authorization on https://dataleague.org")
	}
	*/

	return nil

}

func isFirebaseUserActive(user *auth.UserRecord) error {
	if user.Disabled {
		return errors.New("User not verified, please sign in with email on https://dataleague.org")
	}
	return nil
}

func unpackContext(ctx context.Context) (*SessionContext, error) {
	log.Print(ctx)
	userId, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok {
		return nil, ErrBadContext
	}
	sessionId, ok := ctx.Value(runtime.RUNTIME_CTX_SESSION_ID).(string)
	if !ok {
		return nil, ErrBadContext
	}
	sessionContext := &SessionContext{UserID: userId, SessionID: sessionId}
	log.Print(MarshalIndent(sessionContext))
	return sessionContext, nil
}

func resetVerificationNakamaAccount(ctx context.Context, db *sql.DB, userID string) error {
	var params []interface{}
	query := fmt.Sprintf("UPDATE users SET verify_time = '1970-01-01 00:00:00 UTC' WHERE id = '%v'", userID)
	if _, err := db.ExecContext(ctx, query, params...); err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func verifyNakamaAccount(ctx context.Context, db *sql.DB, userID string) error {
	var params []interface{}
	query := fmt.Sprintf("UPDATE users SET verify_time = now() WHERE id = '%v'", userID)
	if _, err := db.ExecContext(ctx, query, params...); err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func getNakamaAccountToUpdateJson(account *common_api.Account, authenticateCustomRequest *common_api.AuthenticateCustomRequest) (string, error) {
	nakamaAccountToUpdate := map[string]interface{}{}
	nakamaAccountToUpdate["AvatarUrl"] = ""
	if val, ok := authenticateCustomRequest.Account.Vars["Author.AvatarUrl"]; ok {
		nakamaAccountToUpdate["AvatarUrl"] = val
	}
	nakamaAccountToUpdate["LangTag"] = ""
	if val, ok := authenticateCustomRequest.Account.Vars["Author.Locale"]; ok {
		nakamaAccountToUpdate["LangTag"] = val
	}
	nakamaAccountToUpdate["ID"] = account.User.Id
	nakamaAccountToUpdate["Username"] = authenticateCustomRequest.Username
	nakamaAccountToUpdate["Metadata"] = authenticateCustomRequest.Account.Vars
	nakamaAccountToUpdate["DisplayName"] = authenticateCustomRequest.Username
	nakamaAccountToUpdate["Timezone"] = ""
	nakamaAccountToUpdate["Location"] = ""
	nakamaAccountToUpdateJSON, err := json.Marshal(nakamaAccountToUpdate)
	if err != nil {
		log.Print(err)
		return "", err
	}
	return string(nakamaAccountToUpdateJSON), nil
}

func firstLoginWithFirebase(ctx context.Context, db *sql.DB, nk runtime.NakamaModule, authenticateCustomRequest *common_api.AuthenticateCustomRequest, account *common_api.Account, firebaseCredential string) error {
	if isEmailValid(firebaseCredential) {
		if err := nk.LinkEmail(ctx, account.User.Id, firebaseCredential, uuid.Must(uuid.NewV4()).String()); err != nil {
			log.Print(err)
			return err
		}
	}
	// Verify account, set verify_time
	verifyNakamaAccount(ctx, db, account.User.Id)
	nakamaAccountToUpdateJSON, err := getNakamaAccountToUpdateJson(account, authenticateCustomRequest)
	if err != nil {
		log.Print(err)
		return err
	}
	if _, err := AccountUpdateIDRPC(ctx, nil, nil, nk, string(nakamaAccountToUpdateJSON)); err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func firstLoginWithFirebase2(ctx context.Context, db *sql.DB, nk runtime.NakamaModule, authenticateCustomRequest *common_api.AuthenticateCustomRequest, account *common_api.Account, firebaseCredential string) error {
	opt := option.WithCredentialsFile("dataleague-org-2846b61502d2.json")
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Print(err)
		return err
	}
	client, err := app.Auth(context.Background())
	if err != nil {
		log.Print(err)
		return err
	}
	var user *auth.UserRecord
	if isEmailValid(firebaseCredential) {
		user, err = getFirebaseUserByEmail(client, firebaseCredential)
	} else {
		log.Print("provided firebase credential is not email, try login by uuid")
		user, err = getFirebaseUserByUID(client, firebaseCredential)
	}
	if err != nil {
		log.Print(err)
		return err
	}
	if err := isFirebaseUserActive(user); err != nil {
		log.Print(err)
		return err
	}
	if err := isFirebaseUserVerified(user); err != nil {
		log.Print(err)
		return err
	}
	// Try to link email before the user update to check if email is already in use
	if err := nk.LinkEmail(ctx, account.User.Id, user.Email, uuid.Must(uuid.NewV4()).String()); err != nil {
		log.Print(err)
		return err
	}

	// Try to link phone number before the user update to check if phone number is already in use
	// Workaround to pass phone number from firebase to nakama through the PhotoURL field
	//if err := nk.LinkDevice(ctx, account.User.Id, user.PhotoURL); err != nil {

	/* TODO: FIX
	if err := nk.LinkDevice(ctx, account.User.Id, user.PhoneNumber); err != nil {
		log.Print(err)
		return err
	}
	*/

	/*
		// Workaround to set phone number in firebase from nakama through the PhotoURL field
		if _, err := client.UpdateUser(context.Background(), user.UID, firebaseUserToUpdate.PhoneNumber(user.PhotoURL)); err != nil {
			log.Print(err)
			return err
		}
	*/

	// Verify account, set verify_time
	verifyNakamaAccount(ctx, db, account.User.Id)

	// Update firebase PhotoURL from the discord PhotoURL
	if val, ok := authenticateCustomRequest.Account.Vars["Author.AvatarUrl"]; ok {
		firebaseUserToUpdate := &auth.UserToUpdate{}
		client.UpdateUser(context.Background(), user.UID, firebaseUserToUpdate.PhotoURL(val))
	}

	nakamaAccountToUpdateJSON, err := getNakamaAccountToUpdateJson(account, authenticateCustomRequest)
	if err != nil {
		log.Print(err)
		return err
	}
	if _, err := AccountUpdateIDRPC(ctx, nil, nil, nk, string(nakamaAccountToUpdateJSON)); err != nil {
		log.Print(err)
		return err
	}
	return nil
}

func isEmailValid(email string) bool {
	Re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]+$`)
	return Re.MatchString(email)
}

func beforeAuthenticateCustom(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, in *common_api.AuthenticateCustomRequest) (*common_api.AuthenticateCustomRequest, error) {
	log.Printf("%+v", in)
	userID, username, new, err := nk.AuthenticateCustom(ctx, in.Account.Id, in.Username, true)
	log.Printf("%+v %+v %+v %+v", userID, username, new, err)

	if in.Account.Id == "administrator" {
		return in, nil
	}

	account, err := nk.AccountGetId(ctx, userID)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	if account.DisableTime != nil {
		log.Print("account disabled")
		return nil, errors.New("account disabled")
	}

	/*
		if val, ok := in.Account.Vars["Content"]; ok {
			args := strings.Split(val, " ")
			if args[0] == "login" || args[0] == "l" {
				if account.VerifyTime != nil {
					log.Print("account already verified")
					return nil, errors.New("account already verified")
				}
				if len(args) != 2 {
					log.Print("login email or UID is missing")
					return nil, errors.New("login email or UID is missing")
				}
				return in, firstLoginWithFirebase(ctx, db, nk, in, account, args[1])
			}
			//if len(args) != 2 {
			//	log.Print("login email or UID is missing")
			//	return nil, errors.New("login email or UID is missing")
			//}
			//return in, firstLoginWithFirebase(ctx, db, nk, in, account, args[1])
		}
	*/

	if account.VerifyTime == nil {
		return in, firstLoginWithFirebase(ctx, db, nk, in, account, in.Username)
		//return nil, errors.New(`account not verified, please complete registration at **https://dataleague.org** and use the following commands: **dl login your-email@example.com** or **dl login your-uid**`)
		//return nil, errors.New(`account not verified, please use the following commands: **rm login your-email@example.com** or **rm login your-uid**`)
	}

	if account.User.Username != in.Username {
		nakamaAccountToUpdateJSON, err := getNakamaAccountToUpdateJson(account, in)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		if _, err := AccountUpdateIDRPC(ctx, nil, nil, nk, string(nakamaAccountToUpdateJSON)); err != nil {
			log.Print(err)
			return nil, err
		}
	}

	return in, nil
}

func UserDataFromSession(session *common_api.Session) (map[string]interface{}, error) {
	parts := strings.Split(session.Token, ".")
	content, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal(content, &data)
	if err != nil {
		return nil, err
	}
	log.Printf("DECODE %v", data)
	return data, err
}

func AccountUpdateIDRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return "", err
	}

	if err := nk.AccountUpdateId(
		ctx,
		params["ID"].(string),
		params["Username"].(string),
		params["Metadata"].(map[string]interface{}),
		params["DisplayName"].(string),
		params["Timezone"].(string),
		params["Location"].(string),
		params["LangTag"].(string),
		params["AvatarUrl"].(string),
	); err != nil {
		return "", err
	} else {
		return "", nil
	}
}
