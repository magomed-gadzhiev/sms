package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/handlers"
)

// Scheduler опрашивает campaign-service и создаёт уведомления о завершённых рассылках
type Scheduler struct {
	pool           *pgxpool.Pool
	campaignClient campaignv1.CampaignServiceClient
	ticker         *time.Ticker
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewScheduler создает новый Scheduler
func NewScheduler(pool *pgxpool.Pool, campaignClient campaignv1.CampaignServiceClient) *Scheduler {
	return &Scheduler{
		pool:           pool,
		campaignClient: campaignClient,
		stopCh:         make(chan struct{}),
	}
}

// Start запускает планировщик
func (s *Scheduler) Start() {
	s.ticker = time.NewTicker(5 * time.Minute)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Первый запуск сразу
		s.run()
		for {
			select {
			case <-s.ticker.C:
				s.run()
			case <-s.stopCh:
				return
			}
		}
	}()
	log.Info().Msg("уведомления: планировщик запущен")
}

// Stop останавливает планировщик
func (s *Scheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopCh)
	s.wg.Wait()
	log.Info().Msg("уведомления: планировщик остановлен")
}

func (s *Scheduler) run() {
	if s.pool == nil || s.campaignClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Запрашиваем завершённые рассылки за последние 10 минут
	for _, status := range []string{"completed", "failed"} {
		resp, err := s.campaignClient.ListCampaigns(ctx, &campaignv1.ListCampaignsRequest{
			Status: status,
			Limit:  50,
		})
		if err != nil {
			log.Error().Err(err).Str("status", status).Msg("ошибка получения рассылок для уведомлений")
			continue
		}

		for _, campaign := range resp.Campaigns {
			// Проверяем completedAt — только за последние 10 минут
			if campaign.CompletedAt == nil {
				continue
			}
			completedAt := campaign.CompletedAt.AsTime()
			if time.Since(completedAt) > 10*time.Minute {
				continue
			}

			s.maybeCreateNotification(ctx, campaign, status)
		}
	}
}

func (s *Scheduler) maybeCreateNotification(ctx context.Context, campaign *campaignv1.Campaign, status string) {
	// Получаем user_id владельца кампании через client_id
	// Ищем через таблицу users напрямую
	var userID string
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE client_id = $1 AND role = 'client' LIMIT 1`,
		campaign.ClientId,
	).Scan(&userID)
	if err != nil {
		// Нет пользователя или ошибка — пропускаем
		return
	}

	// Проверяем, нет ли уже уведомления для этой кампании
	var exists bool
	s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM notifications WHERE object_type = 'campaign' AND object_id = $1)`,
		campaign.Id,
	).Scan(&exists)
	if exists {
		return
	}

	// Формируем тело уведомления
	var notifType, body string
	switch status {
	case "completed":
		notifType = "campaign_completed"
		body = fmt.Sprintf("Рассылка «%s» завершена: %d/%d доставлено",
			campaign.Name, campaign.DeliveredCount, campaign.TotalRecipients)
	case "failed":
		notifType = "campaign_failed"
		body = fmt.Sprintf("Рассылка «%s» завершилась с ошибкой: %d получателей не охвачено",
			campaign.Name, campaign.FailedCount)
	default:
		return
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO notifications (user_id, type, body, object_type, object_id)
		 VALUES ($1, $2, $3, 'campaign', $4)`,
		userID, notifType, body, campaign.Id,
	)
	if err != nil {
		log.Error().Err(err).Str("campaign_id", campaign.Id).Msg("ошибка создания уведомления")
		return
	}

	handlers.IncrementNotificationsCreated()
	log.Debug().Str("campaign_id", campaign.Id).Str("type", notifType).Msg("уведомление создано")
}
