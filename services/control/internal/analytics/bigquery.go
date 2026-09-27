package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/google/externalaccount"
)

type BigQuerySink struct {
	client   *http.Client
	endpoint string
}

type insertRow struct {
	InsertID string `json:"insertId"`
	JSON     Record `json:"json"`
}

type insertRequest struct {
	Rows []insertRow `json:"rows"`
}

func NewBigQuerySink(client *http.Client, endpoint string) *BigQuerySink {
	return &BigQuerySink{client: client, endpoint: endpoint}
}

func NewSink(ctx context.Context) (Sink, error) {
	switch os.Getenv("GRIDOS_ANALYTICS") {
	case "", "local":
		return NewLocalSink(".local/analytics"), nil
	case "bigquery":
		tokenSource, defaultProject, err := bigQueryTokenSource(ctx)
		if err != nil {
			return nil, err
		}
		project := os.Getenv("GRIDOS_BIGQUERY_PROJECT")
		if project == "" {
			project = defaultProject
		}
		dataset := os.Getenv("GRIDOS_BIGQUERY_DATASET")
		table := os.Getenv("GRIDOS_BIGQUERY_TABLE")
		if project == "" || dataset == "" || table == "" {
			return nil, errors.New("BigQuery project, dataset, and table are required")
		}
		client := oauth2.NewClient(ctx, tokenSource)
		client.Timeout = 10 * time.Second
		endpoint := fmt.Sprintf("https://bigquery.googleapis.com/bigquery/v2/projects/%s/datasets/%s/tables/%s/insertAll", url.PathEscape(project), url.PathEscape(dataset), url.PathEscape(table))
		return NewBigQuerySink(client, endpoint), nil
	default:
		return nil, errors.New("unknown analytics sink")
	}
}

type ecsTaskCredentials struct {
	client   *http.Client
	endpoint string
	region   string
}

func (source ecsTaskCredentials) AwsRegion(context.Context, externalaccount.SupplierOptions) (string, error) {
	return source.region, nil
}

func (source ecsTaskCredentials) AwsSecurityCredentials(ctx context.Context, _ externalaccount.SupplierOptions) (*externalaccount.AwsSecurityCredentials, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := source.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ECS task credentials returned HTTP %d", response.StatusCode)
	}
	var temporary struct {
		AccessKeyID     string    `json:"AccessKeyId"`
		SecretAccessKey string    `json:"SecretAccessKey"`
		Token           string    `json:"Token"`
		Expiration      time.Time `json:"Expiration"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&temporary); err != nil {
		return nil, err
	}
	if temporary.AccessKeyID == "" || temporary.SecretAccessKey == "" || temporary.Token == "" || !temporary.Expiration.After(time.Now().Add(time.Minute)) {
		return nil, errors.New("ECS task credentials are missing or expired")
	}
	return &externalaccount.AwsSecurityCredentials{AccessKeyID: temporary.AccessKeyID, SecretAccessKey: temporary.SecretAccessKey, SessionToken: temporary.Token}, nil
}

func bigQueryTokenSource(ctx context.Context) (oauth2.TokenSource, string, error) {
	relative := os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI")
	if relative == "" {
		credentials, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/bigquery.insertdata")
		if err != nil {
			return nil, "", err
		}
		return credentials.TokenSource, credentials.ProjectID, nil
	}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	audience := os.Getenv("GRIDOS_WIF_AUDIENCE")
	account := os.Getenv("GRIDOS_WIF_SERVICE_ACCOUNT")
	if !strings.HasPrefix(relative, "/v2/credentials/") || path.Clean(relative) != relative || strings.ContainsAny(relative, "?#") || region == "" || audience == "" || account == "" {
		return nil, "", errors.New("ECS federation configuration is incomplete")
	}
	source := ecsTaskCredentials{client: &http.Client{Timeout: 5 * time.Second}, endpoint: "http://169.254.170.2" + relative, region: region}
	tokenSource, err := externalaccount.NewTokenSource(ctx, externalaccount.Config{
		Audience: audience, SubjectTokenType: "urn:ietf:params:aws:token-type:aws4_request",
		ServiceAccountImpersonationURL: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" + url.PathEscape(account) + ":generateAccessToken",
		Scopes:                         []string{"https://www.googleapis.com/auth/bigquery.insertdata"}, AwsSecurityCredentialsSupplier: source,
	})
	return tokenSource, "", err
}

func (sink *BigQuerySink) Write(ctx context.Context, record Record) error {
	if err := record.validate(); err != nil {
		return err
	}
	body, err := json.Marshal(insertRequest{Rows: []insertRow{{InsertID: record.ID, JSON: record}}})
	if err != nil {
		return err
	}
	for attempt := range 3 {
		if err := sink.post(ctx, body); err == nil {
			return nil
		} else if attempt == 2 || !isRetryable(err) {
			return err
		}
		timer := time.NewTimer(time.Duration(100<<attempt) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return errors.New("BigQuery insert exhausted retries")
}

type retryableStatus struct {
	code int
}

func (status retryableStatus) Error() string {
	return fmt.Sprintf("BigQuery returned HTTP %d", status.code)
}

func isRetryable(err error) bool {
	var status retryableStatus
	return errors.As(err, &status)
}

func (sink *BigQuerySink) post(ctx context.Context, body []byte) (postErr error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sink.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := sink.client.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		postErr = errors.Join(postErr, response.Body.Close())
	}()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
		return retryableStatus{code: response.StatusCode}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, err := io.ReadAll(io.LimitReader(response.Body, 4096))
		if err != nil {
			return err
		}
		return fmt.Errorf("BigQuery returned HTTP %d: %s", response.StatusCode, message)
	}
	var result struct {
		InsertErrors []json.RawMessage `json:"insertErrors"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return err
	}
	if len(result.InsertErrors) != 0 {
		return errors.New("BigQuery rejected an inserted row")
	}
	return nil
}
