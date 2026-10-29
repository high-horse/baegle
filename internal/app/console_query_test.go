package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"db-lens/internal/engine/entities"
)

func TestOnDemandCountAndLastPage(t *testing.T) {
	service, _, conn := editService(t)
	ctx := context.Background()
	query := "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<250) SELECT x FROM n"
	first, err := service.ExecuteQuery(ctx, entities.QueryInput{Query: query, PageSize: 100})
	if err != nil || first.TotalRows != nil || !first.HasMore {
		t.Fatalf("first page counted eagerly: %+v %v", first, err)
	}
	count, err := service.ExecuteConsoleQuery(ctx, "tab:count", "request", entities.QueryInput{Query: query, Type: entities.QueryExecutionCount})
	if err != nil || count.TotalRows == nil || *count.TotalRows != 250 || len(count.Rows) != 0 {
		t.Fatalf("bad count response: %+v %v", count, err)
	}
	last, err := service.ExecuteQuery(ctx, entities.QueryInput{Query: query, Type: entities.QueryExecutionNavigate, PageSize: 100, Page: 3})
	if err != nil || last.StartRow != 201 || len(last.Rows) != 50 || last.HasMore {
		t.Fatalf("bad last page: %+v %v", last, err)
	}
	if conn.DB().Stats().InUse != 0 {
		t.Fatal("count or page retained a connection")
	}
}

func TestResultWhereCountMatchesPage(t *testing.T) {
	service, _, _ := editService(t)
	input := entities.QueryInput{Query: "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<250) SELECT x FROM n", Where: "x > 200", OrderBy: "1 DESC", PageSize: 10, Type: entities.QueryExecutionNavigate}
	page, err := service.ExecuteQuery(context.Background(), input)
	if err != nil || page.Rows[0][0] != int64(250) {
		t.Fatalf("bad filtered page: %+v %v", page, err)
	}
	input.Type = entities.QueryExecutionCount
	count, err := service.ExecuteQuery(context.Background(), input)
	if err != nil || count.TotalRows == nil || *count.TotalRows != 50 {
		t.Fatalf("bad filtered count: %+v %v", count, err)
	}
}

func TestConsoleQueryCancellationAndExclusiveExecution(t *testing.T) {
	service, _, conn := editService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := service.ExecuteConsoleQuery(ctx, "console", "first", entities.QueryInput{
			Query: `WITH RECURSIVE numbers(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM numbers WHERE n < 1000000000) SELECT sum(n) FROM numbers`, PageSize: 100,
		})
		done <- err
	}()
	deadline := time.Now().Add(3 * time.Second)
	for conn.DB().Stats().InUse == 0 {
		if time.Now().After(deadline) {
			t.Fatal("query did not start")
		}
		select {
		case err := <-done:
			t.Fatalf("query exited before cancellation: %v", err)
		default:
		}
		time.Sleep(time.Millisecond)
	}
	if service.CancelConsoleQuery("console", "older-request") {
		t.Fatal("stale Stop targeted running query")
	}
	_, err := service.ExecuteConsoleQuery(ctx, "console", "second", entities.QueryInput{Query: "SELECT 1"})
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("concurrent execution not rejected: %v", err)
	}
	if !service.CancelConsoleQuery("console", "first") {
		t.Fatal("Stop did not find running query")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("database operation did not stop")
	}
	if service.CancelConsoleQuery("console", "first") {
		t.Fatal("completed execution remained registered")
	}
	result, err := service.ExecuteConsoleQuery(ctx, "console", "third", entities.QueryInput{Query: "SELECT 1", PageSize: 100})
	if err != nil || result == nil {
		t.Fatalf("console unusable after Stop: %v", err)
	}
}

func TestConsoleCancellationDoesNotAffectOtherConsoles(t *testing.T) {
	first, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	service := NewDbService(nil)
	service.consoleQueries = map[string]*consoleQuery{
		"one": {requestID: "first", cancel: cancelFirst},
		"two": {requestID: "second", cancel: cancelSecond},
	}
	if !service.CancelConsoleQuery("one", "first") || first.Err() == nil {
		t.Fatal("target console was not cancelled")
	}
	if second.Err() != nil {
		t.Fatal("another console was cancelled")
	}
}
