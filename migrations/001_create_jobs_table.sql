CREATE SCHEMA pgqueue;
CREATE TABLE pgqueue.jobs (
  id bigint PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  state text NOT NULL DEFAULT 'available',
  arguments jsonb,
  created_at timestamptz NOT NULL DEFAULT statement_timestamp(),
  scheduled_at timestamptz NOT NULL DEFAULT statement_timestamp(),
  attempted_at timestamptz,
  finalized_at timestamptz,
  attempt smallint NOT NULL DEFAULT 0,
  max_attempts smallint NOT NULL CHECK (max_attempts > 0),
  kind text NOT NULL CHECK (LENGTH(kind) BETWEEN 1 AND 127),
  last_error text,
  attempted_by text[],
  CONSTRAINT valid_state CHECK (
    state in (
      'available','scheduled', 'executing', 'completed', 'retryable', 'discarded', 'cancelled'
    )
  ),
  CONSTRAINT finalized_at_matches_state CHECK (
    (state in ('completed', 'discarded', 'cancelled') AND finalized_at IS NOT NULL) OR
    (state not in ('completed', 'discarded', 'cancelled') AND finalized_at IS NULL)
  )
);
