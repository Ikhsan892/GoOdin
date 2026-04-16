package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"goodin/pkg/cqrs"
	"goodin/use_cases/loyalty/dto"
)

// LoyaltyController handles HTTP requests for the loyalty domain.
// Constructed by FX via NewLoyaltyController — buses are injected automatically.
type LoyaltyController struct {
	cmd   cqrs.CommandBus
	query cqrs.QueryBus
}

// NewLoyaltyController is the FX constructor.
// FX injects CommandBus and QueryBus provided by watermillfx.Module.
func NewLoyaltyController(cmd cqrs.CommandBus, qry cqrs.QueryBus) *LoyaltyController {
	return &LoyaltyController{cmd: cmd, query: qry}
}

// Register implements httpdriver.EchoRoute (structural — no import of drivers/http needed).
// Called by drivers/http.NewEchoFX for every FX-provided route.
func (h *LoyaltyController) Register(g *echo.Group) {
	g.GET("/v1/loyalty", h.GetBalance)
	g.POST("/v1/loyalty/credit", h.CreditPoints)
}

// GET /api/v1/loyalty?customer_id=xxx
func (h *LoyaltyController) GetBalance(c echo.Context) error {
	result, err := h.query.Ask(c.Request().Context(), dto.GetLoyaltyBalanceRequest{
		CustomerID: c.QueryParam("customer_id"),
	})
	if err != nil {
		return NewServiceError(err)
	}
	return NewApiResponse(result.(dto.GetLoyaltyBalanceResponse), http.StatusOK, c)
}

// POST /api/v1/loyalty/credit
// Body: {"CustomerID":"...","OrderID":"...","Points":10}
func (h *LoyaltyController) CreditPoints(c echo.Context) error {
	var req dto.CreditLoyaltyRequest
	if err := c.Bind(&req); err != nil {
		return NewBadRequestError(err.Error(), nil)
	}
	result, err := h.cmd.Dispatch(c.Request().Context(), req)
	if err != nil {
		return NewServiceError(err)
	}
	return NewApiResponse(result.(dto.CreditLoyaltyResponse), http.StatusCreated, c)
}
