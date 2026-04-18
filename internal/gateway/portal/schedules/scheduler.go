package schedules

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
)

// Scheduler polls campaign_schedules and launches campaigns when they are due.
// Uses a dequeue pattern (SELECT ... FOR UPDATE SKIP LOCKED) to safely support
// multiple portal-gateway instances.
type Scheduler struct {
	pool           *pgxpool.Pool
	campaignClient campaignv1.CampaignServiceClient
	ticker         *time.Ticker
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewScheduler creates a new campaign schedule Scheduler.
func NewScheduler(pool *pgxpool.Pool, campaignClient campaignv1.CampaignServiceClient) *Scheduler {
	return &Scheduler{
		pool:           pool,
		campaignClient: campaignClient,
		stopCh:         make(chan struct{}),
	}
}

// Start launches the background polling goroutine.
func (s *Scheduler) Start() {
	s.ticker = time.NewTicker(time.Minute)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run() // run immediately on start
		for {
			select {
			case <-s.ticker.C:
				s.run()
			case <-s.stopCh:
				return
			}
		}
	}()
	log.Info().Msg("campaign-schedules: планировщик запущен")
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopCh)
	s.wg.Wait()
	log.Info().Msg("campaign-schedules: планировщик остановлен")
}

type scheduleRow struct {
	ID                 string
	ClientID           string
	TemplateCampaignID string
	Name               string
	Frequency          string
	CronExpression     *string
	RunCount           int
	MaxRuns            *int
}

func (s *Scheduler) run() {
	if s.pool == nil || s.campaignClient == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Process all due schedules one by one using atomic dequeue.
	for {
		sched, err := s.dequeueOne(ctx)
		if err == pgx.ErrNoRows {
			return
		}
		if err != nil {
			log.Error().Err(err).Msg("campaign-schedules: ошибка dequeue расписания")
			return
		}
		s.executeSchedule(ctx, *sched)
	}
}

// dequeueOne atomically claims one due schedule by temporarily advancing its
// next_run_at by 10 minutes (processing lock). Returns pgx.ErrNoRows when
// there are no due schedules.
func (s *Scheduler) dequeueOne(ctx context.Context) (*scheduleRow, error) {
	var r scheduleRow
	err := s.pool.QueryRow(ctx, `
		UPDATE campaign_schedules
		SET next_run_at = NOW() + INTERVAL '10 minutes'
		WHERE id = (
			SELECT id FROM campaign_schedules
			WHERE is_active = true
			  AND next_run_at IS NOT NULL
			  AND next_run_at <= NOW()
			ORDER BY next_run_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id::text, client_id::text, template_campaign_id::text, name,
		          frequency, cron_expression, run_count, max_runs
	`).Scan(
		&r.ID, &r.ClientID, &r.TemplateCampaignID, &r.Name,
		&r.Frequency, &r.CronExpression, &r.RunCount, &r.MaxRuns,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Scheduler) executeSchedule(ctx context.Context, sched scheduleRow) {
	log.Info().
		Str("schedule_id", sched.ID).
		Str("name", sched.Name).
		Msg("campaign-schedules: выполнение расписания")

	// 1. Fetch template campaign settings via gRPC.
	tmpl, err := s.campaignClient.GetCampaign(ctx, &campaignv1.GetCampaignRequest{
		Id:       sched.TemplateCampaignID,
		ClientId: sched.ClientID,
	})
	if err != nil {
		log.Error().Err(err).
			Str("schedule_id", sched.ID).
			Str("template_campaign_id", sched.TemplateCampaignID).
			Msg("campaign-schedules: ошибка получения шаблона кампании, пропускаем")
		s.reschedule(ctx, sched, false)
		return
	}

	// 2. Create a new campaign copying the template's settings.
	newName := fmt.Sprintf("%s (%s)", sched.Name, time.Now().UTC().Format("2006-01-02 15:04 UTC"))
	created, err := s.campaignClient.CreateCampaign(ctx, &campaignv1.CreateCampaignRequest{
		ClientId:              sched.ClientID,
		Name:                  newName,
		ContactListId:         tmpl.ContactListId,
		TemplateId:            tmpl.TemplateId,
		Source:                tmpl.Source,
		SegmentRules:          tmpl.SegmentRules,
		SegmentTags:           tmpl.SegmentTags,
		SendRate:              tmpl.SendRate,
		UseSubscriberTimezone: tmpl.UseSubscriberTimezone,
	})
	if err != nil {
		log.Error().Err(err).
			Str("schedule_id", sched.ID).
			Msg("campaign-schedules: ошибка создания кампании, пропускаем")
		s.reschedule(ctx, sched, false)
		return
	}

	// 3. Launch the newly created campaign.
	_, err = s.campaignClient.LaunchCampaign(ctx, &campaignv1.LaunchCampaignRequest{
		Id:       created.Id,
		ClientId: sched.ClientID,
	})
	if err != nil {
		log.Error().Err(err).
			Str("schedule_id", sched.ID).
			Str("campaign_id", created.Id).
			Msg("campaign-schedules: ошибка запуска кампании (кампания создана в статусе draft)")
		// Continue — update the schedule regardless; the draft campaign can be launched manually.
	}

	// 4. Update the schedule: increment run_count, set last_run_at, compute real next_run_at.
	s.reschedule(ctx, sched, true)

	log.Info().
		Str("schedule_id", sched.ID).
		Str("campaign_id", created.Id).
		Int("run_count", sched.RunCount+1).
		Msg("campaign-schedules: расписание выполнено успешно")
}

// reschedule updates the schedule after a run attempt.
// If launched=true the run_count is incremented; otherwise only next_run_at is corrected.
func (s *Scheduler) reschedule(ctx context.Context, sched scheduleRow, launched bool) {
	now := time.Now().UTC()
	cronExpr := ""
	if sched.CronExpression != nil {
		cronExpr = *sched.CronExpression
	}
	nextRun := NextRunTime(sched.Frequency, cronExpr, now)

	var newRunCount int
	var deactivate bool
	if launched {
		newRunCount = sched.RunCount + 1
		deactivate = sched.MaxRuns != nil && newRunCount >= *sched.MaxRuns
	} else {
		newRunCount = sched.RunCount
		deactivate = false
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE campaign_schedules
		SET run_count   = $1,
		    last_run_at = CASE WHEN $5 THEN $2 ELSE last_run_at END,
		    next_run_at = $3,
		    is_active   = CASE WHEN $4 THEN false ELSE is_active END,
		    updated_at  = NOW()
		WHERE id = $6::uuid
	`, newRunCount, now, nextRun, deactivate, launched, sched.ID)
	if err != nil {
		log.Error().Err(err).
			Str("schedule_id", sched.ID).
			Msg("campaign-schedules: ошибка обновления расписания после запуска")
	}
	if deactivate {
		log.Info().
			Str("schedule_id", sched.ID).
			Int("max_runs", *sched.MaxRuns).
			Msg("campaign-schedules: расписание деактивировано — достигнут лимит запусков")
	}
}
