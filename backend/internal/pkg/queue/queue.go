package queue

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"

	"anxuncloud/internal/config"
)

type Queue struct {
	pool    *pgxpool.Pool
	workers *river.Workers
	client  *river.Client[pgx.Tx]
	mu      sync.Mutex
}

func New(cfg config.PostgresConfig) (*Queue, error) {
	dsn := fmt.Sprintf("postgres://%s@%s:%d/%s?sslmode=%s&TimeZone=Asia%%2FShanghai", url.UserPassword(cfg.User, cfg.Password).String(), cfg.Host, cfg.Port, cfg.DBName, url.QueryEscape(cfg.SSLMode))
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		pool.Config().MaxConns = int32(cfg.MaxOpenConns)
	}
	return &Queue{pool: pool, workers: river.NewWorkers()}, nil
}

func RegisterWorker[T river.JobArgs](q *Queue, worker river.Worker[T]) {
	river.AddWorker(q.workers, worker)
}

func (q *Queue) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.client != nil {
		return nil
	}
	driver := riverpgxv5.New(q.pool)
	migrator, err := rivermigrate.New(driver, &rivermigrate.Config{})
	if err != nil {
		return err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}
	client, err := river.NewClient(driver, &river.Config{
		Workers: q.workers,
		Queues: map[string]river.QueueConfig{
			"notify": {MaxWorkers: 4},
			"ai":     {MaxWorkers: 8},
			"render": {MaxWorkers: 2},
		},
		MaxAttempts:         8,
		JobTimeout:          5 * time.Minute,
		SoftStopTimeout:     10 * time.Second,
		FetchOnlyKnownKinds: true,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	q.client = client
	return nil
}

func (q *Queue) Stop(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.client != nil {
		if err := q.client.StopAndCancel(ctx); err != nil {
			q.pool.Close()
			return err
		}
		q.client = nil
	}
	q.pool.Close()
	return nil
}

func (q *Queue) InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	q.mu.Lock()
	client := q.client
	q.mu.Unlock()
	if client == nil {
		return nil, fmt.Errorf("任务队列尚未启动")
	}
	return client.InsertTx(ctx, tx, args, opts)
}

func (q *Queue) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return q.pool.BeginTx(ctx, pgx.TxOptions{})
}
