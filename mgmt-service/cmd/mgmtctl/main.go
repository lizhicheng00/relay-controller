package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"mgmt-service/internal/domain"
	"mgmt-service/internal/idgen"
	"mgmt-service/internal/store/mysqlstore"
)

type bindOptions struct {
	domainID         string
	userID           string
	userName         string
	accountNamespace string
	namespace        string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "bind-namespace" {
		return errors.New("usage: mgmtctl bind-namespace [options]")
	}
	flags := flag.NewFlagSet("bind-namespace", flag.ContinueOnError)
	options := bindOptions{}
	flags.StringVar(&options.domainID, "iam-domain-id", "", "Huawei IAM domain ID")
	flags.StringVar(&options.userID, "iam-user-id", "", "Huawei IAM user ID")
	flags.StringVar(&options.userName, "iam-user-name", "", "Huawei IAM user name")
	flags.StringVar(&options.accountNamespace, "account-namespace", "", "existing billing namespace")
	flags.StringVar(&options.namespace, "namespace", "", "existing resource namespace")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if err := options.validate(); err != nil {
		return err
	}
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		return errors.New("DATABASE_DSN is required")
	}

	repository, err := mysqlstore.Open(dsn)
	if err != nil {
		return err
	}
	defer repository.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	seed, err := bindingSeed(options)
	if err != nil {
		return err
	}
	identity, err := repository.ResolveIdentity(ctx, domain.IAMIdentity{
		DomainID: options.domainID,
		UserID:   options.userID,
		UserName: options.userName,
	}, seed)
	if err != nil {
		return fmt.Errorf("bind namespace: %w", err)
	}
	if identity.AccountNamespace != options.accountNamespace || identity.Namespace != options.namespace {
		return fmt.Errorf(
			"IAM identity is already bound to accountNamespace=%s namespace=%s",
			identity.AccountNamespace, identity.Namespace,
		)
	}
	_, _ = fmt.Fprintf(os.Stdout, "bound IAM domain %s user %s to %s (%s)\n",
		options.domainID, options.userID, identity.Namespace, identity.AccountNamespace)
	return nil
}

func (o bindOptions) validate() error {
	values := map[string]string{
		"iam-domain-id":     o.domainID,
		"iam-user-id":       o.userID,
		"account-namespace": o.accountNamespace,
		"namespace":         o.namespace,
	}
	for name, value := range values {
		if !validIdentifier(value) {
			return fmt.Errorf("--%s is required and must be a valid identifier", name)
		}
	}
	if len(o.userName) > 128 || strings.ContainsAny(o.userName, "\r\n\x00") {
		return errors.New("--iam-user-name is invalid")
	}
	return nil
}

func bindingSeed(options bindOptions) (domain.IdentitySeed, error) {
	accountID, err := idgen.New("acc_")
	if err != nil {
		return domain.IdentitySeed{}, err
	}
	principalID, err := idgen.New("prn_")
	if err != nil {
		return domain.IdentitySeed{}, err
	}
	namespaceID, err := idgen.New("nsp_")
	if err != nil {
		return domain.IdentitySeed{}, err
	}
	return domain.IdentitySeed{
		AccountID:        accountID,
		AccountNamespace: options.accountNamespace,
		PrincipalID:      principalID,
		NamespaceID:      namespaceID,
		Namespace:        options.namespace,
		DisplayName:      options.namespace,
	}, nil
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || index > 0 && strings.ContainsRune("._:-", char) {
			continue
		}
		return false
	}
	return true
}
