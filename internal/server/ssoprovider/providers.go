package ssoprovider

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// checkTimeout limits how long TestSsoProvider waits for the issuer.
const checkTimeout = 20 * time.Second

// ListSsoProviders returns all sign-in providers by name
func (s *Server) ListSsoProviders(
	ctx context.Context,
	req *connect.Request[pb.ListSsoProvidersRequest],
) (*connect.Response[pb.ListSsoProvidersResponse], error) {
	var providers []models.SSOProvider
	if err := s.db.WithContext(ctx).Order("name").Find(&providers).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to list sign-in providers")
	}

	result := make([]*pb.SsoProvider, len(providers))
	for i := range providers {
		result[i] = converter.SSOProviderToProto(&providers[i])
	}

	return connect.NewResponse(&pb.ListSsoProvidersResponse{SsoProviders: result}), nil
}

// CreateSsoProvider adds a sign-in provider
func (s *Server) CreateSsoProvider(
	ctx context.Context,
	req *connect.Request[pb.CreateSsoProviderRequest],
) (*connect.Response[pb.CreateSsoProviderResponse], error) {
	name, err := validateName(req.Msg.Name)
	if err != nil {
		return nil, err
	}

	issuer, err := validateIssuer(req.Msg.IssuerUrl)
	if err != nil {
		return nil, err
	}

	clientID, err := validateClientID(req.Msg.ClientId)
	if err != nil {
		return nil, err
	}

	roleID, err := defaultRole(s.db.WithContext(ctx), req.Msg.GetDefaultRoleId())
	if err != nil {
		return nil, err
	}

	clientSecret, err := s.encryptSecret(req.Msg.ClientSecret)
	if err != nil {
		return nil, err
	}

	provider := models.SSOProvider{
		Name:          name,
		IssuerURL:     issuer,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		DefaultRoleID: roleID,
		Enabled:       req.Msg.Enabled,
		CreatedBy:     middleware.UserIDPtr(ctx),
	}

	if err := s.db.WithContext(ctx).Create(&provider).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to create sign-in provider")
	}

	return connect.NewResponse(&pb.CreateSsoProviderResponse{SsoProvider: converter.SSOProviderToProto(&provider)}), nil
}

// UpdateSsoProvider changes the given settings of a sign-in provider
func (s *Server) UpdateSsoProvider(
	ctx context.Context,
	req *connect.Request[pb.UpdateSsoProviderRequest],
) (*connect.Response[pb.UpdateSsoProviderResponse], error) {
	provider, err := s.findProvider(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if req.Msg.Name != nil {
		if provider.Name, err = validateName(req.Msg.GetName()); err != nil {
			return nil, err
		}
	}

	if req.Msg.IssuerUrl != nil {
		if provider.IssuerURL, err = validateIssuer(req.Msg.GetIssuerUrl()); err != nil {
			return nil, err
		}
	}

	if req.Msg.ClientId != nil {
		if provider.ClientID, err = validateClientID(req.Msg.GetClientId()); err != nil {
			return nil, err
		}
	}

	if req.Msg.ClientSecret != nil {
		if provider.ClientSecret, err = s.encryptSecret(req.Msg.GetClientSecret()); err != nil {
			return nil, err
		}
	}

	if req.Msg.DefaultRoleId != nil {
		if provider.DefaultRoleID, err = defaultRole(s.db.WithContext(ctx), req.Msg.GetDefaultRoleId()); err != nil {
			return nil, err
		}
	}

	if req.Msg.Enabled != nil {
		provider.Enabled = req.Msg.GetEnabled()
	}

	provider.UpdatedBy = middleware.UserIDPtr(ctx)

	if err := s.db.WithContext(ctx).Save(&provider).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to update sign-in provider")
	}

	return connect.NewResponse(&pb.UpdateSsoProviderResponse{SsoProvider: converter.SSOProviderToProto(&provider)}), nil
}

// DeleteSsoProvider removes a sign-in provider and its links to users. The users
// stay, but those without a password can't sign in until an admin gives them one.
func (s *Server) DeleteSsoProvider(
	ctx context.Context,
	req *connect.Request[pb.DeleteSsoProviderRequest],
) (*connect.Response[pb.DeleteSsoProviderResponse], error) {
	provider, err := s.findProvider(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, model := range []any{&models.SSOIdentity{}, &models.SSOLogin{}} {
			if err := tx.Where("provider_id = ?", provider.ID).Delete(model).Error; err != nil {
				return err
			}
		}

		return tx.Delete(&provider).Error
	})
	if err != nil {
		return nil, errors.InternalCause(err, "failed to delete sign-in provider")
	}

	return connect.NewResponse(&pb.DeleteSsoProviderResponse{}), nil
}

// TestSsoProvider fetches the provider's OpenID configuration from its issuer, and
// reports what went wrong
func (s *Server) TestSsoProvider(
	ctx context.Context,
	req *connect.Request[pb.TestSsoProviderRequest],
) (*connect.Response[pb.TestSsoProviderResponse], error) {
	provider, err := s.findProvider(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	if err := s.check(ctx, provider.IssuerURL); err != nil {
		return connect.NewResponse(&pb.TestSsoProviderResponse{Error: err.Error()}), nil
	}

	return connect.NewResponse(&pb.TestSsoProviderResponse{Ok: true}), nil
}
