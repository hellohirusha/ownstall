# Architecture Decision Records

Each record captures one decision that shaped the system, the options that
were on the table, and what would have to change to revisit it. They are
written after the fact but before the code settled, so the tradeoffs are the
ones actually weighed rather than the ones that read well in hindsight.

| ADR                                       | Decision                                             |
| ----------------------------------------- | ---------------------------------------------------- |
| [001](001-multi-tenant-isolation.md)      | Row-level security for multi-tenant isolation        |
| [002](002-graphql-over-rest.md)           | GraphQL via gqlgen as the primary API                |
| [003](003-redis-lists-over-pubsub.md)     | Redis lists as the job queue                         |
| [004](004-groq-as-ai-provider.md)         | Groq as the AI provider, behind a metered client     |
| [005](005-railway-over-gcp.md)            | Railway for hosting                                  |
