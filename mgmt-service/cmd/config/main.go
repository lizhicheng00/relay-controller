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
	if len(args) != 2 {
		return errors.New("usage: mgmt-config <init-key|encrypt> <key-file>")
	}

	switch args[0] {
	case "init-key":
		return secret.GenerateKeyFile(args[1])
	case "encrypt":
		codec, err := secret.Load(args[1])
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
		return errors.New("usage: mgmt-config <init-key|encrypt> <key-file>")
	}
}
