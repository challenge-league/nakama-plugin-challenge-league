module github.com/challenge-league/nakama-plugin-challenge-league/v2

go 1.14

require (
	cloud.google.com/go/firestore v1.1.1 // indirect
	cloud.google.com/go/pubsub v1.3.0 // indirect
	firebase.google.com/go v3.13.0+incompatible
	github.com/766b/go-outliner v0.0.0-20180511142203-fc6edecdadd7 // indirect
	github.com/bwmarrin/discordgo v0.21.1
	github.com/challenge-league/nakama-go/commands v0.0.0-00010101000000-000000000000
	github.com/challenge-league/nakama-go/context v0.0.0-00010101000000-000000000000
	github.com/envoyproxy/go-control-plane v0.9.4 // indirect
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/golang/mock v1.4.3 // indirect
	github.com/golang/protobuf v1.4.1
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.14.3 // indirect
	github.com/hako/durafmt v0.0.0-20200710122514-c0fb7b4da026 // indirect
	github.com/heroiclabs/nakama-common v1.5.1
	github.com/heroiclabs/nakama/v2/apigrpc v0.0.0-00010101000000-000000000000 // indirect
	github.com/jackc/pgx v3.5.0+incompatible
	github.com/micro/go-micro/v2 v2.7.0
	github.com/prometheus/common v0.7.0
	go.uber.org/zap v1.14.1
	golang.org/x/crypto v0.0.0-20200323165209-0ec3e9974c59
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	golang.org/x/tools v0.0.0-20200505023115-26f46d2f7ef8 // indirect
	google.golang.org/api v0.20.0
	google.golang.org/genproto v0.0.0-20200325114520-5b2d0af7952b // indirect
	google.golang.org/grpc v1.27.1
	open-match.dev/open-match v1.0.0
)

replace (
	github.com/challenge-league/nakama-go/commands => ./nakama-go/commands
	github.com/challenge-league/nakama-go/context => ./nakama-go/context
	github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2
	github.com/grpc-ecosystem/grpc-gateway => github.com/grpc-ecosystem/grpc-gateway v1.14.3
	github.com/heroiclabs/nakama/v2/apigrpc => ./apigrpc
)
