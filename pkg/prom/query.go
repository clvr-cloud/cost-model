package prom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/kubecost/cost-model/pkg/errors"
	"github.com/kubecost/cost-model/pkg/log"
	"github.com/kubecost/cost-model/pkg/util"
	prometheus "github.com/prometheus/client_golang/api"
	"k8s.io/klog"
)

const (
	apiPrefix    = "/api/v1"
	epQuery      = apiPrefix + "/query"
	epQueryRange = apiPrefix + "/query_range"
)

// Context wraps a Prometheus client and provides methods for querying and
// parsing query responses and errors.
type Context struct {
	Client         prometheus.Client
	ErrorCollector *errors.ErrorCollector
}

// NewContext creates a new Promethues querying context from the given client
func NewContext(client prometheus.Client) *Context {
	var ec errors.ErrorCollector

	return &Context{
		Client:         client,
		ErrorCollector: &ec,
	}
}

// Errors returns the errors collected from the Context's ErrorCollector
func (ctx *Context) Errors() []error {
	return ctx.ErrorCollector.Errors()
}

// HasErrors returns true if the ErrorCollector has errors
func (ctx *Context) HasErrors() bool {
	return ctx.ErrorCollector.IsError()
}

// Query returns a QueryResultsChan, then runs the given query and sends the
// results on the provided channel. Receiver is responsible for closing the
// channel, preferably using the Read method.
func (ctx *Context) Query(query string) QueryResultsChan {
	resCh := make(QueryResultsChan)

	go runQuery(query, ctx, resCh, "")

	return resCh
}

// ProfileQuery returns a QueryResultsChan, then runs the given query with a profile
// label and sends the results on the provided channel. Receiver is responsible for closing the
// channel, preferably using the Read method.
func (ctx *Context) ProfileQuery(query string, profileLabel string) QueryResultsChan {
	resCh := make(QueryResultsChan)

	go runQuery(query, ctx, resCh, profileLabel)

	return resCh
}

// QueryAll returns one QueryResultsChan for each query provided, then runs
// each query concurrently and returns results on each channel, respectively,
// in the order they were provided; i.e. the response to queries[1] will be
// sent on channel resChs[1].
func (ctx *Context) QueryAll(queries ...string) []QueryResultsChan {
	resChs := []QueryResultsChan{}

	for _, q := range queries {
		resChs = append(resChs, ctx.Query(q))
	}

	return resChs
}

// ProfileQueryAll returns one QueryResultsChan for each query provided, then runs
// each ProfileQuery concurrently and returns results on each channel, respectively,
// in the order they were provided; i.e. the response to queries[1] will be
// sent on channel resChs[1].
func (ctx *Context) ProfileQueryAll(queries ...string) []QueryResultsChan {
	resChs := []QueryResultsChan{}

	for _, q := range queries {
		resChs = append(resChs, ctx.ProfileQuery(q, fmt.Sprintf("Query #%d", len(resChs)+1)))
	}

	return resChs
}

func (ctx *Context) QuerySync(query string) ([]*QueryResult, error) {
	raw, err := ctx.query(query)
	if err != nil {
		return nil, err
	}

	results := NewQueryResults(query, raw)
	if results.Error != nil {
		return nil, results.Error
	}

	return results.Results, nil
}

// QueryURL returns the URL used to query Prometheus
func (ctx *Context) QueryURL() *url.URL {
	return ctx.Client.URL(epQuery, nil)
}

// runQuery executes the prometheus query asynchronously, collects results and
// errors, and passes them through the results channel.
func runQuery(query string, ctx *Context, resCh QueryResultsChan, profileLabel string) {
	defer errors.HandlePanic()
	startQuery := time.Now()

	raw, promErr := ctx.query(query)
	ctx.ErrorCollector.Report(promErr)

	results := NewQueryResults(query, raw)
	if results.Error != nil {
		ctx.ErrorCollector.Report(results.Error)
	}

	if profileLabel != "" {
		log.Profile(startQuery, profileLabel)
	}

	resCh <- results
}

func (ctx *Context) query(query string) (interface{}, error) {
	u := ctx.Client.URL(epQuery, nil)
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, body, warnings, err := ctx.Client.Do(context.Background(), req)
	for _, w := range warnings {
		klog.V(3).Infof("Warning '%s' fetching query '%s'", w, query)
	}
	if err != nil {
		if resp == nil {
			return nil, fmt.Errorf("Error %s fetching query %s", err.Error(), query)
		}

		return nil, fmt.Errorf("%d Error %s fetching query %s", resp.StatusCode, err.Error(), query)
	}

	var toReturn interface{}
	err = json.Unmarshal(body, &toReturn)
	if err != nil {
		return nil, fmt.Errorf("Error %s fetching query %s", err.Error(), query)
	}

	return toReturn, nil
}

func (ctx *Context) QueryRange(query string, start, end time.Time, step time.Duration) QueryResultsChan {
	resCh := make(QueryResultsChan)

	go runQueryRange(query, start, end, step, ctx, resCh, "")

	return resCh
}

func (ctx *Context) ProfileQueryRange(query string, start, end time.Time, step time.Duration, profileLabel string) QueryResultsChan {
	resCh := make(QueryResultsChan)

	go runQueryRange(query, start, end, step, ctx, resCh, profileLabel)

	return resCh
}

func (ctx *Context) QueryRangeSync(query string, start, end time.Time, step time.Duration) ([]*QueryResult, error) {
	raw, err := ctx.queryRange(query, start, end, step)
	if err != nil {
		return nil, err
	}

	results := NewQueryResults(query, raw)
	if results.Error != nil {
		return nil, results.Error
	}

	return results.Results, nil
}

// QueryRangeURL returns the URL used to query_range Prometheus
func (ctx *Context) QueryRangeURL() *url.URL {
	return ctx.Client.URL(epQueryRange, nil)
}

// runQueryRange executes the prometheus queryRange asynchronously, collects results and
// errors, and passes them through the results channel.
func runQueryRange(query string, start, end time.Time, step time.Duration, ctx *Context, resCh QueryResultsChan, profileLabel string) {
	defer errors.HandlePanic()
	startQuery := time.Now()

	raw, promErr := ctx.queryRange(query, start, end, step)
	ctx.ErrorCollector.Report(promErr)

	results := NewQueryResults(query, raw)
	if results.Error != nil {
		ctx.ErrorCollector.Report(results.Error)
	}

	if profileLabel != "" {
		log.Profile(startQuery, profileLabel)
	}

	resCh <- results
}

func (ctx *Context) queryRange(query string, start, end time.Time, step time.Duration) (interface{}, error) {
	u := ctx.Client.URL(epQueryRange, nil)
	q := u.Query()
	q.Set("query", query)
	q.Set("start", start.Format(time.RFC3339Nano))
	q.Set("end", end.Format(time.RFC3339Nano))
	q.Set("step", strconv.FormatFloat(step.Seconds(), 'f', 3, 64))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, body, warnings, err := ctx.Client.Do(context.Background(), req)
	for _, w := range warnings {
		klog.V(3).Infof("Warning '%s' fetching query '%s'", w, query)
	}
	if err != nil {
		if resp == nil {
			return nil, fmt.Errorf("Error: %s, Body: %s Query: %s", err.Error(), body, query)
		}

		return nil, fmt.Errorf("%d (%s) Headers: %s Error: %s Body: %s Query: %s", resp.StatusCode, http.StatusText(resp.StatusCode), util.HeaderString(resp.Header), body, err.Error(), query)
	}

	// Unsuccessful Status Code, log body and status
	statusCode := resp.StatusCode
	statusText := http.StatusText(statusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%d (%s) Headers: %s, Body: %s Query: %s", statusCode, statusText, util.HeaderString(resp.Header), body, query)
	}

	var toReturn interface{}
	err = json.Unmarshal(body, &toReturn)
	if err != nil {
		return nil, fmt.Errorf("%d (%s) Headers: %s Error: %s Body: %s Query: %s", statusCode, statusText, util.HeaderString(resp.Header), err.Error(), body, query)
	}

	return toReturn, nil
}
