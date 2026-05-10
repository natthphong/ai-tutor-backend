package auth

import (
	"context"
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/home-server7795544/home-server/iam/iam-backend/api"
	"time"
)

func GenerateJWTForUser(
	db *pgxpool.Pool,
	userID, password, appCode string,
	jwtSecret string,
	accessTokenDuration, refreshTokenDuration time.Duration,
	refreshTokenFlag bool,
) (map[string]interface{}, error) {
	// 1. Find user by appCode and userId
	var user User
	query := `SELECT user_id, first_name_th, first_name_en, mid_name_th, mid_name_en, last_name_th, last_name_en,
					email, nationality, occupation, birth_date, gender, tax_id, second_email, occupation_other_desc,
					is_active, branchCode, appCode, companyCode, status, account_name, user_active_time, external_id,
					in_active, password
				  FROM tbl_user
				  WHERE user_id = $1 AND appCode = $2 AND is_delete = 'N'`
	err := db.QueryRow(context.Background(), query, userID, appCode).Scan(
		&user.UserID, &user.FirstNameTh, &user.FirstNameEn, &user.MidNameTh, &user.MidNameEn, &user.LastNameTh,
		&user.LastNameEn, &user.Email, &user.Nationality, &user.Occupation, &user.BirthDate, &user.Gender,
		&user.TaxID, &user.SecondEmail, &user.OccupationOtherDesc, &user.IsActive, &user.BranchCode, &user.AppCode,
		&user.CompanyCode, &user.Status, &user.AccountName, &user.UserActiveTime, &user.ExternalID, &user.InActive,
		&user.Password,
	)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if user.InActive {
		return nil, errors.New("user is inactive")
	}
	if !refreshTokenFlag {
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			return nil, errors.New("invalid password")
		}
	}

	// 2. Retrieve roles and objects for the user
	rolesQuery := `
		SELECT tr.role_code, tr.role_name_th, tr.role_name_en, tro.object_code
		FROM tbl_user_role tur
		LEFT JOIN tbl_role tr ON tur.role_code = tr.role_code
		LEFT JOIN tbl_role_object tro ON tr.role_code = tro.role_code
		LEFT JOIN tbl_object toj on tro.object_code = toj.object_code
		WHERE tur.user_id = $1 AND tur.appCode = $2
		and tur.is_delete = 'N' and tr.is_delete='N' and tro.is_delete='N' and toj.is_delete='N'
		ORDER BY tr.role_code;
	`
	rows, err := db.Query(context.Background(), rolesQuery, userID, appCode)
	if err != nil {
		return nil, errors.New("Failed to retrieve roles")
	}
	defer rows.Close()

	roleMap := make(map[string]*Role)
	for rows.Next() {
		var objectCode *string
		var roleCode, roleNameTh, roleNameEn string
		if err := rows.Scan(&roleCode, &roleNameTh, &roleNameEn, &objectCode); err != nil {
			return nil, errors.New("Failed to scan roles")
		}

		if _, exists := roleMap[roleCode]; !exists {
			roleMap[roleCode] = &Role{
				RoleCode:   roleCode,
				RoleNameTh: roleNameTh,
				RoleNameEn: roleNameEn,
				Objects:    []string{},
			}
		}
		if objectCode != nil {
			roleMap[roleCode].Objects = append(roleMap[roleCode].Objects, *objectCode)
		}

	}

	// Convert roleMap to a slice of roles
	roles := make([]Role, 0, len(roleMap))
	for _, role := range roleMap {
		roles = append(roles, *role)
	}

	// 3. Generate JWT tokens
	// Access Token
	accessTokenClaims := jwt.MapClaims{
		"userId":      user.UserID,
		"firstNameTh": user.FirstNameTh,
		"lastNameTh":  user.LastNameTh,
		"appCode":     user.AppCode,
		"companyCode": user.CompanyCode,
		"accountName": user.AccountName,
		"status":      user.Status,
		"roles":       roles, // Add roles to the JWT
		"exp":         time.Now().Add(accessTokenDuration).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString([]byte(jwtSecret))
	if err != nil {
		return nil, errors.New("Failed to generate access token")
	}

	// Refresh Token
	refreshTokenClaims := jwt.MapClaims{
		"userId":  user.UserID,
		"appCode": user.AppCode,
		"exp":     time.Now().Add(refreshTokenDuration).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(jwtSecret))
	if err != nil {
		return nil, errors.New("Failed to generate refresh token")
	}

	// Response
	response := map[string]interface{}{
		"accessToken":  accessTokenString,
		"refreshToken": refreshTokenString,
		"jwtBody":      accessTokenClaims,
	}
	return response, nil
}

func LoginHandler(db *pgxpool.Pool, jwtSecret string, accessTokenDuration, refreshTokenDuration time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req LoginRequest
		if err := c.BodyParser(&req); err != nil {
			return api.BadRequest(c, "Invalid input")
		}

		// Call GenerateJWTForUser
		response, err := GenerateJWTForUser(db, req.Username, req.Password, req.AppCode, jwtSecret, accessTokenDuration, refreshTokenDuration, false)
		if err != nil {
			return api.InternalError(c, err.Error())
		}

		return api.Ok(c, response)
	}
}
