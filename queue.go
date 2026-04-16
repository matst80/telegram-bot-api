package tgbotapi

import (
	"context"
	"time"
)

type executorResult struct {
	resp *APIResponse
	err  error
}

type workType int

const (
	workMakeRequest workType = iota
	workRequest
	workUploadFiles
)

type queueItem struct {
	typ      workType
	endpoint string
	params   Params
	c        Chattable
	files    []RequestFile
	respChan chan executorResult
}

// QueuedExecutor is a RequestExecutor that queues requests and handles rate limits.
type QueuedExecutor struct {
	Parent RequestExecutor
	queue  chan queueItem
	ctx    context.Context
}

// QueuedExtractor is an alias for QueuedExecutor.
type QueuedExtractor = QueuedExecutor

// NewQueuedExecutor creates a new QueuedExecutor.
func NewQueuedExecutor(parent RequestExecutor, ctx context.Context) *QueuedExecutor {
	q := &QueuedExecutor{
		Parent: parent,
		queue:  make(chan queueItem, 100),
		ctx:    ctx,
	}

	go q.run()

	return q
}

func (q *QueuedExecutor) SetDebug(debug bool) {
	q.Parent.SetDebug(debug)
}

// SetAPIEndpoint changes the Telegram Bot API endpoint used by the instance.
func (q *QueuedExecutor) SetApiEndpoint(apiEndpoint string) {
	q.Parent.SetApiEndpoint(apiEndpoint)
}

func (q *QueuedExecutor) Debug() bool {
	return q.Parent.Debug()
}

func (q *QueuedExecutor) MakeRequest(endpoint string, params Params) (*APIResponse, error) {
	item := queueItem{
		typ:      workMakeRequest,
		endpoint: endpoint,
		params:   params,
		respChan: make(chan executorResult, 1),
	}

	return q.enqueueAndWait(item)
}

func (q *QueuedExecutor) Request(c Chattable) (*APIResponse, error) {
	item := queueItem{
		typ:      workRequest,
		c:        c,
		respChan: make(chan executorResult, 1),
	}

	return q.enqueueAndWait(item)
}

func (q *QueuedExecutor) UploadFiles(endpoint string, params Params, files []RequestFile) (*APIResponse, error) {
	item := queueItem{
		typ:      workUploadFiles,
		endpoint: endpoint,
		params:   params,
		files:    files,
		respChan: make(chan executorResult, 1),
	}

	return q.enqueueAndWait(item)
}

func (q *QueuedExecutor) enqueueAndWait(item queueItem) (*APIResponse, error) {
	select {
	case q.queue <- item:
	case <-q.ctx.Done():
		return nil, q.ctx.Err()
	}

	select {
	case res := <-item.respChan:
		return res.resp, res.err
	case <-q.ctx.Done():
		return nil, q.ctx.Err()
	}
}

func (q *QueuedExecutor) run() {
	for {
		select {
		case item := <-q.queue:
			var res executorResult
			switch item.typ {
			case workMakeRequest:
				res.resp, res.err = q.Parent.MakeRequest(item.endpoint, item.params)
			case workRequest:
				res.resp, res.err = q.Parent.Request(item.c)
			case workUploadFiles:
				res.resp, res.err = q.Parent.UploadFiles(item.endpoint, item.params, item.files)
			}

			if res.resp != nil && res.resp.ErrorCode == 429 {
				q.handleRateLimit(q.ctx, item, res.resp)
				continue
			}

			item.respChan <- res
		case <-q.ctx.Done():
			return
		}
	}
}

func (q *QueuedExecutor) handleRateLimit(ctx context.Context, item queueItem, resp *APIResponse) {
	retryAfter := 5
	if resp.Parameters != nil && resp.Parameters.RetryAfter > 0 {
		retryAfter = resp.Parameters.RetryAfter
	}

	time.AfterFunc(time.Duration(retryAfter)*time.Second, func() {
		select {
		case q.queue <- item:
		case <-ctx.Done():
			item.respChan <- executorResult{err: ctx.Err()}
		}
	})
}
