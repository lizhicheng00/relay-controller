//go:build integration

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

	"relay-controller/internal/core"
	"relay-controller/internal/security"
	"relay-controller/internal/store"
)

func TestServiceAgainstMariaDB(t *testing.T) {
	dsn := os.Getenv("RELAY_INTEGRATION_DATABASE_DSN")
	if dsn == "" {
		t.Skip("RELAY_INTEGRATION_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	database, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	}()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := security.NewJWTSigner(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatal(err)
	}
	application := New(database, signer, "myhuaweicloud.com", "region-a", slog.New(slog.NewTextHandler(io.Discard, nil)))
	now := time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC)
	application.now = func() time.Time { return now }

	accountNamespace := fmt.Sprintf("ns-integration-account-%d", time.Now().UnixNano())
	firstNamespace := fmt.Sprintf("ns-integration-user-a-%d", time.Now().UnixNano())
	secondNamespace := fmt.Sprintf("ns-integration-user-b-%d", time.Now().UnixNano())
	if _, err := application.GetLimits(ctx, firstNamespace, accountNamespace); err != nil {
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
			namespace := firstNamespace
			if index%2 != 0 {
				namespace = secondNamespace
			}
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
	if len(tunnels) != 20 || len(createFailures) != 0 {
		t.Fatalf("created %d tunnels concurrently, want 20; failures: %v", len(tunnels), createFailures)
	}
	_, err = application.CreateTunnel(ctx, firstNamespace, accountNamespace, core.CreateTunnelRequest{
		Name: "tunnel-overflow", ClusterID: "cluster-a",
	})
	assertErrorCode(t, err, core.CodeTunnelQuotaExceeded)
	_, err = application.CreateTunnel(ctx, secondNamespace, accountNamespace, core.CreateTunnelRequest{
		Name: "tunnel-overflow", ClusterID: "cluster-a",
	})
	assertErrorCode(t, err, core.CodeTunnelQuotaExceeded)

	first := tunnels[0]
	if first.response.Type != "bridge" || first.response.ExpirationHours != 72 || first.response.URL != first.response.TunnelID+".cluster-a.myhuaweicloud.com" {
		t.Fatalf("unexpected tunnel response: %#v", first.response)
	}
	originalExpiration := first.response.TunnelExpiration
	now = now.Add(time.Hour)
	name := "updated-tunnel"
	if _, err := application.UpdateTunnel(ctx, first.namespace, first.response.TunnelID, core.UpdateTunnelRequest{Name: &name}); err != nil {
		t.Fatal(err)
	}
	detail, err := application.GetTunnel(ctx, first.namespace, first.response.TunnelID)
	if err != nil || detail.TunnelExpiration != originalExpiration {
		t.Fatalf("metadata update changed expiration: expiration=%d, err=%v", detail.TunnelExpiration, err)
	}
	expirationHours := 48
	if _, err := application.UpdateTunnel(ctx, first.namespace, first.response.TunnelID, core.UpdateTunnelRequest{Expiration: &expirationHours}); err != nil {
		t.Fatal(err)
	}
	detail, err = application.GetTunnel(ctx, first.namespace, first.response.TunnelID)
	if err != nil || detail.TunnelExpiration != now.Unix()+48*3600 {
		t.Fatalf("expiration update = %d, err=%v", detail.TunnelExpiration, err)
	}
	for index := 1; index <= 10; index++ {
		protocol := "auto"
		allowAnonymous := false
		port := int64(8000 + index)
		if _, err := application.CreatePort(ctx, first.namespace, first.response.TunnelID,
			core.CreateTunnelPortRequest{Port: &port, Protocol: &protocol, AllowAnonymous: &allowAnonymous}); err != nil {
			t.Fatalf("create port %d: %v", port, err)
		}
	}
	port := int64(9000)
	protocol := "http"
	allowAnonymous := false
	_, err = application.CreatePort(ctx, first.namespace, first.response.TunnelID,
		core.CreateTunnelPortRequest{Port: &port, Protocol: &protocol, AllowAnonymous: &allowAnonymous})
	assertErrorCode(t, err, core.CodeTunnelPortQuotaExceeded)

	firstToken, err := application.IssueTunnelToken(ctx, first.namespace, first.response.TunnelID, "host")
	if err != nil {
		t.Fatal(err)
	}
	secondToken, err := application.IssueTunnelToken(ctx, first.namespace, first.response.TunnelID, "host")
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
	detail, err = application.GetTunnel(ctx, first.namespace, first.response.TunnelID)
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
		if _, err := application.DeleteTunnel(ctx, tunnel.namespace, tunnel.response.TunnelID); err != nil {
			t.Fatal(err)
		}
	}

	duplicateNamespace := fmt.Sprintf("ns-integration-duplicate-%d", time.Now().UnixNano())
	duplicateAccount := duplicateNamespace + "-account"
	firstNamed, err := application.CreateTunnel(ctx, duplicateNamespace, duplicateAccount,
		core.CreateTunnelRequest{Name: "shared-name", ClusterID: "cluster-a"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = application.CreateTunnel(ctx, duplicateNamespace, duplicateAccount,
		core.CreateTunnelRequest{Name: "shared-name", ClusterID: "cluster-a"})
	assertErrorCode(t, err, core.CodeTunnelNameConflict)
	secondNamed, err := application.CreateTunnel(ctx, duplicateNamespace, duplicateAccount,
		core.CreateTunnelRequest{Name: "other-name", ClusterID: "cluster-a"})
	if err != nil {
		t.Fatal(err)
	}
	conflictingName := "shared-name"
	_, err = application.UpdateTunnel(ctx, duplicateNamespace, secondNamed.TunnelID,
		core.UpdateTunnelRequest{Name: &conflictingName})
	assertErrorCode(t, err, core.CodeTunnelNameConflict)
	for _, tunnelID := range []string{firstNamed.TunnelID, secondNamed.TunnelID} {
		if _, err := application.DeleteTunnel(ctx, duplicateNamespace, tunnelID); err != nil {
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
