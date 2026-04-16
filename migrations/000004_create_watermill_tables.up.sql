-- Watermill SQL PubSub message queue table.
-- Stores domain events for durable delivery (critical topics only).
-- Watermill SQL publisher writes to this table; subscribers read and ack rows.
CREATE TABLE IF NOT EXISTS watermill_messages (
    offset          BIGSERIAL PRIMARY KEY,
    uuid            VARCHAR(36)   NOT NULL UNIQUE,
    created_at      TIMESTAMP     NOT NULL DEFAULT NOW(),
    payload         JSONB         NOT NULL,
    metadata        JSONB         NOT NULL DEFAULT '{}',
    topic           VARCHAR(255)  NOT NULL,
    transaction_id  BIGINT        NOT NULL DEFAULT txid_current()
);

CREATE INDEX IF NOT EXISTS idx_watermill_messages_topic ON watermill_messages (topic, offset);

-- Consumer group offset tracking.
-- Watermill SQL subscriber stores its last processed offset per (consumer_group, topic).
CREATE TABLE IF NOT EXISTS watermill_offsets (
    consumer_group  VARCHAR(255) NOT NULL,
    topic           VARCHAR(255) NOT NULL,
    offset_acked    BIGINT       NOT NULL DEFAULT 0,
    PRIMARY KEY (consumer_group, topic)
);
