package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vanzheng/kodaclaw-community/internal/model"
	"github.com/vanzheng/kodaclaw-community/internal/repository"
)

// notifiedExpiringKeys 记录已发送"即将过期"通知的 key ID 及通知时间
// 内存存储，重启后重置（最差情况多通知一次，可接受）
var notifiedExpiringKeys sync.Map // key: string(keyID), value: time.Time

// CheckExpiringWebhookKeys 检查即将过期或已过期的 webhook key 并发送通知
func CheckExpiringWebhookKeys(ctx context.Context, notifSvc *NotificationService, webhookKeyRepo repository.WebhookKeyRepository) {
	now := time.Now()
	before := now.Add(24 * time.Hour)

	// 构建已通知的 key 集合（24h 内已通知的"即将过期" key 不重复通知）
	notifiedIDs := make(map[string]bool)
	notifiedExpiringKeys.Range(func(k, v any) bool {
		notifiedAt, ok := v.(time.Time)
		if ok && time.Since(notifiedAt) < 24*time.Hour {
			notifiedIDs[k.(string)] = true
		} else {
			notifiedExpiringKeys.Delete(k)
		}
		return true
	})

	keys, err := webhookKeyRepo.GetExpiringKeys(ctx, before, notifiedIDs)
	if err != nil {
		log.Printf("[webhook_key_checker] GetExpiringKeys error: %v", err)
		return
	}

	for _, k := range keys {
		userID, err := uuid.Parse(k.UserID)
		if err != nil {
			log.Printf("[webhook_key_checker] invalid user_id %q for key %s: %v", k.UserID, k.ID, err)
			continue
		}

		var notifType, title string
		var msg string
		if k.ExpiresAt != nil && !k.ExpiresAt.After(now) {
			// 已过期：每次检查都通知（无去重）
			notifType = "webhook_key_expired"
			title = "Webhook 密钥已过期"
			msg = fmt.Sprintf("密钥「%s」（%s）已于 %s 过期，实例：%s",
				k.KeyName, k.KeyPrefix, k.ExpiresAt.Format("2006-01-02 15:04:05"), k.InstanceName)
		} else {
			// 即将过期（24h 内）：标记已通知，避免重复
			notifType = "webhook_key_expiring"
			title = "Webhook 密钥即将过期"
			msg = fmt.Sprintf("密钥「%s」（%s）将于 %s 过期，实例：%s",
				k.KeyName, k.KeyPrefix, k.ExpiresAt.Format("2006-01-02 15:04:05"), k.InstanceName)
			notifiedExpiringKeys.Store(k.ID, time.Now())
		}

		msgCopy := msg
		n := &model.Notification{
			UserID:  userID,
			Type:    notifType,
			Title:   title,
			Message: &msgCopy,
		}
		if err := notifSvc.CreateAndNotify(ctx, n); err != nil {
			log.Printf("[webhook_key_checker] CreateAndNotify error for key %s: %v", k.ID, err)
		}
	}
}
