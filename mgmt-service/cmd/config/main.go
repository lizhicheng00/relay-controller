package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"mgmt-service/internal/secret"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && args[0] == "generate-pig" {
		pig, err := secret.GenerateComponent()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, pig)
		return err
	}
	if len(args) != 2 {
		return usageError()
	}

	switch args[0] {
	case "init-dog":
		return secret.GenerateDogFile(args[1])
	case "encrypt":
		codec, err := secret.Load(args[1], os.Getenv("CONFIG_PIG"))
		if err != nil {
			return err
		}
		plaintext, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read value: %w", err)
		}
		value, err := codec.Encrypt(plaintext)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, value)
		return err
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: mgmt-config generate-pig | <init-dog|encrypt> <dog-file>")
}
