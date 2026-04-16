-- Idempotency keys table.
-- Prevents duplicate processing when Watermill replays unacked messages after a crash.
-- Each handler marks its processed events here inside the same DB transaction as business work.
CREATE TABLE IF NOT EXISTS idempotency_keys (
    id              VARCHAR(36)   PRIMARY KEY DEFAULT gen_random_uuid()::text,
    causation_event_id VARCHAR(36) NOT NULL UNIQUE,  -- EventID that triggered the handler
    domain          VARCHAR(100)  NOT NULL,           -- e.g. "loyalty", "orders"
    handler         VARCHAR(100)  NOT NULL,           -- e.g. "credit_loyalty"
    processed_at    TIMESTAMP     NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_idempotency_causation ON idempotency_keys (causation_event_id);

-- Saga state table.
-- Tracks multi-event saga progress keyed by CorrelationID.
-- Used by OrderFulfillmentSaga to wait for both loyalty.credited + inventory.reserved.
CREATE TABLE IF NOT EXISTS saga_states (
    correlation_id      VARCHAR(36)   PRIMARY KEY,
    saga_name           VARCHAR(100)  NOT NULL,   -- e.g. "order_fulfillment"
    state               JSONB         NOT NULL DEFAULT '{}',
    completed_at        TIMESTAMP     NULL,
    created_at          TIMESTAMP     NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP     NOT NULL DEFAULT NOW()
);

-- Loyalty accounts table (write model).
CREATE TABLE IF NOT EXISTS loyalty_accounts (
    id              VARCHAR(36)   PRIMARY KEY,
    customer_id     VARCHAR(36)   NOT NULL UNIQUE,
    balance         INTEGER       NOT NULL DEFAULT 0,
    tier            VARCHAR(20)   NOT NULL DEFAULT 'BRONZE',
    created_at      BIGINT        NOT NULL,  -- Unix timestamp (project convention)
    updated_at      BIGINT        NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_loyalty_accounts_customer ON loyalty_accounts (customer_id);
