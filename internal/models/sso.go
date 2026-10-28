package models

import "time"

// SSOProvider is an OpenID Connect provider users sign in with.
type SSOProvider struct {
	ID        string `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name      string `gorm:"type:varchar(100);not null" json:"name"`
	IssuerURL string `gorm:"type:text;not null" json:"issuer_url"`
	ClientID  string `gorm:"type:varchar(255);not null" json:"client_id"`
	// ClientSecret is encrypted with the server's encryption key; empty for public clients.
	ClientSecret string `gorm:"type:text" json:"-"`
	// DefaultRoleID is the role of accounts created at someone's first sign-in.
	// Without one, only existing users can sign in.
	DefaultRoleID *string   `gorm:"type:uuid" json:"default_role_id,omitempty"`
	Enabled       bool      `gorm:"not null" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CreatedBy     *string   `gorm:"type:uuid" json:"created_by,omitempty"`
	UpdatedBy     *string   `gorm:"type:uuid" json:"updated_by,omitempty"`
}

func (SSOProvider) TableName() string {
	return "sso_providers"
}

// SSOIdentity links a user to their account at a provider, so later sign-ins find
// them even after their email changes.
type SSOIdentity struct {
	ID         string       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	ProviderID string       `gorm:"type:uuid;not null;uniqueIndex:idx_sso_identities_subject,priority:1" json:"provider_id"`
	Provider   *SSOProvider `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	// Subject is the provider's id of the account, the "sub" claim.
	Subject string `gorm:"type:varchar(255);not null;uniqueIndex:idx_sso_identities_subject,priority:2" json:"subject"`
	UserID  string `gorm:"type:uuid;not null;index" json:"user_id"`
	User    *User  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	// Email is the address the provider gave at the last sign-in.
	Email       string    `gorm:"type:varchar(255);not null;default:''" json:"email"`
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt time.Time `json:"last_login_at"`
}

func (SSOIdentity) TableName() string {
	return "sso_identities"
}

// SSOLogin is a sign-in in progress, from leaving for the provider until coming
// back with a code. Each is used once.
type SSOLogin struct {
	State      string `gorm:"type:varchar(64);primaryKey" json:"state"`
	ProviderID string `gorm:"type:uuid;not null" json:"provider_id"`
	Nonce      string `gorm:"type:varchar(64);not null" json:"-"`
	// CodeVerifier proves to the provider that the code comes back to who asked for it (PKCE).
	CodeVerifier string    `gorm:"type:varchar(128);not null" json:"-"`
	RedirectURI  string    `gorm:"type:text;not null" json:"redirect_uri"`
	ExpiresAt    time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func (SSOLogin) TableName() string {
	return "sso_logins"
}
