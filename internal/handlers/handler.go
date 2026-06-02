package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"subscriptions-service/internal/models"
	"subscriptions-service/internal/reps"
	"subscriptions-service/internal/service"
)

type SubscriptionHandler struct {
	svc    service.SubscriptionService
	logger *logrus.Logger
}

func NewSubscriptionHandler(svc service.SubscriptionService, logger *logrus.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc, logger: logger}
}

// Create godoc
// @Summary      Create a subscription
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Param        body body     models.CreateSubscriptionRequest true "Subscription payload"
// @Success      201  {object} models.SubscriptionResponse
// @Failure      400  {object} models.ErrorResponse
// @Router       /subscriptions [post]
func (h *SubscriptionHandler) Create(c *gin.Context) {
	 h.logger.WithField("method", "Create").Info("handling request") 
	var req models.CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// GetByID godoc
// @Summary      Get subscription by ID
// @Tags         subscriptions
// @Produce      json
// @Param        id  path     string true "Subscription UUID"
// @Success      200 {object} models.SubscriptionResponse
// @Failure      404 {object} models.ErrorResponse
// @Router       /subscriptions/{id} [get]
func (h *SubscriptionHandler) GetByID(c *gin.Context) {
	h.logger.WithField("id", c.Param("id")).Info("handling get by id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid uuid"})
		return
	}
	resp, err := h.svc.GetByID(c.Request.Context(), id)
	if errors.Is(err, reps.ErrNotFound) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// List godoc
// @Summary      List subscriptions
// @Tags         subscriptions
// @Produce      json
// @Param        user_id      query string false "Filter by user UUID"
// @Param        service_name query string false "Filter by service name"
// @Param        limit        query int    false "Page size (default 20)"
// @Param        offset       query int    false "Offset (default 0)"
// @Success      200 {object} models.ListSubscriptionsResponse
// @Router       /subscriptions [get]
func (h *SubscriptionHandler) List(c *gin.Context) {
	h.logger.WithFields(logrus.Fields{
        "user_id":      c.Query("user_id"),
        "service_name": c.Query("service_name"),
        "limit":        c.Query("limit"),
        "offset":       c.Query("offset"),
    }).Info("handling list")
	filter := models.ListFilter{Limit: 20}

	if v := c.Query("user_id"); v != "" {
		uid, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user_id"})
			return
		}
		filter.UserID = &uid
	}
	if v := c.Query("service_name"); v != "" {
		filter.ServiceName = &v
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	resp, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Update godoc
// @Summary      Update a subscription
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Param        id   path     string                          true "Subscription UUID"
// @Param        body body     models.UpdateSubscriptionRequest true "Fields to update"
// @Success      200  {object} models.SubscriptionResponse
// @Failure      404  {object} models.ErrorResponse
// @Router       /subscriptions/{id} [patch]
func (h *SubscriptionHandler) Update(c *gin.Context) {
	h.logger.WithField("id", c.Param("id")).Info("handling update")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid uuid"})
		return
	}
	var req models.UpdateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, req)
	if errors.Is(err, reps.ErrNotFound) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Delete godoc
// @Summary      Delete a subscription
// @Tags         subscriptions
// @Param        id path string true "Subscription UUID"
// @Success      204
// @Failure      404 {object} models.ErrorResponse
// @Router       /subscriptions/{id} [delete]
func (h *SubscriptionHandler) Delete(c *gin.Context) {
	h.logger.WithField("id", c.Param("id")).Info("handling delete")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid uuid"})
		return
	}
	err = h.svc.Delete(c.Request.Context(), id)
	if errors.Is(err, reps.ErrNotFound) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

// CalculateCost godoc
// @Summary      Calculate total cost for a period
// @Tags         subscriptions
// @Produce      json
// @Param        period_start query string true  "MM-YYYY"
// @Param        period_end   query string true  "MM-YYYY"
// @Param        user_id      query string false "Filter by user UUID"
// @Param        service_name query string false "Filter by service name"
// @Success      200 {object} models.CostResponse
// @Failure      400 {object} models.ErrorResponse
// @Router       /subscriptions/cost [get]
func (h *SubscriptionHandler) CalculateCost(c *gin.Context) {
	var params models.CostQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	resp, err := h.svc.CalculateCost(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}