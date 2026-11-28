package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func runCLI(ctx context.Context, args []string, grant func(context.Context, int32) (bool, error), stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "grant-admin" {
		_, _ = fmt.Fprintln(stderr, "usage: onlineconf grant-admin --user-id N")
		return 2
	}
	flags := flag.NewFlagSet("grant-admin", flag.ContinueOnError)
	flags.SetOutput(stderr)
	userID := flags.String("user-id", "", "GoGet user ID")
	if err := flags.Parse(args[1:]); err != nil {
		return 1
	}
	id, err := strconv.ParseInt(*userID, 10, 32)
	if err != nil || id <= 0 || flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "user-id must be a positive int32; usage: onlineconf grant-admin --user-id N")
		return 1
	}
	created, err := grant(ctx, int32(id))
	if errors.Is(err, domain.ErrUserNotFound) {
		_, _ = fmt.Fprintf(stderr, "user %d not found\n", id)
		return 1
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if created {
		_, _ = fmt.Fprintf(stdout, "granted user %d\n", id)
	} else {
		_, _ = fmt.Fprintf(stdout, "user %d is already an admin\n", id)
	}
	return 0
}

// Без явного DATABASE_URL config.Load подставит локальную базу, и право уйдёт не туда.
func requireDatabaseURL(lookup func(string) (string, bool)) (string, error) {
	if url, ok := lookup("DATABASE_URL"); ok && url != "" {
		return url, nil
	}
	return "", errors.New("DATABASE_URL must be set")
}
