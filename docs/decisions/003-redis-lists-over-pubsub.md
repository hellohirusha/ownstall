# ADR-003: Redis lists as the job queue

**Status**: Accepted
**Date**: 2026-08-01

## Context

Several things must happen after a request returns rather than during it:
order confirmation emails, campaign sends, AI copy generation, production
queue transitions. Doing them inline would put a provider's latency — and a
provider's outage — on the critical path of a checkout.

The reference architecture for this kind of system uses a managed broker
(GCP Pub/Sub, SQS, or similar). None of them have a free tier that covers
development and a public demo.

## Decision

A small job queue built on Redis data structures, behind a two-method
interface:

```go
func (c *Client) Publish(ctx context.Context, queueName string,
    payload interface{}, opts ...PublishOption) error

func (c *Client) Subscribe(ctx context.Context, queueName string,
    processFunc func(ctx context.Context, job *Job) error)
```

Immediate jobs `LPUSH` onto a list. Jobs scheduled for the future go into a
sorted set scored by their run time, which a promoter moves onto the list
when they come due. Jobs carry an attempt count and a retry ceiling; a
`processFunc` that returns an error is retried, and one that returns nil
acknowledges.

## Rationale

The interface is the decision, not Redis. `Publish` and `Subscribe` are
what the rest of the codebase depends on, and both have direct equivalents
in every managed broker. Swapping the implementation touches one file and
no callers.

Redis specifically because it was already provisioned for rate limiting, so
the queue costs one dependency rather than two, and because lists and sorted
sets give delayed delivery and retries without a broker's operational
surface.

Choosing the managed broker now would have meant designing against a system
that is not deployed, paying for it during development, and still writing
the abstraction — because a demo that cannot run without a cloud account is
not a demo.

## Consequences

This is a queue, not a broker. It has no fan-out to multiple independent
consumers, no dead-letter queue, and no delivery guarantee stronger than
"retried until the attempt ceiling". Losing the Redis instance loses queued
jobs. That is acceptable for confirmation emails and AI generation, and it
would not be acceptable for anything that moves money — which is why
payments run through Stripe's webhooks and their retry policy rather than
through this queue.

Redis is now load-bearing for two unrelated concerns. Its absence is handled
explicitly rather than fatally: the API logs a warning at boot, the rate
limiter becomes a pass-through, and queued work is skipped rather than
crashing the process.

## Revisiting

The migration is mechanical:

```go
// Now
return c.redis.LPush(ctx, queueName, string(jobBytes)).Err()

// Pub/Sub, same signature
return c.pubsub.Topic(queueName).
    Publish(ctx, &pubsub.Message{Data: jobBytes}).Get(ctx)
```

The scheduled-job sorted set has no Pub/Sub equivalent and would move to
Cloud Tasks or a scheduled trigger. That is the one piece of this ADR that
is not a like-for-like swap, and it is worth knowing before the migration
rather than during it.
