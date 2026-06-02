package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"subscriptions-service/internal/models"
	"subscriptions-service/internal/reps"
)

type SubscriptionService interface {
	Create(ctx context.Context, req models.CreateSubscriptionRequest) (*models.SubscriptionResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.SubscriptionResponse, error)
	List(ctx context.Context, f models.ListFilter) (*models.ListSubscriptionsResponse, error)
	Update(ctx context.Context, id uuid.UUID, req models.UpdateSubscriptionRequest) (*models.SubscriptionResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CalculateCost(ctx context.Context, params models.CostQueryParams) (*models.CostResponse, error)
}

type subscriptionService struct {
	repo   reps.SubscriptionRepository
	logger *logrus.Logger
}

func NewSubscriptionService(repo reps.SubscriptionRepository, logger *logrus.Logger) SubscriptionService {
	return &subscriptionService{repo: repo, logger: logger}
}

func (s *subscriptionService) Create(ctx context.Context, req models.CreateSubscriptionRequest) (*models.SubscriptionResponse, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	startDate, err := models.ParseDate(req.StartDate)
	if err != nil {
		return nil, err
	}

	input := models.CreateSubscriptionInput{
		ServiceName: req.ServiceName,
		Price:       req.Price,
		UserID:      userID,
		StartDate:   startDate,
	}

	if req.EndDate != nil {
		endDate, err := models.ParseDate(*req.EndDate)
		if err != nil {
			return nil, err
		}
		if endDate.Before(startDate) {
			return nil, fmt.Errorf("end_date must be >= start_date")
		}
		input.EndDate = &endDate
	}

	sub, err := s.repo.Create(ctx, input)
	if err != nil {
		s.logger.WithError(err).Error("failed to create subscription")
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"id": sub.ID, "service_name": sub.ServiceName, "user_id": sub.UserID,
	}).Info("subscription created")

	return models.ToResponse(sub), nil
}

func (s *subscriptionService) GetByID(ctx context.Context, id uuid.UUID) (*models.SubscriptionResponse, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return models.ToResponse(sub), nil
}

func (s *subscriptionService) List(ctx context.Context, f models.ListFilter) (*models.ListSubscriptionsResponse, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}

	subs, total, err := s.repo.List(ctx, f)
	if err != nil {
		s.logger.WithError(err).Error("failed to list subscriptions")
		return nil, err
	}

	resp := &models.ListSubscriptionsResponse{
		Data:   make([]*models.SubscriptionResponse, 0, len(subs)),
		Total:  total,
		Limit:  f.Limit,
		Offset: f.Offset,
	}
	for _, sub := range subs {
		resp.Data = append(resp.Data, models.ToResponse(sub))
	}
	return resp, nil
}

func (s *subscriptionService) Update(ctx context.Context, id uuid.UUID, req models.UpdateSubscriptionRequest) (*models.SubscriptionResponse, error) {
	input := models.UpdateSubscriptionInput{
		ServiceName: req.ServiceName,
		Price:       req.Price,
	}

	if req.StartDate != nil {
		t, err := models.ParseDate(*req.StartDate)
		if err != nil {
			return nil, err
		}
		input.StartDate = &t
	}
	if req.EndDate != nil {
		t, err := models.ParseDate(*req.EndDate)
		if err != nil {
			return nil, err
		}
		input.EndDate = &t
	}

	sub, err := s.repo.Update(ctx, id, input)
	if err != nil {
		s.logger.WithError(err).WithField("id", id).Error("failed to update subscription")
		return nil, err
	}

	s.logger.WithField("id", id).Info("subscription updated")
	return models.ToResponse(sub), nil
}

func (s *subscriptionService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.WithError(err).WithField("id", id).Error("failed to delete subscription")
		return err
	}
	s.logger.WithField("id", id).Info("subscription deleted")
	return nil
}

func (s *subscriptionService) CalculateCost(ctx context.Context, params models.CostQueryParams) (*models.CostResponse, error) {
	periodStart, err := models.ParseDate(params.PeriodStart)
	if err != nil {
		return nil, fmt.Errorf("invalid period_start: %w", err)
	}
	periodEnd, err := models.ParseDate(params.PeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("invalid period_end: %w", err)
	}
	if periodEnd.Before(periodStart) {
		return nil, fmt.Errorf("period_end must be >= period_start")
	}

	filter := models.CostFilter{PeriodStart: periodStart, PeriodEnd: periodEnd}

	if params.UserID != nil {
		uid, err := uuid.Parse(*params.UserID)
		if err != nil {
			return nil, fmt.Errorf("invalid user_id: %w", err)
		}
		filter.UserID = &uid
	}
	if params.ServiceName != nil {
		filter.ServiceName = params.ServiceName
	}

	subs, err := s.repo.ListForCost(ctx, filter)
	if err != nil {
		s.logger.WithError(err).Error("failed to calculate cost")
		return nil, err
	}

	total := 0
	for _, sub := range subs {
		total += sub.Price * overlapMonths(sub.StartDate, sub.EndDate, periodStart, periodEnd)
	}

	s.logger.WithFields(logrus.Fields{
		"period_start": params.PeriodStart,
		"period_end":   params.PeriodEnd,
		"total_cost":   total,
	}).Info("cost calculated")

	return &models.CostResponse{
		TotalCost:   total,
		Currency:    "RUB",
		PeriodStart: params.PeriodStart,
		PeriodEnd:   params.PeriodEnd,
	}, nil
}

func overlapMonths(subStart time.Time, subEnd *time.Time, periodStart, periodEnd time.Time) int {
	start := subStart
	if periodStart.After(start) {
		start = periodStart
	}

	end := periodEnd
	if subEnd != nil && subEnd.Before(end) {
		end = *subEnd
	}

	if end.Before(start) {
		return 0
	}

	months := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month()) + 1
	if months < 0 {
		return 0
	}
	return months
}