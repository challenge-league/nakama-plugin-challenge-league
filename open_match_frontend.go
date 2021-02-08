package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"log"

	nakamaCommands "github.com/challenge-league/nakama-go/commands"
	"github.com/heroiclabs/nakama-common/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"open-match.dev/open-match/pkg/pb"
)

const (
	// The endpoint for the Open Match Frontend service.
	omFrontendEndpoint = "open-match-frontend.open-match.svc.cluster.local:50504"
)

var (
	openMatchFrontendServiceClientBuilder *openMatchFrontendServiceClient
	once                                  sync.Once
	kacp                                  = keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}
)

type openMatchFrontendServiceClient struct {
	conn   *grpc.ClientConn
	client pb.FrontendServiceClient
}

func NewOpenMatchFrontEndSingleton() *openMatchFrontendServiceClient {
	once.Do(func() {
		// Connect to Open Match Frontend.
		conn, err := grpc.Dial(omFrontendEndpoint, grpc.WithInsecure(), grpc.WithKeepaliveParams(kacp))
		if err != nil {
			log.Fatalf("Failed to connect to Open Match, got %v", err)
		}

		//defer conn.Close()

		openMatchFrontendServiceClientBuilder = &openMatchFrontendServiceClient{
			conn:   conn,
			client: pb.NewFrontendServiceClient(conn),
		}
	})
	//log.Printf("DKOZLOV %+v", openMatchFrontendServiceClientBuilder.conn.GetState().String())
	return openMatchFrontendServiceClientBuilder
}

func (b *openMatchFrontendServiceClient) SetClient(client pb.FrontendServiceClient) *openMatchFrontendServiceClient {
	b.client = client
	return b
}

func (b *openMatchFrontendServiceClient) GetClient() pb.FrontendServiceClient {
	return b.client
}

func (b *openMatchFrontendServiceClient) SetConn(conn *grpc.ClientConn) *openMatchFrontendServiceClient {
	b.conn = conn
	return b
}

func (b *openMatchFrontendServiceClient) GetConn() *grpc.ClientConn {
	return b.conn
}

func OpenMatchFrontendTicketCreateRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var ticketCreateRequest pb.CreateTicketRequest
	json.Unmarshal([]byte(payload), &ticketCreateRequest)
	log.Printf(MarshalIndent(ticketCreateRequest))

	// TODO: Allow only limited number of tickets per user

	fe := NewOpenMatchFrontEndSingleton().GetClient()
	resp, err := fe.CreateTicket(context.Background(), &ticketCreateRequest)
	if err != nil {
		log.Printf("Failed to Create Ticket, got %s", err.Error())
		return "", err
	}

	var user *nakamaCommands.User
	json.Unmarshal(resp.Extensions["user"].Value, &user)
	log.Printf("Ticket created successfully, id: %v", resp.Id)
	return MarshalIndent(resp), nil
}

func OpenMatchFrontendTicketGetRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var ticketGetRequest pb.GetTicketRequest
	json.Unmarshal([]byte(payload), &ticketGetRequest)
	log.Printf(MarshalIndent(ticketGetRequest))

	fe := NewOpenMatchFrontEndSingleton().GetClient()
	got, err := fe.GetTicket(context.Background(), &pb.GetTicketRequest{TicketId: ticketGetRequest.TicketId})
	if err != nil {
		log.Printf("Failed to Get Ticket %v, got %s", ticketGetRequest.TicketId, err.Error())
		return "", err
	}

	if got.GetAssignment() != nil {
		log.Printf("Ticket %v got assignment %v", got.GetId(), got.GetAssignment())
	}

	return MarshalIndent(got), nil
}

func OpenMatchFrontendTicketDelete(ticketID string) error {
	fe := NewOpenMatchFrontEndSingleton().GetClient()
	_, err := fe.DeleteTicket(context.Background(), &pb.DeleteTicketRequest{TicketId: ticketID})
	if err != nil {
		log.Printf("Failed to Delete Ticket %v from OpenMatchFrontend, got %s", ticketID, err.Error())
		return err
	}
	return nil
}

func OpenMatchFrontendTicketDeleteRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var ticketDeleteRequest pb.DeleteTicketRequest
	json.Unmarshal([]byte(payload), &ticketDeleteRequest)
	log.Printf(MarshalIndent(ticketDeleteRequest))

	err := OpenMatchFrontendTicketDelete(ticketDeleteRequest.TicketId)
	if err != nil {
		return "", err
	}

	return "", nil
}

func OpenMatchFrontendTicketWatchAssignmentsRPC(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var watchAssignmentsRequest *pb.WatchAssignmentsRequest
	json.Unmarshal([]byte(payload), &watchAssignmentsRequest)
	log.Printf(MarshalIndent(watchAssignmentsRequest))

	fe := NewOpenMatchFrontEndSingleton().GetClient()
	watchAssignmentsClient, err := fe.WatchAssignments(context.Background(), &pb.WatchAssignmentsRequest{TicketId: watchAssignmentsRequest.TicketId})
	if err != nil {
		log.Printf("Failed to watch assignments request %v, got %s", watchAssignmentsRequest.TicketId, err.Error())
		return "", err
	}

	watchAssignmentsResponse, err := watchAssignmentsClient.Recv()
	if err != nil {
		log.Printf("Failed to get assignments response %v, got %s", watchAssignmentsResponse.String, err.Error())
		return "", err
	}
	return MarshalIndent(watchAssignmentsResponse), nil
}

func init() {
	NewOpenMatchFrontEndSingleton()
}
