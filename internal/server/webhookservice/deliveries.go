package webhookservice

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/webhook"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// TestWebhook sends a ping to the webhook right away, whether it is enabled or not,
// and returns the delivery. A failed ping is not tried again.
func (s *Server) TestWebhook(
	ctx context.Context,
	req *connect.Request[pb.TestWebhookRequest],
) (*connect.Response[pb.TestWebhookResponse], error) {
	hook, err := s.findWebhook(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	delivery, err := webhook.Ping(&hook)
	if err != nil {
		return nil, errors.InternalCause(err, "failed to prepare the ping")
	}

	if err := s.sender.SendNow(ctx, s.db.DB, &hook, &delivery); err != nil {
		return nil, errors.InternalCause(err, "failed to record the ping")
	}

	return connect.NewResponse(&pb.TestWebhookResponse{Delivery: deliveryToProto(&hook, &delivery)}), nil
}

// ListWebhookDeliveries returns a webhook's deliveries, newest first, without their
// payloads and answers
func (s *Server) ListWebhookDeliveries(
	ctx context.Context,
	req *connect.Request[pb.ListWebhookDeliveriesRequest],
) (*connect.Response[pb.ListWebhookDeliveriesResponse], error) {
	if _, err := s.findWebhook(ctx, req.Msg.WebhookId); err != nil {
		return nil, err
	}

	var deliveries []models.WebhookDelivery

	query := s.db.WithContext(ctx).Model(&models.WebhookDelivery{}).Omit("payload", "response_body").
		Where("webhook_id = ?", req.Msg.WebhookId).
		Order("created_at DESC")

	pagination, err := common.Paginate(query, req.Msg.Pagination, &deliveries)
	if err != nil {
		return nil, err
	}

	result := make([]*pb.WebhookDelivery, len(deliveries))
	for i := range deliveries {
		result[i] = converter.WebhookDeliveryToProto(&deliveries[i], false)
	}

	return connect.NewResponse(&pb.ListWebhookDeliveriesResponse{Deliveries: result, Pagination: pagination}), nil
}

// GetWebhookDelivery returns a delivery with its body, as the webhook's format
// renders it now, and the answer
func (s *Server) GetWebhookDelivery(
	ctx context.Context,
	req *connect.Request[pb.GetWebhookDeliveryRequest],
) (*connect.Response[pb.GetWebhookDeliveryResponse], error) {
	delivery, err := s.findDelivery(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	hook, err := s.findWebhook(ctx, delivery.WebhookID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetWebhookDeliveryResponse{Delivery: deliveryToProto(&hook, &delivery)}), nil
}

// RedeliverWebhookDelivery sends a delivery's event again right away, in the
// webhook's current format, as a new delivery that is not tried again should it
// fail. Webhooks that are turned off get it too.
func (s *Server) RedeliverWebhookDelivery(
	ctx context.Context,
	req *connect.Request[pb.RedeliverWebhookDeliveryRequest],
) (*connect.Response[pb.RedeliverWebhookDeliveryResponse], error) {
	original, err := s.findDelivery(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	hook, err := s.findWebhook(ctx, original.WebhookID)
	if err != nil {
		return nil, err
	}

	delivery := webhook.Redelivery(&original)

	if err := s.sender.SendNow(ctx, s.db.DB, &hook, &delivery); err != nil {
		return nil, errors.InternalCause(err, "failed to record the delivery")
	}

	return connect.NewResponse(&pb.RedeliverWebhookDeliveryResponse{Delivery: deliveryToProto(&hook, &delivery)}), nil
}

func (s *Server) findDelivery(ctx context.Context, id string) (models.WebhookDelivery, error) {
	if id == "" {
		return models.WebhookDelivery{}, errors.InvalidInput(errors.ErrInvalidInput, "delivery id is required")
	}

	return common.FindByID[models.WebhookDelivery](s.db.WithContext(ctx), id, errors.ErrWebhookDeliveryNotFound, "webhook delivery not found")
}
