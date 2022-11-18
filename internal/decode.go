package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

func Decode[T any](
	ctx context.Context,
	r io.ReadCloser,
	logs chan<- T,
	decodeNext func(d *json.Decoder) (T, error),
) error {
	defer func() {
		_, _ = io.Copy(io.Discard, r)
		_ = r.Close()
	}()

	d := json.NewDecoder(r)
	if err := readBracket(d, "["); err != nil {
		return err
	}
	for d.More() {
		msg, err := decodeNext(d)
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("decoding was interrupted with context error: %w", ctx.Err())
		default:
		}
		logs <- msg
	}
	if err := readBracket(d, "]"); err != nil {
		return err
	}

	return nil
}

func readBracket(d *json.Decoder, want string) error {
	t, err := d.Token()
	if err != nil {
		return fmt.Errorf("malformed json array: %w", err)
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return fmt.Errorf("malformed json array, want %s, got %v", want, t)
	}
	if delim.String() != want {
		return fmt.Errorf("malformed json array, want %s, got %v", want, delim.String())
	}

	return nil
}
