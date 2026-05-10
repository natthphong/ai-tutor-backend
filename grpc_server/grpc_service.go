package grpc_server

import (
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	gauth "gitlab.com/home-server7795544/home-server/iam/iam-backend/grpc/auth"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/grpc_server/auth"
	"google.golang.org/grpc"
	"net"
	"time"
)

func StartGRPCServer(db *pgxpool.Pool, jwtSecret string, accessTokenDuration, refreshTokenDuration time.Duration) {
	lis, err := net.Listen("tcp", ":8081")
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()
	gauth.RegisterAuthServiceServer(grpcServer, &auth.AuthServiceServer{
		DB:                   db,
		JWTSecret:            jwtSecret,
		AccessTokenDuration:  accessTokenDuration,
		RefreshTokenDuration: refreshTokenDuration,
	})

	fmt.Println("Starting gRPC server on port 8081")
	if err := grpcServer.Serve(lis); err != nil {
		panic(err)
	}
}
