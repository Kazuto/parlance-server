package webhookservice

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListWebhooks returns all webhooks by name, each with its newest delivery and the
// events it collected
func (s *Server) ListWebhooks(
	ctx context.Context,
	req *connect.Request[pb.ListWebhooksRequest],
) (*connect.Response[pb.ListWebhooksResponse], error) {
	var hooks []models.Webhook
	if err := s.db.WithContext(ctx).Order("name").Find(&hooks).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to list webhooks")
	}

	result, err := s.webhooksToProto(ctx, hooks)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListWebhooksResponse{Webhooks: result}), nil
}

// CreateWebhook adds a webhook
func (s *Server) CreateWebhook(
	ctx context.Context,
	req *connect.Request[pb.CreateWebhookRequest],
) (*connect.Response[pb.CreateWebhookResponse], error) {
	name, err := validateName(req.Msg.Name)
	if err != nil {
		return nil, err
	}

	address, err := validateURL(req.Msg.Url)
	if err != nil {
		return nil, err
	}

	events, err := eventsFromProto(req.Msg.Events)
	if err != nil {
		return nil, err
	}

	format, err := formatFromProto(req.Msg.Format)
	if err != nil {
		return nil, err
	}

	schedule, err := validateSchedule(req.Msg.Schedule)
	if err != nil {
		return nil, err
	}

	secret, err := s.encryptSecret(req.Msg.Secret)
	if err != nil {
		return nil, err
	}

	hook := models.Webhook{
		Name:      name,
		URL:       address,
		Events:    events,
		Enabled:   req.Msg.Enabled,
		Format:    format,
		Schedule:  schedule,
		Secret:    secret,
		CreatedBy: middleware.UserIDPtr(ctx),
	}

	if hook.NextDigestAt, err = webhook.NextDigest(&hook, time.Now()); err != nil {
		return nil, errors.InternalCause(err, "failed to plan the digest")
	}

	if err := s.db.WithContext(ctx).Create(&hook).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to create webhook")
	}

	return connect.NewResponse(&pb.CreateWebhookResponse{Webhook: converter.WebhookToProto(&hook, nil)}), nil
}

// UpdateWebhook changes the given settings of a webhook. Turning it off fails the
// deliveries still pending and the events it collected; removing its schedule
// sends what it collected as a digest right away.
func (s *Server) UpdateWebhook(
	ctx context.Context,
	req *connect.Request[pb.UpdateWebhookRequest],
) (*connect.Response[pb.UpdateWebhookResponse], error) {
	hook, err := s.findWebhook(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if req.Msg.Name != nil {
		if hook.Name, err = validateName(req.Msg.GetName()); err != nil {
			return nil, err
		}
	}

	if req.Msg.Url != nil {
		if hook.URL, err = validateURL(req.Msg.GetUrl()); err != nil {
			return nil, err
		}
	}

	if req.Msg.Events != nil {
		if hook.Events, err = eventsFromProto(req.Msg.Events.Events); err != nil {
			return nil, err
		}
	}

	if req.Msg.Format != nil {
		if hook.Format, err = formatFromProto(req.Msg.GetFormat()); err != nil {
			return nil, err
		}
	}

	if req.Msg.Schedule != nil {
		if hook.Schedule, err = validateSchedule(req.Msg.GetSchedule()); err != nil {
			return nil, err
		}
	}

	if req.Msg.Secret != nil {
		if hook.Secret, err = s.encryptSecret(req.Msg.GetSecret()); err != nil {
			return nil, err
		}
	}

	if req.Msg.Enabled != nil {
		hook.Enabled = req.Msg.GetEnabled()
	}

	hook.UpdatedBy = middleware.UserIDPtr(ctx)

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := planDigest(tx, &hook, time.Now()); err != nil {
			return err
		}

		if err := tx.Save(&hook).Error; err != nil {
			return err
		}

		if hook.Enabled {
			return nil
		}

		return webhook.FailPending(tx, hook.ID)
	})
	if err != nil {
		return nil, errors.InternalCause(err, "failed to update webhook")
	}

	return connect.NewResponse(&pb.UpdateWebhookResponse{Webhook: s.webhookToProto(ctx, &hook)}), nil
}

// planDigest sets when the webhook's next digest goes out: on its schedule while it
// is on, and right away when it is on without a schedule but still holds events
// collected under its last one.
func planDigest(tx *gorm.DB, hook *models.Webhook, now time.Time) error {
	next, err := webhook.NextDigest(hook, now)
	if err != nil {
		return err
	}

	hook.NextDigestAt = next

	if !hook.Enabled || hook.Schedule != "" {
		return nil
	}

	var collected int64
	if err := tx.Model(&models.WebhookDelivery{}).
		Where("webhook_id = ? AND status = ?", hook.ID, models.DeliveryCollected).
		Count(&collected).Error; err != nil {
		return err
	}

	if collected > 0 {
		hook.NextDigestAt = &now
	}

	return nil
}

// DeleteWebhook removes a webhook and its deliveries. A delivery being sent is not
// recorded.
func (s *Server) DeleteWebhook(
	ctx context.Context,
	req *connect.Request[pb.DeleteWebhookRequest],
) (*connect.Response[pb.DeleteWebhookResponse], error) {
	hook, err := s.findWebhook(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("webhook_id = ?", hook.ID).Delete(&models.WebhookDelivery{}).Error; err != nil {
			return err
		}

		return tx.Delete(&hook).Error
	})
	if err != nil {
		return nil, errors.InternalCause(err, "failed to delete webhook")
	}

	return connect.NewResponse(&pb.DeleteWebhookResponse{}), nil
}
