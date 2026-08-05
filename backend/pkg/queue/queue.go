package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Queue names — used as Redis list keys
const (
	QueueEmail         = "queue:email"
	QueueEmailCampaign = "queue:email:campaign"
	QueueNotify        = "queue:notify"
	QueueAI            = "queue:ai"
)

// Job represents a unit of work in the queue
type Job struct {
	ID         string          `json:"id"`
	Queue      string          `json:"queue"`
	Payload    json.RawMessage `json:"payload"`
	Attempts   int             `json:"attempts"`
	MaxRetries int             `json:"max_retries"`
	CreatedAt  time.Time       `json:"created_at"`
	RunAt      time.Time       `json:"run_at"`
}

// EmailJobPayload is the data passed with every email job
type EmailJobPayload struct {
	TenantID   string            `json:"tenant_id"`
	TemplateID string            `json:"template_id"`
	ToEmail    string            `json:"to_email"`
	ToName     string            `json:"to_name"`
	Variables  map[string]string `json:"variables"`
	OrderID    string            `json:"order_id,omitempty"`
	CampaignID string            `json:"campaign_id,omitempty"`
	LogID      string            `json:"log_id,omitempty"`
}

// Client wraps the Redis client with queue operations
type Client struct {
	redis *redis.Client
}

func NewClient() (*Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{redis: client}, nil
}

// Publish adds a job to the named queue
// Uses Redis LPUSH — list-based FIFO queue
func (c *Client) Publish(ctx context.Context, queueName string, payload interface{}, opts ...PublishOption) error {
	options := &publishOptions{
		maxRetries: 3,
		runAt:      time.Now(),
	}
	for _, o := range opts {
		o(options)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	job := Job{
		ID:         fmt.Sprintf("%s-%d", queueName, time.Now().UnixNano()),
		Queue:      queueName,
		Payload:    json.RawMessage(payloadBytes),
		Attempts:   0,
		MaxRetries: options.maxRetries,
		CreatedAt:  time.Now(),
		RunAt:      options.runAt,
	}

	jobBytes, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// If scheduled for future, use a sorted set (score = unix timestamp)
	if options.runAt.After(time.Now().Add(1 * time.Second)) {
		return c.redis.ZAdd(ctx, queueName+":scheduled", redis.Z{
			Score:  float64(options.runAt.Unix()),
			Member: string(jobBytes),
		}).Err()
	}

	// Otherwise push to immediate queue
	return c.redis.LPush(ctx, queueName, string(jobBytes)).Err()
}

// Subscribe blocks and processes jobs from a queue
// processFunc is called for each job — returns error to retry, nil to ack
func (c *Client) Subscribe(ctx context.Context, queueName string, processFunc func(ctx context.Context, job *Job) error) {
	fmt.Printf("Worker listening on queue: %s\n", queueName)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// BRPOP blocks for up to 5 seconds waiting for a job
			result, err := c.redis.BRPop(ctx, 5*time.Second, queueName).Result()
			if err == redis.Nil {
				// No jobs available — check scheduled queue and continue
				c.promoteScheduledJobs(ctx, queueName)
				continue
			}
			if err != nil {
				fmt.Printf("Queue error on %s: %v\n", queueName, err)
				time.Sleep(1 * time.Second)
				continue
			}

			// result[0] = queue name, result[1] = job data
			var job Job
			if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
				fmt.Printf("Failed to unmarshal job: %v\n", err)
				continue
			}

			job.Attempts++

			if err := processFunc(ctx, &job); err != nil {
				fmt.Printf("Job %s failed (attempt %d/%d): %v\n",
					job.ID, job.Attempts, job.MaxRetries, err)

				// Retry with exponential backoff if under max retries
				if job.Attempts < job.MaxRetries {
					backoff := time.Duration(job.Attempts*job.Attempts) * 30 * time.Second
					job.RunAt = time.Now().Add(backoff)

					// Re-enqueue the same job (preserving Attempts) via the scheduled set
					jobBytes, err := json.Marshal(job)
					if err != nil {
						fmt.Printf("Failed to marshal retry for job %s: %v\n", job.ID, err)
						continue
					}
					if err := c.redis.ZAdd(ctx, queueName+":scheduled", redis.Z{
						Score:  float64(job.RunAt.Unix()),
						Member: string(jobBytes),
					}).Err(); err != nil {
						fmt.Printf("Failed to re-enqueue job %s for retry: %v\n", job.ID, err)
					}
				} else {
					// Move to dead letter queue after max retries
					jobBytes, err := json.Marshal(job)
					if err != nil {
						fmt.Printf("Failed to marshal dead job %s: %v\n", job.ID, err)
						continue
					}
					if err := c.redis.LPush(ctx, queueName+":dead", string(jobBytes)).Err(); err != nil {
						fmt.Printf("Failed to move job %s to dead letter queue: %v\n", job.ID, err)
					} else {
						fmt.Printf("Job %s moved to dead letter queue\n", job.ID)
					}
				}
			}
		}
	}
}

// promoteScheduledJobs moves jobs whose run_at has passed to the main queue
func (c *Client) promoteScheduledJobs(ctx context.Context, queueName string) {
	now := float64(time.Now().Unix())

	// Get all jobs scheduled for <= now
	jobs, err := c.redis.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     queueName + ":scheduled",
		Start:   "-inf",
		Stop:    fmt.Sprintf("%f", now),
		ByScore: true,
	}).Result()
	if err != nil || len(jobs) == 0 {
		return
	}

	for _, jobData := range jobs {
		// Move from scheduled set to immediate queue
		pipe := c.redis.TxPipeline()
		pipe.ZRem(ctx, queueName+":scheduled", jobData)
		pipe.LPush(ctx, queueName, jobData)
		if _, err := pipe.Exec(ctx); err != nil {
			fmt.Printf("Failed to promote scheduled job on %s: %v\n", queueName, err)
		}
	}
}

// Redis exposes the underlying client so other components (the rate
// limiter) can reuse this connection pool instead of opening a second
// one against the same server.
func (c *Client) Redis() *redis.Client {
	if c == nil {
		return nil
	}
	return c.redis
}

// Queue stats for monitoring dashboard
func (c *Client) Stats(ctx context.Context, queueName string) map[string]int64 {
	pending, _ := c.redis.LLen(ctx, queueName).Result()
	scheduled, _ := c.redis.ZCard(ctx, queueName+":scheduled").Result()
	dead, _ := c.redis.LLen(ctx, queueName+":dead").Result()

	return map[string]int64{
		"pending":   pending,
		"scheduled": scheduled,
		"dead":      dead,
	}
}

// PublishOption functional options for Publish
type PublishOption func(*publishOptions)

type publishOptions struct {
	maxRetries int
	runAt      time.Time
}

func WithDelay(d time.Duration) PublishOption {
	return func(o *publishOptions) { o.runAt = time.Now().Add(d) }
}

func WithMaxRetries(n int) PublishOption {
	return func(o *publishOptions) { o.maxRetries = n }
}

func WithScheduleAt(t time.Time) PublishOption {
	return func(o *publishOptions) { o.runAt = t }
}
