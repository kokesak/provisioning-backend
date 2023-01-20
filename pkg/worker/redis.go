package worker

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RHEnVision/provisioning-backend/internal/ctxval"
	"github.com/RHEnVision/provisioning-backend/internal/math"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type RedisWorker struct {
	// the main client for enqueue and dequeue workers - safe for concurrent use
	client *redis.Client

	// handler functions
	handlers map[JobType]JobHandler

	// queue for all jobs
	queueName string

	// close channel
	closeCh chan interface{}

	// polling and wait groups
	pollInterval time.Duration
	pollMax      int
	loopWG       sync.WaitGroup
	handleWG     sync.WaitGroup

	// number of in-flight jobs (must be use via atomic functions)
	inFlight int64
}

var _ JobWorker = &RedisWorker{}

// NewRedisWorker creates new worker that keeps all jobs in a single queue (list), starts CPU+1 polling
// goroutines which fetch jobs from the queue and process them in separate goroutines. There is no limit
// on how many jobs can be processed, use Stats function to track number of in-flight jobs.
func NewRedisWorker(address, username, password string, db int, queueName string, pollInterval time.Duration, pollMax int) (*RedisWorker, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Username: username,
		Password: password,
		DB:       db,
	})
	return &RedisWorker{
		handlers:     make(map[JobType]JobHandler),
		client:       rdb,
		queueName:    queueName,
		pollInterval: pollInterval,
		pollMax:      pollMax,
		closeCh:      make(chan interface{}),
	}, nil
}

func (w *RedisWorker) RegisterHandler(jtype JobType, handler JobHandler, args any) {
	w.handlers[jtype] = handler
	gob.Register(args)
}

func loggerWithJob(ctx context.Context, job *Job) *zerolog.Logger {
	logger := ctxval.Logger(ctx).With().
		Str("job_id", job.ID.String()).
		Str("job_type", string(job.Type)).
		Interface("job_args", job.Args).Logger()
	return &logger
}

func (w *RedisWorker) Enqueue(ctx context.Context, job *Job) error {
	var err error
	if job.ID == uuid.Nil {
		job.ID, err = uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("unable to generate UUID: %w", err)
		}
	}

	logger := loggerWithJob(ctx, job)
	logger.Info().Msgf("Enqueuing job type: %s", job.Type)

	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err = enc.Encode(&job)
	if err != nil {
		return fmt.Errorf("unable to encode args: %w", err)
	}

	cmd := w.client.LPush(ctx, w.queueName, buffer.Bytes())
	if cmd.Err() != nil {
		logger.Error().Err(err).Msg("Unable to push job into Redis")
		return fmt.Errorf("unable to push job into Redis: %w", cmd.Err())
	}

	result, err := cmd.Result()
	if err != nil {
		return fmt.Errorf("unable to process result: %w", err)
	}
	logger.Info().Int64("job_result", result).Msg("Pushed job successfully")
	return nil
}

func (w *RedisWorker) Stop(ctx context.Context) {
	logger := ctxval.Logger(ctx)
	close(w.closeCh)
	logger.Info().Msg("Waiting for all workers to finish")
	w.loopWG.Wait()
	w.handleWG.Wait()
	logger.Info().Msg("Done waiting for all workers to finish")
}

func (w *RedisWorker) DequeueLoop(ctx context.Context) {
	logger := ctxval.Logger(ctx)
	pollers := math.Min(w.pollMax, runtime.NumCPU()+1)
	logger.Info().Msgf("Starting %d polling goroutines", pollers)
	for i := 1; i <= pollers; i++ {
		w.loopWG.Add(1)
		go w.dequeueLoop(ctx, i, pollers)
	}
}

func (w *RedisWorker) dequeueLoop(ctx context.Context, i, total int) {
	defer w.loopWG.Done()
	logger := ctxval.Logger(ctx)

	// spread polling intervals
	delayMs := (int(w.pollInterval.Milliseconds()) / total) * (i - 1)
	logger.Info().Msgf("Worker start delay %dms", delayMs)
	time.Sleep(time.Duration(delayMs) * time.Millisecond)

	for {
		select {
		case <-w.closeCh:
			logger.Info().Msg("Shutting down a Redis poller (stop)")
			return
		case <-ctx.Done():
			logger.Info().Msg("Shutting down a Redis poller (cancel)")
			return
		default:
			w.fetchJob(ctx)
		}
	}
}

func (w *RedisWorker) fetchJob(ctx context.Context) {
	res, err := w.client.BLPop(ctx, w.pollInterval, w.queueName).Result()

	if errors.Is(err, redis.Nil) {
		// timeout occurred
		return
	} else if err != nil {
		logger := ctxval.Logger(ctx)
		logger.Error().Err(err).Msg("Error consuming from Redis queue")
		return
	}

	var job Job
	dec := gob.NewDecoder(strings.NewReader(res[1]))
	err = dec.Decode(&job)
	logger := loggerWithJob(ctx, &job)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to unmarshal job payload, skipping")
	}

	w.handleWG.Add(1)
	atomic.AddInt64(&w.inFlight, 1)
	go w.processJob(ctx, &job)
}

func (w *RedisWorker) processJob(ctx context.Context, job *Job) {
	defer w.handleWG.Done()
	defer atomic.AddInt64(&w.inFlight, -1)
	logger := loggerWithJob(ctx, job)

	logger.Info().Msg("Dequeued job from Redis")
	if h, ok := w.handlers[job.Type]; ok {
		h(ctx, job)
	} else {
		// handler not found
		ctxval.Logger(ctx).Warn().Msgf("Redis worker handler not found for job type: %s", job.Type)
	}
}

func (w *RedisWorker) Stats(ctx context.Context) (Stats, error) {
	count, err := w.client.LLen(ctx, w.queueName).Result()
	if err != nil {
		return Stats{}, fmt.Errorf("unable to get queue len: %w", err)
	}

	return Stats{
		EnqueuedJobs: uint64(count),
		InFlight:     w.inFlight,
	}, nil
}