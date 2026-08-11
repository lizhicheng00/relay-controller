package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lizhicheng00/relay-controller/internal/config"
	"github.com/lizhicheng00/relay-controller/internal/core"
	"github.com/lizhicheng00/relay-controller/internal/security"
	"github.com/lizhicheng00/relay-controller/internal/store"
)

func TestServiceAgainstMariaDB(t *testing.T) {
	databaseURL := os.Getenv("RELAY_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("RELAY_INTEGRATION_DATABASE_URL is not set")
	}
	ctx := context.Background()
	database, err := store.Open(ctx, config.Database{
		URL: databaseURL, Username: os.Getenv("RELAY_INTEGRATION_DATABASE_USERNAME"),
		Password: os.Getenv("RELAY_INTEGRATION_DATABASE_PASSWORD"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	signer, err := security.NewJWTSigner(base64.StdEncoding.EncodeToString(der), "devbridge", "relay-gateway", "1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	application := New(database, signer, "myhuaweicloud.com", "region-a", slog.New(slog.NewTextHandler(io.Discard, nil)))
	now := time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC)
	application.now = func() time.Time { return now }

	accountNamespace := fmt.Sprintf("ns-integration-account-%d", time.Now().UnixNano())
	if _, err := application.GetLimits(ctx, "ns-integration-bootstrap", accountNamespace); err != nil {
		t.Fatal(err)
	}

	type createdTunnel struct {
		namespace string
		response  core.TunnelResponse
	}
	created := make(chan createdTunnel, 20)
	failures := make(chan error, 20)
	var waitGroup sync.WaitGroup
	for index := 0; index < 20; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			namespace := fmt.Sprintf("ns-integration-%02d", index)
			response, err := application.CreateTunnel(ctx, namespace, accountNamespace, core.CreateTunnelRequest{
				Name: fmt.Sprintf("tunnel-%02d", index), ClusterID: "cluster-a",
			})
			if err != nil {
				failures <- err
				return
			}
			created <- createdTunnel{namespace: namespace, response: response}
		}(index)
	}
	waitGroup.Wait()
	close(created)
	close(failures)
	var tunnels []createdTunnel
	for tunnel := range created {
		tunnels = append(tunnels, tunnel)
	}
	var createFailures []error
	for failure := range failures {
		createFailures = append(createFailures, failure)
	}
	if len(tunnels) != 10 {
		t.Fatalf("created %d tunnels concurrently, want 10; failures: %v", len(tunnels), createFailures)
	}
	for _, failure := range createFailures {
		assertErrorCode(t, failure, core.CodeTunnelQuotaExceeded)
	}

	first := tunnels[0]
	if first.response.Type != "bridge" || first.response.ExpirationHours != 72 || first.response.URL != first.response.TunnelID+".cluster-a.myhuaweicloud.com" {
		t.Fatalf("unexpected tunnel response: %#v", first.response)
	}
	for index := 1; index <= 10; index++ {
		protocol := "auto"
		allowAnonymous := false
		port := int64(8000 + index)
		if _, err := application.CreatePort(ctx, first.namespace, accountNamespace, first.response.TunnelID,
			core.CreateTunnelPortRequest{Port: &port, Protocol: &protocol, AllowAnonymous: &allowAnonymous}); err != nil {
			t.Fatalf("create port %d: %v", port, err)
		}
	}
	port := int64(9000)
	protocol := "http"
	allowAnonymous := false
	_, err = application.CreatePort(ctx, first.namespace, accountNamespace, first.response.TunnelID,
		core.CreateTunnelPortRequest{Port: &port, Protocol: &protocol, AllowAnonymous: &allowAnonymous})
	assertErrorCode(t, err, core.CodeTunnelPortQuotaExceeded)

	firstToken, err := application.IssueTunnelToken(ctx, first.namespace, accountNamespace, first.response.TunnelID, "host")
	if err != nil {
		t.Fatal(err)
	}
	secondToken, err := application.IssueTunnelToken(ctx, first.namespace, accountNamespace, first.response.TunnelID, "host")
	if err != nil || firstToken.Token == secondToken.Token {
		t.Fatal("token calls must return independently signed tokens")
	}

	var accountID uint64
	if err := database.DB().QueryRowContext(ctx, `SELECT account_id FROM tunnel WHERE tunnel_id = ?`, first.response.TunnelID).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	reportedAt := now.Unix() + 10
	insertMetering := `INSERT IGNORE INTO tunnel_metering
		(account_id, cluster_id, tunnel_id, session_id, upload_bytes, download_bytes, reported_at, created_at, settled)
		VALUES (?, 'cluster-a', ?, 'session-1', 100, 200, ?, ?, 0)`
	if _, err := database.DB().ExecContext(ctx, insertMetering, accountID, first.response.TunnelID, reportedAt, reportedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB().ExecContext(ctx, insertMetering, accountID, first.response.TunnelID, reportedAt, reportedAt); err != nil {
		t.Fatal(err)
	}
	settled, err := application.SettleBatch(ctx, 500)
	if err != nil || settled != 1 {
		t.Fatalf("settled = %d, err = %v", settled, err)
	}
	detail, err := application.GetTunnel(ctx, first.namespace, accountNamespace, first.response.TunnelID)
	if err != nil || detail.BandwidthUsed != 300 {
		t.Fatalf("tunnel usage = %d, err = %v", detail.BandwidthUsed, err)
	}
	limits, err := application.GetLimits(ctx, first.namespace, accountNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if limits.ActiveTunnels != 10 || limits.RemainingBytes != limits.QuotaBytes-300 {
		t.Fatalf("unexpected limits: %#v", limits)
	}

	for _, tunnel := range tunnels {
		if _, err := application.DeleteTunnel(ctx, tunnel.namespace, accountNamespace, tunnel.response.TunnelID); err != nil {
			t.Fatal(err)
		}
	}
}

func assertErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var appError *core.AppError
	if !errors.As(err, &appError) || appError.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}
