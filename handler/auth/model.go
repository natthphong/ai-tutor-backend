package auth

import "time"

type LoginRequest struct {
	AppCode  string `json:"appCode"`  // The application code
	Username string `json:"username"` // The user's ID (username)
	Password string `json:"password"` // The user's password
}

type User struct {
	UserID              string      `json:"userId"`              // User ID
	FirstNameTh         *string     `json:"firstNameTh"`         // First name in Thai
	FirstNameEn         *string     `json:"firstNameEn"`         // First name in English (optional)
	MidNameTh           *string     `json:"midNameTh"`           // Middle name in Thai (optional)
	MidNameEn           *string     `json:"midNameEn"`           // Middle name in English (optional)
	LastNameTh          *string     `json:"lastNameTh"`          // Last name in Thai
	LastNameEn          *string     `json:"lastNameEn"`          // Last name in English (optional)
	Phone               *string     `json:"phone"`               // Phone number (optional)
	UserIDType          *string     `json:"userIdType"`          // User ID type (optional)
	Email               *string     `json:"email"`               // Email (optional)
	Nationality         *string     `json:"nationality"`         // Nationality (optional)
	Occupation          *string     `json:"occupation"`          // Occupation (optional)
	RequestRef          *string     `json:"requestRef"`          // Request reference (optional)
	BirthDate           *time.Time  `json:"birthDate"`           // Birth date (optional)
	Gender              *string     `json:"gender"`              // Gender (optional)
	TaxID               *string     `json:"taxId"`               // Tax ID (optional)
	SecondEmail         *string     `json:"secondEmail"`         // Second email (optional)
	OccupationOtherDesc *string     `json:"occupationOtherDesc"` // Other occupation description (optional)
	IsActive            *string     `json:"isActive"`            // Active status (optional)
	Password            string      `json:"-"`                   // Password (excluded from JSON output)
	BranchCode          *string     `json:"branchCode"`          // Branch code (optional)
	AppCode             string      `json:"appCode"`             // Application code
	CompanyCode         *string     `json:"companyCode"`         // Company code (optional)
	Status              string      `json:"status"`              // User status
	AccountName         *string     `json:"accountName"`         // Account name (optional)
	UserActiveTime      *time.Time  `json:"userActiveTime"`      // User active time (optional)
	ExternalID          *string     `json:"externalId"`          // External ID (optional)
	UserDetails         interface{} `json:"userDetails"`         // Additional user details (optional)
	InActive            bool        `json:"inActive"`            // Inactive status
}

type AccessTokenClaims struct {
	UserID      string  `json:"userId"`      // User ID
	FirstNameTh string  `json:"firstNameTh"` // First name in Thai
	LastNameTh  string  `json:"lastNameTh"`  // Last name in Thai
	AppCode     string  `json:"appCode"`     // Application code
	CompanyCode *string `json:"companyCode"` // Company code (optional)
	AccountName *string `json:"accountName"` // Account name (optional)
	Status      string  `json:"status"`      // User status
	Exp         int64   `json:"exp"`         // Expiry timestamp
}
type RefreshTokenClaims struct {
	UserID string `json:"userId"` // User ID
	Exp    int64  `json:"exp"`    // Expiry timestamp
}
type Role struct {
	RoleCode   string   `json:"roleCode"`
	RoleNameTh string   `json:"roleNameTh"`
	RoleNameEn string   `json:"roleNameEn"`
	Objects    []string `json:"objects"` // List of object codes
}
