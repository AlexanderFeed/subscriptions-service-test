package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const DateLayout = "01-2006"

type Subscription struct {
	ID          uuid.UUID
	ServiceName string
	Price       int
	UserID      uuid.UUID
	StartDate   time.Time
	EndDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// --- HTTP запросы / ответы ---

type CreateSubscriptionRequest struct {
	ServiceName string  `json:"service_name" binding:"required"`
	Price       int     `json:"price"        binding:"required,min=1"`
	UserID      string  `json:"user_id"      binding:"required"`
	StartDate   string  `json:"start_date"   binding:"required"`
	EndDate     *string `json:"end_date"`
}

type UpdateSubscriptionRequest struct {
	ServiceName *string `json:"service_name"`
	Price       *int    `json:"price" binding:"omitempty,min=1"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
}

type SubscriptionResponse struct {
	ID          uuid.UUID `json:"id"`
	ServiceName string    `json:"service_name"`
	Price       int       `json:"price"`
	UserID      uuid.UUID `json:"user_id"`
	StartDate   string    `json:"start_date"`
	EndDate     *string   `json:"end_date,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ListSubscriptionsResponse struct {
	Data   []*SubscriptionResponse `json:"data"`
	Total  int                     `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
}

type CostQueryParams struct {
	UserID      *string `form:"user_id"`
	ServiceName *string `form:"service_name"`
	PeriodStart string  `form:"period_start" binding:"required"`
	PeriodEnd   string  `form:"period_end"   binding:"required"`
}

type CostResponse struct {
	TotalCost   int    `json:"total_cost"`
	Currency    string `json:"currency"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// --- Внутренние структуры для сервисного слоя ---

type CreateSubscriptionInput struct {
	ServiceName string
	Price       int
	UserID      uuid.UUID
	StartDate   time.Time
	EndDate     *time.Time
}

type UpdateSubscriptionInput struct {
	ServiceName *string
	Price       *int
	StartDate   *time.Time
	EndDate     *time.Time
}

type ListFilter struct {
	UserID      *uuid.UUID
	ServiceName *string
	Limit       int
	Offset      int
}

type CostFilter struct {
	UserID      *uuid.UUID
	ServiceName *string
	PeriodStart time.Time
	PeriodEnd   time.Time
}

// --- Хелперы ---

func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format %q, expected MM-YYYY: %w", s, err)
	}
	return t.UTC(), nil
}

func FormatDate(t time.Time) string {
	return t.UTC().Format(DateLayout)
}

func ToResponse(s *Subscription) *SubscriptionResponse {
	r := &SubscriptionResponse{
		ID:          s.ID,
		ServiceName: s.ServiceName,
		Price:       s.Price,
		UserID:      s.UserID,
		StartDate:   FormatDate(s.StartDate),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
	if s.EndDate != nil {
		f := FormatDate(*s.EndDate)
		r.EndDate = &f
	}
	return r
}