package auth

import (
	"context"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	pb "gitlab.com/home-server7795544/home-server/iam/iam-backend/grpc/auth"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/handler/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

type AuthServiceServer struct {
	DB                   *pgxpool.Pool
	JWTSecret            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	pb.UnimplementedAuthServiceServer
}

func (s *AuthServiceServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// Validate input
	if req.Username == "" || req.AppCode == "" || req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "BadRequest")
	}

	// Generate JWT
	response, err := auth.GenerateJWTForUser(
		s.DB,
		req.Username,
		req.Password,
		req.AppCode,
		s.JWTSecret,
		s.AccessTokenDuration,
		s.RefreshTokenDuration,
		false,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	// Convert roles from JWT response
	roles := []*pb.Role{}
	if jwtRoles, ok := response["jwtBody"].(jwt.MapClaims)["roles"].([]auth.Role); ok {
		for _, role := range jwtRoles {
			roles = append(roles, &pb.Role{
				RoleCode:   role.RoleCode,
				RoleNameTh: getString(role.RoleNameTh),
				RoleNameEn: getString(role.RoleNameEn),
				Objects:    role.Objects,
			})
		}
	}

	// Construct JwtBody
	jwtBodyClaims := response["jwtBody"].(jwt.MapClaims)
	jwtBody := &pb.JwtBody{
		UserId:      getString(jwtBodyClaims["userId"]),
		FirstNameTh: getString(jwtBodyClaims["firstNameTh"]),
		LastNameTh:  getString(jwtBodyClaims["lastNameTh"]),
		AppCode:     getString(jwtBodyClaims["appCode"]),
		CompanyCode: getString(jwtBodyClaims["companyCode"]),
		AccountName: getString(jwtBodyClaims["accountName"]),
		Status:      getString(jwtBodyClaims["status"]),
		Roles:       roles,
	}

	// Construct response
	return &pb.LoginResponse{
		AccessToken:  getString(response["accessToken"]),
		RefreshToken: getString(response["refreshToken"]),
		JwtBody:      jwtBody,
	}, nil
}

func getString(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case *string:
		if v != nil {
			return *v
		}
	}
	return ""
}
