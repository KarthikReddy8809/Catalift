// Command admin is the operator's tool (ADR-0006, D13): it creates the
// seller and reviewer accounts and clears the AI budget block. Passwords are
// read from stdin so they never appear in shell history or process lists.
//
//	echo -n "$PASSWORD" | admin create-user seller@example.com seller
//	echo -n "$PASSWORD" | admin set-password seller@example.com
//	admin clear-budget-block 8000000
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/KarthikReddy8809/catalift/internal/auth"
	"github.com/KarthikReddy8809/catalift/internal/config"
	"github.com/KarthikReddy8809/catalift/internal/store"
)

const usage = `usage:
  admin create-user <email> <seller|reviewer>   (password on stdin)
  admin set-password <email>                    (password on stdin)
  admin clear-budget-block <limit_micro_usd>`

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer st.Close()
	q := store.New(st.Pool)

	switch {
	case args[0] == "create-user" && len(args) == 3:
		role := store.UserRole(args[2])
		if role != store.UserRoleSeller && role != store.UserRoleReviewer {
			return errors.New("role must be seller or reviewer")
		}
		hash, err := passwordFrom(stdin)
		if err != nil {
			return err
		}
		id, err := q.CreateUser(ctx, store.CreateUserParams{Email: strings.ToLower(args[1]), Role: role, PasswordHash: hash})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		return say(out, fmt.Sprintf("created %s user %d", role, id))
	case args[0] == "set-password" && len(args) == 2:
		hash, err := passwordFrom(stdin)
		if err != nil {
			return err
		}
		n, err := q.SetUserPassword(ctx, store.SetUserPasswordParams{Lower: args[1], PasswordHash: hash})
		if err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if n == 0 {
			return errors.New("no user with that email")
		}
		return say(out, "password updated")
	case args[0] == "clear-budget-block" && len(args) == 2:
		limit, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || limit <= 0 {
			return errors.New("limit_micro_usd must be a positive integer (8000000 is USD 8)")
		}
		if err := q.ClearBudgetBlock(ctx, limit); err != nil {
			return fmt.Errorf("clear budget block: %w", err)
		}
		return say(out, fmt.Sprintf("budget block cleared; limit %d micro-USD", limit))
	}
	return errors.New(usage)
}

func say(out io.Writer, msg string) error {
	if _, err := fmt.Fprintln(out, msg); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// passwordFrom reads one line from stdin and hashes it; at least 12 characters.
func passwordFrom(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password: %w", err)
	}
	pw := strings.TrimRight(line, "\r\n")
	if len(pw) < 12 || len(pw) > 200 {
		return "", errors.New("the password must be 12 to 200 characters")
	}
	return auth.HashPassword(pw)
}
