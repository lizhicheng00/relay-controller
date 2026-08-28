package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"mgmt-service/internal/config"
	"mgmt-service/internal/secret"
)

const identitySource = `
main domain-id user-id namespace
sub domain-id user-id ns-sub-namespace
`

type identity struct {
	kind      string
	domainID  string
	userID    string
	namespace string
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return convert(identitySource, os.Stdout, cfg.Secrets)
}

func convert(source string, output io.Writer, codec *secret.Codec) error {
	identities, err := parseIdentities(source)
	if err != nil {
		return err
	}
	if len(identities) == 0 {
		return fmt.Errorf("input contains no identities")
	}

	_, _ = fmt.Fprintln(output, "INSERT INTO legacy_identity")
	_, _ = fmt.Fprintln(output, "    (identity_type, account_mapping_key, member_mapping_key, namespace)")
	_, _ = fmt.Fprintln(output, "VALUES")
	for index, item := range identities {
		accountKey := codec.Fingerprint("domain", item.domainID)
		memberKey := codec.Fingerprint("user", item.domainID, item.userID)
		separator := ","
		if index == len(identities)-1 {
			separator = ";"
		}
		_, _ = fmt.Fprintf(output, "    ('%s', UNHEX('%s'), UNHEX('%s'), '%s')%s\n",
			item.kind, hex.EncodeToString(accountKey), hex.EncodeToString(memberKey),
			strings.ReplaceAll(item.namespace, "'", "''"), separator)
	}
	return nil
}

func parseIdentities(source string) ([]identity, error) {
	var identities []identity
	for index, sourceLine := range strings.Split(source, "\n") {
		lineNumber := index + 1
		line := strings.TrimSpace(sourceLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 4 || (fields[0] != "main" && fields[0] != "sub") {
			return nil, fmt.Errorf("line %d must be: main|sub domain user namespace", lineNumber)
		}
		identities = append(identities, identity{
			kind: fields[0], domainID: fields[1], userID: fields[2], namespace: fields[3],
		})
	}
	return identities, nil
}
