package command

import (
	"context"
	"errors"
	"log/slog"

	"goodin/pkg/cqrs"
	"goodin/pkg/events"
	"goodin/use_cases/loyalty/domain/aggregate"
	"goodin/use_cases/loyalty/domain/event"
	valueobject "goodin/use_cases/loyalty/domain/value_object"
	"goodin/use_cases/loyalty/dto"
	"goodin/use_cases/loyalty/exception"
	portcommand "goodin/use_cases/loyalty/port/input/command"
	"goodin/use_cases/loyalty/port/output/idempotency"
	"goodin/use_cases/loyalty/port/output/repository"

	core "goodin/internal"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/oklog/ulid/v2"
)

var _ portcommand.CreditLoyaltyCommandHandler = (*CreditLoyaltyHandler)(nil)

// CreditLoyaltyHandler implements the credit loyalty command.
//
// Transaction contract (using go-transaction-manager or equivalent):
//  1. Check idempotency key (inside TX)
//  2. Load or create aggregate
//  3. Mutate aggregate (raises domain events internally)
//  4. Save aggregate (inside TX)
//  5. Publish next events via SQL EventBus (writes to watermill_messages inside TX)
//  6. Mark idempotency key (inside TX)
//
// Either all steps commit together OR all roll back — no partial state.
type CreditLoyaltyHandler struct {
	app       core.App
	writeRepo repository.LoyaltyWriteRepository
	idem      idempotency.IdempotencyRepository
	eventBus  cqrs.EventBus
}

func NewCreditLoyaltyHandler(
	app core.App,
	writeRepo repository.LoyaltyWriteRepository,
	idem idempotency.IdempotencyRepository,
	eventBus cqrs.EventBus,
) *CreditLoyaltyHandler {
	return &CreditLoyaltyHandler{
		app:       app,
		writeRepo: writeRepo,
		idem:      idem,
		eventBus:  eventBus,
	}
}

// Handle processes the CreditLoyaltyRequest command.
// The caller (event handler) is responsible for wrapping this in a DB transaction.
func (h *CreditLoyaltyHandler) Handle(ctx context.Context, cmd dto.CreditLoyaltyRequest) (dto.CreditLoyaltyResponse, error) {
	ctx, span := otel.Tracer("loyalty").Start(ctx, "command.credit_loyalty")
	// Use standard OTel API
	tracer := otel.Tracer("loyalty.command.credit_loyalty")
	ctx, span2 := tracer.Start(ctx, "Handle") // intentionally separate from outer span — this is the command span

	_ = span
	defer span2.End()
	span2.SetAttributes(
		attribute.String("correlation.id", cmd.CorrelationID),
		attribute.String("causation.id", cmd.CausationEventID),
		attribute.String("customer.id", cmd.CustomerID),
	)

	// 1. Idempotency check — skip if already processed (Watermill may replay)
	if cmd.CausationEventID != "" {
		exists, err := h.idem.Exists(ctx, cmd.CausationEventID)
		if err != nil {
			return dto.CreditLoyaltyResponse{}, err
		}
		if exists {
			h.app.Logger().Info("loyalty credit: duplicate event skipped",
				slog.String("causation_id", cmd.CausationEventID))
			return dto.CreditLoyaltyResponse{}, exception.ErrDuplicateEvent
		}
	}

	// 2. Load or create aggregate
	agg, err := h.writeRepo.FindByCustomerID(ctx, cmd.CustomerID)
	if err != nil {
		return dto.CreditLoyaltyResponse{}, err
	}
	if agg == nil {
		agg = aggregate.NewLoyaltyAggregate(cmd.CustomerID)
	}

	// 3. Mutate aggregate — raises LoyaltyCreditedEvent internally
	pts, err := valueobject.NewPoints(cmd.Points)
	if err != nil {
		return dto.CreditLoyaltyResponse{}, err
	}
	if err := agg.CreditPoints(cmd.OrderID, pts); err != nil {
		return dto.CreditLoyaltyResponse{}, err
	}

	// 4. Save aggregate (caller's TX)
	if err := h.writeRepo.Save(ctx, agg); err != nil {
		return dto.CreditLoyaltyResponse{}, err
	}

	// 5. Publish domain events via SQL EventBus (writes to watermill_messages in same TX)
	domainEvents := agg.FlushEvents()
	envelopes := make([]cqrs.EventEnvelope, 0, len(domainEvents))
	for _, de := range domainEvents {
		envelopes = append(envelopes, cqrs.EventEnvelope{
			EventID:       ulid.Make().String(),
			CorrelationID: cmd.CorrelationID,
			CausationID:   cmd.CausationEventID,
			EventName:     events.LoyaltyCredited,
			OccurredAt:    de.OccurredAt(),
			Payload:       de,
		})
	}
	if err := h.eventBus.Publish(ctx, envelopes...); err != nil {
		h.app.Logger().Error("loyalty credit: failed to publish events",
			slog.String("error", err.Error()))
		return dto.CreditLoyaltyResponse{}, err
	}

	// 6. Mark idempotency key (same TX)
	if cmd.CausationEventID != "" {
		if err := h.idem.Mark(ctx, cmd.CausationEventID); err != nil {
			return dto.CreditLoyaltyResponse{}, err
		}
	}

	return dto.CreditLoyaltyResponse{
		LoyaltyAccountID: agg.Account().ID,
		NewBalance:       agg.Balance(),
		NewTier:          string(agg.Tier()),
	}, nil
}

// wrapEnvelope is a helper to assert unused event type — keeps vet happy.
var _ cqrs.DomainEvent = (*event.LoyaltyCreditedEvent)(nil)

// compile-time assertion that CreditLoyaltyRequest implements Command
var _ cqrs.Command = (*dto.CreditLoyaltyRequest)(nil)

// ensure errors package is used
var _ = errors.New
