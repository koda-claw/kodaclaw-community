package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/relay"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

type NotificationService struct {
	notificationRepo repository.NotificationRepository
	relayRepo        repository.RelayInstanceRepository
	hub              *relay.Hub
}

func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	relayRepo repository.RelayInstanceRepository,
	hub *relay.Hub,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		relayRepo:        relayRepo,
		hub:              hub,
	}
}

// CreateAndNotify 创建通知并推送给在线的 Relay 客户端。
// 通知入库失败时返回 error；推送失败时只记日志，不影响主流程。
func (s *NotificationService) CreateAndNotify(ctx context.Context, notification *model.Notification) error {
	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		return err
	}

	if s.hub == nil || s.relayRepo == nil {
		return nil
	}

	instances, err := s.relayRepo.ListRelayInstancesByUserID(ctx, notification.UserID.String())
	if err != nil {
		log.Printf("[NotificationService] list relay instances for user %s: %v", notification.UserID, err)
		return nil
	}

	type metadataPayload struct {
		ID             string  `json:"id"`
		Type           string  `json:"type"`
		Title          string  `json:"title"`
		Message        *string `json:"message,omitempty"`
		RelatedAssetID *string `json:"relatedAssetId,omitempty"`
		CreatedAt      string  `json:"createdAt"`
	}

	meta := metadataPayload{
		ID:        notification.ID.String(),
		Type:      notification.Type,
		Title:     notification.Title,
		Message:   notification.Message,
		CreatedAt: notification.CreatedAt.Format(time.RFC3339),
	}
	if notification.RelatedAssetID != nil {
		assetID := notification.RelatedAssetID.String()
		meta.RelatedAssetID = &assetID
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		log.Printf("[NotificationService] marshal metadata: %v", err)
		return nil
	}

	frame := relay.EventFrame{
		Type:              relay.FrameTypeEvent,
		EventType:         "NotificationReceived",
		ThreadType:        "DirectMessage",
		EventID:           notification.ID.String(),
		ExternalThreadID:  "community:notifications",
		ExternalMessageID: "community-notification:" + notification.ID.String(),
		Sender: relay.EventSender{
			ID:          "community",
			DisplayName: "KodaClaw 社区",
			IsBot:       true,
		},
		Text:          notification.Title,
		OccurredAt:    notification.CreatedAt,
		CorrelationID: notification.ID.String(),
		MetadataJSON:  string(metaBytes),
	}

	for _, instance := range instances {
		if result := s.hub.OnEvent(instance.AccountID, frame); result != relay.DeliveryQueued {
			log.Printf("[NotificationService] failed to push event to account %s: %s", instance.AccountID, result)
		}
	}

	return nil
}
