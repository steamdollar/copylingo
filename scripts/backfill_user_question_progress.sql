\if :{?owner_id}
\else
    \echo 'owner_id psql variable is required'
    \quit 3
\endif

BEGIN;

SELECT EXISTS (
    SELECT 1 FROM users WHERE id = :'owner_id'::BIGINT
) AS owner_exists \gset

\if :owner_exists
\else
    \echo 'owner_id does not exist in users'
    ROLLBACK;
    \quit 3
\endif

CREATE TEMP TABLE backfill_owner (id BIGINT NOT NULL) ON COMMIT DROP;

INSERT INTO backfill_owner (id)
SELECT id FROM users WHERE id = :'owner_id'::BIGINT;

INSERT INTO user_question_progress (
    user_id,
    question_id,
    ease_factor,
    interval_days,
    repetitions,
    next_review_at,
    last_reviewed_at,
    times_served,
    times_correct
)
SELECT
    owner.id,
    q.id,
    q.ease_factor,
    q.interval_days,
    q.repetitions,
    q.next_review_at,
    q.last_reviewed_at,
    q.times_served,
    q.times_correct
FROM questions q
CROSS JOIN backfill_owner owner
WHERE q.times_served > 0
   OR q.times_correct > 0
   OR q.repetitions > 0
   OR q.next_review_at IS NOT NULL
   OR q.last_reviewed_at IS NOT NULL
ON CONFLICT (user_id, question_id) DO NOTHING;

COMMIT;

SELECT COUNT(*) AS owner_progress_rows
FROM user_question_progress
WHERE user_id = :'owner_id'::BIGINT;
